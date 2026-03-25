package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/alamo-ds/msgraph/graph"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type mockTransport struct {
	http.RoundTripper
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "login.microsoftonline.com") {
		token := `{"access_token": "fake-token", "token_type": "Bearer", "expires_in": 3600}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(token)),
			Header:     make(http.Header),
		}, nil
	}
	return t.RoundTripper.RoundTrip(req)
}

func newGraphMux(t *testing.T) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/groups", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value": [{"id": "group-1"}]}`))
	})
	mux.HandleFunc("/groups/group-1/planner/plans", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value": [{"id": "plan-1"}]}`))
	})
	mux.HandleFunc("/planner/plans/plan-1/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value": [{"id": "task-1", "title": "Test Task", "conversationThreadId": "thread-1"}]}`))
	})
	mux.HandleFunc("/planner/tasks/task-1/details", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"description": "Task details"}`))
	})
	mux.HandleFunc("/groups/group-1/threads/thread-1/posts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value": [{"id": "post-1", "body": {"content": "comment"}}]}`))
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value": [{"id": "user-1", "displayName": "Test User"}]}`))
	})

	return mux
}

func TestRunELT_Integration(t *testing.T) {
	mockClient := &http.Client{
		Transport: &mockTransport{RoundTripper: http.DefaultTransport},
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, mockClient)

	// 1. Override global config variables
	cfg.TenantID = "test-tenant"
	cfg.ClientID = "test-client"
	clientSecret = "test-secret"
	storageAccountName = "devstoreaccount1"
	blobContainerName = "test-container"

	// 2. Mock MS Graph API
	mux := newGraphMux(t)

	mockGraphServer := httptest.NewServer(mux)
	defer mockGraphServer.Close()

	graphClient := graph.NewClient(ctx, clientSecret, cfg)
	graphClient.BaseURL = mockGraphServer.URL

	// 3. Mock Azure Blob Storage
	var uploadedBlob []byte
	var uploadedPath string

	blobMux := http.NewServeMux()
	blobMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			uploadedPath = r.URL.Path
			body, err := io.ReadAll(r.Body)
			if err == nil {
				uploadedBlob = body
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mockBlobServer := httptest.NewServer(blobMux)
	defer mockBlobServer.Close()

	blobClient, err := azblob.NewClientWithNoCredential(mockBlobServer.URL, nil)
	require.NoError(t, err)

	err = runELT(ctx, graphClient, blobClient)
	require.NoError(t, err)
	require.NotEmpty(t, uploadedBlob, "expected a blob to be uploaded")
	require.True(t, strings.HasPrefix(uploadedPath, "/test-container/tasks/"), "expected blob path to start with /test-container/tasks/")

	var tasks []Task
	err = json.Unmarshal(uploadedBlob, &tasks)
	require.NoError(t, err)

	require.Len(t, tasks, 1)
	require.Equal(t, "task-1", tasks[0].ID)
	require.Equal(t, "Test Task", tasks[0].Name)
}
