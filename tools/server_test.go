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

func TestCreateSDKMCPServerPreservesNestedInputContract(t *testing.T) {
	tool := New(
		"plan",
		"Plan work",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"logical_key": map[string]any{"type": "string"},
						},
						"required":             []string{"logical_key"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"items"},
			"additionalProperties": false,
		},
		func(_ context.Context, input map[string]any, _ *Context) (Result, error) {
			items := input["items"].([]any)
			logicalKey := items[0].(map[string]any)["logical_key"]
			return Text(logicalKey.(string)), nil
		},
	)
	server := CreateSDKMCPServer(SDKMCPServerOptions{Name: "test", Tools: []Tool{tool}})

	listed, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list error = %v", err)
	}
	listedTool := listed["result"].(map[string]any)["tools"].([]map[string]any)[0]
	properties := listedTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	itemProperties := properties["items"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	if itemProperties["logical_key"].(map[string]any)["type"] != "string" {
		t.Fatalf("nested schema = %#v, want logical_key string", itemProperties)
	}

	called, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "plan",
			"arguments": map[string]any{
				"items": []any{map[string]any{"logical_key": "research"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("tools/call error = %v", err)
	}
	content := called["result"].(map[string]any)["content"].([]map[string]any)
	if content[0]["text"] != "research" {
		t.Fatalf("nested arguments = %#v, want research", content)
	}
}
