package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// A closed message stream does not prove that its command process has exited.
type deferredExitTransport struct {
	*scriptedTransport
	exit        chan struct{}
	waitStarted chan struct{}
	waitOnce    sync.Once
	closeErr    error
	exitErr     error
}

func (t *deferredExitTransport) Close() error {
	_ = t.scriptedTransport.Close()
	return t.closeErr
}
func (t *deferredExitTransport) Wait() error {
	t.waitOnce.Do(func() { close(t.waitStarted) })
	<-t.exit
	return t.exitErr
}

func TestDisconnectWaitsForExitAfterCloseError(t *testing.T) {
	closeErr, exitErr := errors.New("termination attempt failed"), errors.New("process exited unsuccessfully")
	tr := &deferredExitTransport{scriptedTransport: newScriptedTransport(), exit: make(chan struct{}), waitStarted: make(chan struct{}), closeErr: closeErr, exitErr: exitErr}
	var release sync.Once
	defer release.Do(func() { close(tr.exit) })
	core := newSessionCoreWithTransport(Options{Transport: tr, Runtime: RuntimeOptions{InitializeTimeout: time.Second}}, tr)
	connected := make(chan error, 1)
	go func() { connected <- core.Connect(context.Background()) }()
	assertInitializeRequest(t, receiveWrite(t, tr.scriptedTransport))
	tr.pushRead(successfulInitializeResponse(map[string]any{"session_id": "exit-confirmation"}))
	if err := receiveDone(t, connected); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := core.Disconnect(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Disconnect released exit fence prematurely: %v", err)
	}
	select {
	case <-tr.waitStarted:
	default:
		t.Fatal("Disconnect never requested process exit confirmation")
	}
	select {
	case <-core.streams.closeState.done:
		t.Fatal("cleanup completed while process is still alive")
	default:
	}
	// A second waiter must join the same cleanup, not regard a closed stream as done.
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := core.Disconnect(ctx2); !errors.Is(err, context.Canceled) {
		t.Fatalf("second waiter: %v", err)
	}
	release.Do(func() { close(tr.exit) })
	finished, finishCancel := context.WithTimeout(context.Background(), time.Second)
	defer finishCancel()
	err := core.Disconnect(finished)
	if !errors.Is(err, closeErr) || !errors.Is(err, exitErr) {
		t.Fatalf("lost close or exit diagnostics: %v", err)
	}
}
