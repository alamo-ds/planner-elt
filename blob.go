package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/s-hammon/p"
)

const (
	DefaultNumWorkers = 3
	tasksDir          = "tasks"
)

func newBlobClient() (*azblob.Client, error) {
	cred, err := azidentity.NewClientSecretCredential(tenantId, clientId, clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't authenticate: %v", err)
	}

	containerURL := p.Format("https://%s.blob.core.windows.net/", storageAccountName)
	client, err := azblob.NewClient(containerURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't create Blob client: %v", err)
	}

	return client, nil
}

func pushBlob(ctx context.Context, c *azblob.Client, blobDir string, v any) error {
	data, _ := json.MarshalIndent(v, "", "  ")

	objName := time.Now().Format("2006-01-02") + ".json"
	blobName := filepath.Join(blobDir, objName)

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	_, err := c.UploadBuffer(ctxWithCancel, blobContainerName, blobName, data, nil)
	if err != nil {
		return fmt.Errorf("couldn't write buffer to blob: %v", err)
	}

	slog.Info("pushed successfully to storage", "bytes", len(data), "container", blobContainerName, "directory", blobDir)
	return nil
}
