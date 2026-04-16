package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/alamo-ds/planner-elt/internal/msgraph"
	"github.com/s-hammon/p"
	"golang.org/x/sync/errgroup"
)

type App struct {
	blobClient  *azblob.Client
	graphClient *msgraph.Client
	logger      *slog.Logger
	errCh       chan error

	streams map[string]*ManagedStream
}

func NewApp(blobClient *azblob.Client, graphClient *msgraph.Client, logger *slog.Logger) *App {
	return &App{
		blobClient:  blobClient,
		graphClient: graphClient,
		logger:      logger,
		errCh:       make(chan error),
	}
}

type GraphCollection[T any] struct {
	Value []T `json:"value"`
}

func (a *App) Run(ctx context.Context) error {
	// init streams
	a.streams = map[string]*ManagedStream{
		"groups":  NewManagedStream(ctx, a.blobClient, "groups"),
		"users":   NewManagedStream(ctx, a.blobClient, "users"),
		"plans":   NewManagedStream(ctx, a.blobClient, "plans"),
		"buckets": NewManagedStream(ctx, a.blobClient, "buckets"),
		"tasks":   NewManagedStream(ctx, a.blobClient, "tasks"),
		"details": NewManagedStream(ctx, a.blobClient, "details"),
		"posts":   NewManagedStream(ctx, a.blobClient, "posts"),
	}

	// Groups & Users
	requests := []msgraph.BatchRequest{
		{Id: "groups", Method: "GET", URL: "/groups"},
		{Id: "users", Method: "GET", URL: "/users"},
	}

	respSeq, err := a.graphClient.Batch(ctx, requests)
	if err != nil {
		return fmt.Errorf("batch request failed: %w", err)
	}

	var runErr error
	wg, ctx := errgroup.WithContext(ctx)
	wg.SetLimit(10)
	for resp, err := range respSeq {
		if err != nil {
			runErr = err
			break
		}
		if resp.Status != 200 {
			a.logger.Warn("batch sub-request failed",
				"id", resp.Id,
				"status", resp.Status,
			)
			continue
		}

		r := io.NopCloser(bytes.NewReader(resp.Body))
		switch resp.Id {
		case "groups":
			for item, sErr := range JSONSeq[msgraph.Group](r, "value") {
				if sErr != nil {
					a.logger.Error(p.Format("error in group sub-stream: %v\n", err))
					continue
				}

				if err := a.streams["groups"].JSON.Write(item); err != nil {
					runErr = err
					break
				}

				wg.Go(func() error {
					return a.processGroup(ctx, item.ID)
				})
			}
		case "users":
			for item, sErr := range JSONSeq[msgraph.User](r, "value") {
				if sErr != nil {
					a.logger.Error(p.Format("error in user sub-stream: %v\n", err))
					continue
				}

				if err := a.streams["users"].JSON.Write(item); err != nil {
					runErr = err
					break
				}
			}
		}
	}

	if err := wg.Wait(); err != nil {
		return err
	}

	for key, s := range a.streams {
		if err := s.Close(runErr); err != nil {
			a.logger.Error(p.Format("blob upload failed: %v\n", err))
		}

		nBytes := s.JSON.BytesWritten()
		a.logger.Info(p.Format("successfully wrote %s blob", key), "total_bytes", nBytes, "total_kb", fmtKB(nBytes))
		delete(a.streams, key)
	}
	return runErr
}

func fmtKB(n int) string {
	kb := float64(n) / 1024
	return p.Format("%.1f KB", kb)
}

func (a *App) processGroup(ctx context.Context, groupId string) error {
	body, err := a.graphClient.Get(ctx, "groups", groupId, "planner", "plans")
	if err != nil {
		return fmt.Errorf("couldn't get plans for groupID %q: %w", groupId, err)
	}

	var taskRequests []msgraph.BatchRequest
	for plan, err := range JSONSeq[msgraph.Plan](body, "value") {
		if err != nil {
			return err
		}

		a.streams["plans"].JSON.Write(plan)

		taskRequests = append(taskRequests,
			// new tasks request
			msgraph.BatchRequest{
				Id:     "tasks:" + plan.ID,
				Method: "GET",
				URL:    p.Format("/planner/plans/%s/tasks", plan.ID),
			},
			// new buckets request
			msgraph.BatchRequest{
				Id:     "buckets:" + plan.ID,
				Method: "GET",
				URL:    p.Format("/planner/plans/%s/buckets", plan.ID),
			},
		)
	}

	respSeq, err := a.graphClient.Batch(ctx, taskRequests)
	if err != nil {
		return fmt.Errorf("batch request failed: %w", err)
	}

	var enrichmentRequests []msgraph.BatchRequest
	for resp, err := range respSeq {
		if err != nil {
			a.logger.Error(p.Format("batch sub-request failed: %v\n", err))
			continue
		}
		if resp.Status != 200 {
			a.logger.Warn("batch sub-request failed", "id", resp.Id, "status", resp.Status)
			continue
		}

		r := io.NopCloser(bytes.NewReader(resp.Body))
		switch obj := strings.SplitN(resp.Id, ":", 2); obj[0] {
		case "tasks":
			for task := range JSONSeq[msgraph.Task](r, "value") {
				a.streams["tasks"].JSON.Write(task)

				enrichmentRequests = append(enrichmentRequests, msgraph.BatchRequest{
					Id:     "det:" + task.ID,
					Method: "GET",
					URL:    p.Format("/planner/tasks/%s/details", task.ID),
				})

				if task.ConversationThreadID != "" {
					enrichmentRequests = append(enrichmentRequests, msgraph.BatchRequest{
						Id:     "post:" + task.ID,
						Method: "GET",
						URL:    p.Format("/groups/%s/threads/%s/posts", groupId, task.ConversationThreadID),
					})
				}
			}
		case "buckets":
			for item := range JSONSeq[msgraph.Bucket](r, "value") {
				a.streams["buckets"].JSON.Write(item)
			}
		}
	}

	enrichSeq, err := a.graphClient.Batch(ctx, enrichmentRequests)
	if err != nil {
		return err
	}

	for resp, err := range enrichSeq {
		if err != nil || resp.Status != 200 {
			continue
		}

		parts := strings.Split(resp.Id, ":")
		taskID := parts[1]
		switch parts[0] {
		case "det":
			var details msgraph.TaskDetails
			if err := json.Unmarshal(resp.Body, &details); err == nil {
				details.TaskID = taskID
				a.streams["details"].JSON.Write(details)
			}
		case "post":
			for post := range JSONSeq[msgraph.Post](io.NopCloser(bytes.NewReader(resp.Body)), "value") {
				post.TaskID = taskID
				a.streams["posts"].JSON.Write(post)
			}
		}
	}

	return nil
}
