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

func TestCreateSDKMCPServerListsToolsInRegistrationOrder(t *testing.T) {
	server := CreateSDKMCPServer(SDKMCPServerOptions{
		Name: "test",
		Tools: []Tool{
			noOpTool("beta"),
			noOpTool("alpha"),
			noOpTool("gamma"),
		},
	})

	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	tools := response["result"].(map[string]any)["tools"].([]map[string]any)
	got := []string{tools[0]["name"].(string), tools[1]["name"].(string), tools[2]["name"].(string)}
	want := []string{"beta", "alpha", "gamma"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tools/list order = %#v, want %#v", got, want)
		}
	}
}

func TestCreateSDKMCPServerListsResourcesInRegistrationOrder(t *testing.T) {
	server := CreateSDKMCPServer(SDKMCPServerOptions{
		Name: "test",
		Resources: []Resource{
			{URI: "file:///b.txt", Name: "b"},
			{URI: "file:///a.txt", Name: "a"},
			{URI: "file:///c.txt", Name: "c"},
		},
	})

	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	resources := response["result"].(map[string]any)["resources"].([]map[string]any)
	got := []string{resources[0]["uri"].(string), resources[1]["uri"].(string), resources[2]["uri"].(string)}
	want := []string{"file:///b.txt", "file:///a.txt", "file:///c.txt"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resources/list order = %#v, want %#v", got, want)
		}
	}
}

func noOpTool(name string) Tool {
	return New(name, name, map[string]any{"type": "object", "properties": map[string]any{}}, func(context.Context, map[string]any, *Context) (Result, error) {
		return Text(""), nil
	})
}
