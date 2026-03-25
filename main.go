package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/alamo-ds/msgraph/graph"
)

var (
	tenantId           = os.Getenv("TENANT_ID")
	clientId           = os.Getenv("CLIENT_ID")
	clientSecret       = os.Getenv("CLIENT_SECRET")
	storageAccountName = os.Getenv("STORAGE_ACCOUNT_NAME")
	blobContainerName  = os.Getenv("BLOB_CONTAINER_NAME")
)

var cfg = graph.AzureADConfig{
	TenantID: tenantId,
	ClientID: clientId,
}

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}

func main() {
	blobClient, err := newBlobClient()
	if err != nil {
		slog.Error("couldn't create azblob.Client", "error", err)
	}

	slog.Info("blob client created...")

	ctx := context.Background()
	graphClient := graph.NewClient(ctx, clientSecret, cfg)

	if err := runELT(ctx, graphClient, blobClient); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}

	slog.Info("exiting...")
	os.Exit(0)
}

func runELT(ctx context.Context, graphClient *graph.Client, blobClient *azblob.Client) error {
	if err := checkCfg(); err != nil {
		return fmt.Errorf("invalid config: %v", err)
	}

	client, err := newClient(graphClient)
	if err != nil {
		return fmt.Errorf("couldn't create MS graph client: %v", err)
	}
	defer client.Close()

	slog.Debug("MS graph client created...")

	var tasks []Task

	slog.Info("executing DAG...")
	for r := range client.execute(ctx) {
		task, ok := r.(*Task)
		if !ok {
			slog.Debug("type mismatch!")
		}
		tasks = append(tasks, *task)
	}

	if err := client.Error(); err != nil {
		return err
	}
	if len(tasks) == 0 {
		slog.Info("Job did not return any results")
		return nil
	}

	slog.Info("tasks obtained", "count", len(tasks))

	if err = pushBlob(ctx, blobClient, tasksDir, tasks); err != nil {
		return fmt.Errorf("couldn't push tasks to storage: %v", err)
	}

	return nil
}

func checkCfg() error {
	m := []string{}

	if cfg.TenantID == "" {
		m = append(m, "TENANT_ID")
	}
	if cfg.ClientID == "" {
		m = append(m, "CLIENT_ID")
	}
	if clientSecret == "" {
		m = append(m, "CLIENT_SECRET")
	}

	if storageAccountName == "" {
		m = append(m, "STORAGE_ACOUNT_NAME")
	}
	if blobContainerName == "" {
		m = append(m, "BLOB_CONTAINER_NAME")
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
