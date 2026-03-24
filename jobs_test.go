package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupWorker_TypecastError(t *testing.T) {
	in := make(chan any, 1)
	out := make(chan any, 1)

	in <- "invalid-job-type"
	close(in)

	err := groupWorker(context.Background(), in, out)
	require.Error(t, err)
}
