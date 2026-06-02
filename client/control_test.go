package client

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type failingControlTransport struct {
	writeErr error
	writes   int
}

func (t *failingControlTransport) Start(context.Context) error { return nil }

func (t *failingControlTransport) ReadJSON() (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (t *failingControlTransport) WriteJSON(any) error {
	t.writes++
	return t.writeErr
}

func (t *failingControlTransport) EndInput() error  { return nil }
func (t *failingControlTransport) Interrupt() error { return nil }
func (t *failingControlTransport) Wait() error      { return nil }
func (t *failingControlTransport) Close() error     { return nil }

func TestHandleControlRequestMarksTransportFailedWhenResponseWriteFails(t *testing.T) {
	transport := &failingControlTransport{
		writeErr: errors.New("process: write payload failed: Stream closed"),
	}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycleState().setConnected(true)

	core.handleControlRequest(map[string]any{
		"request_id": "request-hook",
		"request": map[string]any{
			"subtype": "unsupported",
		},
	})

	if transport.writes != 1 {
		t.Fatalf("WriteJSON calls = %d, want 1", transport.writes)
	}
	if core.isConnected() {
		t.Fatal("session should be marked disconnected after control response write failure")
	}
	readErr := core.getReadError()
	if readErr == nil || !strings.Contains(readErr.Error(), "send control response failed") ||
		!strings.Contains(readErr.Error(), "Stream closed") {
		t.Fatalf("read error missing control response failure detail: %v", readErr)
	}
}
