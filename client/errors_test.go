package client

import (
	"context"
	"errors"
	"testing"
)

func TestErrAbortedWrapsContextCancellation(t *testing.T) {
	err := abortError(context.Canceled)
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("abortError(context.Canceled) does not match ErrAborted: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("abortError(context.Canceled) does not preserve context.Canceled: %v", err)
	}

	deadlineErr := abortError(context.DeadlineExceeded)
	if errors.Is(deadlineErr, ErrAborted) {
		t.Fatalf("abortError(context.DeadlineExceeded) unexpectedly matches ErrAborted")
	}
}

func TestStreamRecvReturnsErrAbortedOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := &Stream{core: newSessionCore(Options{})}
	_, err := stream.Recv(ctx)
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("Recv() error = %v, want ErrAborted", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Recv() error = %v, want context.Canceled", err)
	}
}

func TestSessionWaitReturnsErrAbortedForCancelledRead(t *testing.T) {
	core := newSessionCore(Options{})
	core.setReadError(context.Canceled)
	close(core.streamState().readDone)

	err := core.Wait()
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("Wait() error = %v, want ErrAborted", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v, want context.Canceled", err)
	}
}
