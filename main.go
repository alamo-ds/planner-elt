package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/alamo-ds/msgraph/graph"
)

var (
	tenantId     = os.Getenv("tenantid")
	clientId     = os.Getenv("clientid")
	clientSecret = os.Getenv("clientsecret")
)

var cfg = graph.AzureADConfig{
	TenantID:     tenantId,
	ClientID:     clientId,
	ClientSecret: clientSecret,
}

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
	}

	slog.Info("exiting...")
	os.Exit(0)
}

func run() error {
	if err := checkCfg(); err != nil {
		return fmt.Errorf("invalid config: %v", err)
	}

	blobClient, err := newBlobClient()
	if err != nil {
		return fmt.Errorf("error creating blob client: %v", err)
	}
	slog.Info("blob client created...")

	ctx := context.Background()
	client, err := newClient(graph.NewClient(ctx, cfg))
	if err != nil {
		return fmt.Errorf("couldn't create MS graph client: %v", err)
	}
	slog.Debug("MS graph client created...")
	defer client.Close()

	slog.Info("starting root worker...")

	var tasks []Task

	for r := range client.execute(context.Background()) {
		task, ok := r.(*Task)
		if !ok {
			slog.Debug("type mismatch!")
		}
		tasks = append(tasks, *task)
	}

	if len(tasks) == 0 {
		slog.Info("Job did not return any results")
		return nil
	}

	if err = pushBlob(ctx, blobClient, tasksDir, tasks); err != nil {
		return fmt.Errorf("couldn't push tasks to storage: %v", err)
	}

	return nil
}

func checkCfg() error {
	m := []string{}

	if cfg.TenantID == "" {
		m = append(m, "tenantid")
	}
	if cfg.ClientID == "" {
		m = append(m, "clientid")
	}
	if cfg.ClientSecret == "" {
		m = append(m, "clientsecret")
	}

	if len(m) == 0 {
		return nil
	}

	var sb strings.Builder
	for i, envVar := range m {
		sb.WriteString("  " + envVar)
		if i < len(m)-1 {
			sb.WriteString("\n")
		}
	}

	return errors.New("please set the following variables:\n" + sb.String())
}
