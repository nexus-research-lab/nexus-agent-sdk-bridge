package mcpserver

import (
	"context"
	"strings"
	"testing"
)

func TestHandleMessageReturnsJSONRPCErrorForNilToolHandler(t *testing.T) {
	server := NewSimpleServer("test", "", []Tool{{Name: "broken"}})

	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "broken",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	payload := response["error"].(map[string]any)
	if payload["code"] != -32603 {
		t.Fatalf("error code = %#v, want -32603", payload["code"])
	}
	if !strings.Contains(payload["message"].(string), "handler is nil") {
		t.Fatalf("error message = %#v, want nil handler message", payload["message"])
	}
}
