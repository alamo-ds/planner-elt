package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

type spyWriter struct {
	w  io.Writer
	n  int
	mu sync.Mutex
}

func (w *spyWriter) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)

	w.mu.Lock()
	defer w.mu.Unlock()

	w.n += n
	return n, err
}

func (w *spyWriter) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n
}

type JSONStream struct {
	pw    *io.PipeWriter
	sw    *spyWriter
	enc   *json.Encoder
	first bool
	mu    sync.Mutex
}

func NewJSONStream(pw *io.PipeWriter) *JSONStream {
	sw := &spyWriter{w: pw}

	return &JSONStream{
		pw:    pw,
		sw:    sw,
		enc:   json.NewEncoder(sw),
		first: true,
	}
}

func (s *JSONStream) Start() error {
	_, err := s.sw.Write([]byte("["))
	return err
}

func (s *JSONStream) Write(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.first {
		if _, err := s.sw.Write([]byte(",")); err != nil {
			return err
		}
	}

	s.first = false

	if err := s.enc.Encode(v); err != nil {
		return fmt.Errorf("couldn't encode to JSON stream: %w", err)
	}

	return nil
}

func (s *JSONStream) Close(err error) {
	if err == nil {
		s.sw.Write([]byte("]"))
	}

	s.pw.CloseWithError(err)
}

func (s *JSONStream) BytesWritten() int {
	return s.sw.Count()
}

type ManagedStream struct {
	JSON *JSONStream
	done chan error
}

func NewManagedStream(ctx context.Context, client *azblob.Client, dir string) *ManagedStream {
	pr, pw := io.Pipe()
	js := NewJSONStream(pw)
	done := make(chan error, 1)

	go func() {
		defer pr.Close()
		var contentType = "application/json"

		_, err := client.UploadStream(ctx, blobContainerName, newObjNameNow(dir), pr, &azblob.UploadStreamOptions{
			HTTPHeaders: &blob.HTTPHeaders{
				BlobContentType: &contentType,
			},
		})

		done <- err
	}()

	js.Start()
	return &ManagedStream{JSON: js, done: done}
}

func (ms *ManagedStream) Close(err error) error {
	ms.JSON.Close(err)
	return <-ms.done
}

func newObjNameNow(blobDir string) string {
	// #nosec G404
	objName := time.Now().Format("2006-01-02") + "-" + fmt.Sprintf("%08x", rand.Uint32()) + ".json"
	return filepath.Join(blobDir, objName)
}
