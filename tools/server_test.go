package tools

import (
	"context"
	"testing"
)

func TestCreateSDKMCPServerHandlesToolCall(t *testing.T) {
	tool := New(
		"echo",
		"Echo input",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
			},
		},
		func(_ context.Context, input map[string]any, _ *Context) (Result, error) {
			return Text(input["text"].(string)), nil
		},
	)
	server := CreateSDKMCPServer(SDKMCPServerOptions{
		Name:  "test",
		Tools: []Tool{tool},
	})

	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "echo",
			"arguments": map[string]any{
				"text": "hello",
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	result := response["result"].(map[string]any)
	content := result["content"].([]map[string]any)
	if content[0]["text"] != "hello" {
		t.Fatalf("content = %#v, want hello", content)
	}
}
