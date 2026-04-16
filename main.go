package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/alamo-ds/planner-elt/internal/build"
	"github.com/alamo-ds/planner-elt/internal/msgraph"
	"github.com/s-hammon/p"
)

var (
	storageAccountName = os.Getenv("STORAGE_ACCOUNT_NAME")
	blobContainerName  = os.Getenv("BLOB_CONTAINER_NAME")
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	var (
		tenantId     = os.Getenv("TENANT_ID")
		clientId     = os.Getenv("CLIENT_ID")
		clientSecret = os.Getenv("CLIENT_SECRET")
	)

	status := run(ctx, cancel, tenantId, clientId, clientSecret)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, tenantId, clientId, clientSecret string) int {
	if err := checkCfg(tenantId, clientId, clientSecret); err != nil {
		fmt.Fprintf(os.Stderr, "===invalid env var configuration===\n%v\n", err)
		return 1
	}

	logger, closeFn, err := initLogger(os.Getenv("PLANNER_ELT_LOG_FILE"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		return 1
	}
	defer func() {
		if err := closeFn(); err != nil {
			fmt.Fprintf(os.Stderr, "error cleaning up logger resources: %v\n", err)
		}
	}()

	env := os.Getenv("ENV")
	logger = logger.With(
		slog.String("git_sha", build.GitSHA),
		slog.String("build_time", build.BuildTime),
		slog.String("env", env),
		slog.String("tenant_id", p.Format("%s...", tenantId[:8])),
	)

	blobClient, err := newBlobClient()
	if err != nil {
		logger.Error(p.Format("failed to create blob client: %v\n", err))
		return 1
	}

	graphClient := msgraph.NewClient(ctx, tenantId, clientId, clientSecret, msgraph.WithLogger(logger))
	app := NewApp(blobClient, graphClient, logger)

	if err = app.Run(ctx); err != nil {
		logger.Error(p.Format("application error: %v\n", err))
		return 1
	}

	if path := os.Getenv("HEAP_PROFILE"); path != "" {
		root, err := os.OpenRoot(".")
		if err != nil {
			logger.Error(p.Format("failed to open root dir: %v\n", err))
			return 1
		}
		defer root.Close()

		f, err := root.Create(path)
		if err != nil {
			logger.Error(p.Format("failed to create heap profile: %v\n", err))
			return 1
		}
		defer f.Close()

		runtime.GC()

		if err := pprof.WriteHeapProfile(f); err != nil {
			logger.Error(p.Format("failed to write heap profile: %v\n", err))
			return 1
		}

		logger.Debug("heap profile written", slog.String("path", path))
	}

	return 0
}

func checkCfg(tenantId, clientId, clientSecret string) error {
	m := []string{}

	if tenantId == "" {
		m = append(m, "TENANT_ID")
	}
	if clientId == "" {
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

func JSONSeq[T any](body io.ReadCloser, arrayKey string) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		defer func() {
			io.Copy(io.Discard, body)
			body.Close()
		}()
		dec := json.NewDecoder(body)

		// Advance the decoder to the start of the "value" array
		for {
			t, err := dec.Token()
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(nil, err)
				return
			}

			// Look for the key name (e.g., "value")
			if s, ok := t.(string); ok && s == arrayKey {
				break
			}
		}

		// Expect the '[' token to start the array
		if _, err := dec.Token(); err != nil {
			yield(nil, err)
			return
		}

		// Stream the items inside the array
		for dec.More() {
			var item T
			if err := dec.Decode(&item); err != nil {
				if !yield(nil, err) {
					return
				}
				break
			}
			if !yield(&item, nil) {
				return
			}
		}
	}
}

func newBlobClient() (*azblob.Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %v", err)
	}

	containerURL := p.Format("https://%s.blob.core.windows.net/", storageAccountName)
	client, err := azblob.NewClient(containerURL, cred, nil)
	if err != nil {
		return nil, err
	}

	return client, nil
}
