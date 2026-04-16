package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/alamo-ds/planner-elt/internal/msgraph"
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

func TestApp_Run_Profile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profile test")
	}

	mux := newGraphMux(t)
	mux.HandleFunc("/$batch", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"responses": []}`))
	})

	mockGraphServer := httptest.NewServer(mux)
	defer mockGraphServer.Close()

	blobMux := http.NewServeMux()
	blobMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mockBlobServer := httptest.NewServer(blobMux)
	defer mockBlobServer.Close()

	blobClient, err := azblob.NewClientWithNoCredential(mockBlobServer.URL, nil)
	require.NoError(t, err)

	mockClient := &http.Client{
		Transport: &mockTransport{RoundTripper: http.DefaultTransport},
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, mockClient)

	graphClient := msgraph.NewClient(ctx, "tenant", "client", "secret")
	graphClient.BaseURL = mockGraphServer.URL

	logger, _, _ := initLogger("")
	app := NewApp(blobClient, graphClient, logger)

	require.NoError(t, app.Run(ctx))

	f, err := os.Create("mem.prof")
	require.NoError(t, err)
	defer f.Close()

	runtime.GC()
	require.NoError(t, pprof.WriteHeapProfile(f))
}
