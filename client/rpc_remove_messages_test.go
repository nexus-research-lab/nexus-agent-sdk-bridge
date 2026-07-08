package client

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestSessionCoreRemoveMessagesSendsControlRequest(t *testing.T) {
	transport := newScriptedTransport()
	core := newSessionCoreWithTransport(
		Options{
			Transport: transport,
			Runtime:   RuntimeOptions{InitializeTimeout: time.Second},
		},
		transport,
	)

	connectDone := make(chan error, 1)
	go func() {
		connectDone <- core.Connect(context.Background())
	}()
	assertInitializeRequest(t, receiveWrite(t, transport))
	transport.pushRead(successfulInitializeResponse(map[string]any{
		"session_id": "session-1",
	}))
	if err := receiveDone(t, connectDone); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		_ = core.Disconnect(context.Background())
	}()

	removeDone := make(chan error, 1)
	go func() {
		removeDone <- core.removeMessages(context.Background(), []string{" msg-1 ", "msg-2", "msg-1"})
	}()

	payload := receiveWrite(t, transport)
	if payload["type"] != "control_request" {
		t.Fatalf("remove_messages envelope = %#v", payload)
	}
	request, ok := payload["request"].(map[string]any)
	if !ok {
		t.Fatalf("remove_messages request = %#v", payload["request"])
	}
	if request["subtype"] != "remove_messages" {
		t.Fatalf("remove_messages subtype = %#v", request["subtype"])
	}
	if got := request["message_uuids"]; !reflect.DeepEqual(got, []any{"msg-1", "msg-2"}) {
		t.Fatalf("message_uuids = %#v, want msg-1/msg-2", got)
	}
	requestID, _ := payload["request_id"].(string)
	transport.pushRead(map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   map[string]any{"removed": 2},
		},
	})
	if err := receiveDone(t, removeDone); err != nil {
		t.Fatalf("removeMessages() error = %v", err)
	}
}
