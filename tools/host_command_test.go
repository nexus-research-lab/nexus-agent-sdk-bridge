package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHostCommandToolListsAndCallsThroughMCP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	script := writeHostCommandScript(t, `#!/bin/sh
payload=$(cat)
case "$payload" in
  *'"message":"hello"'*) ;;
  *) echo "missing message" >&2; exit 1 ;;
esac
printf '{"content":[{"type":"text","text":"delivered"}],"structuredContent":{"ok":true}}'
`)
	tool, err := NewHostCommandTool(HostCommandOptions{
		Name:        "host_send_user_message",
		Description: "Send a visible user message through the host bridge.",
		Command:     script,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewHostCommandTool() error = %v", err)
	}
	server := CreateSDKMCPServer(SDKMCPServerOptions{Name: "host", Tools: []Tool{tool}})

	listResponse, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list error = %v", err)
	}
	tools := listResponse["result"].(map[string]any)["tools"].([]map[string]any)
	if len(tools) != 1 || tools[0]["name"] != "host_send_user_message" {
		t.Fatalf("tools/list = %#v, want host tool", tools)
	}

	callResponse, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "host_send_user_message",
			"arguments": map[string]any{"message": "hello"},
		},
	})
	if err != nil {
		t.Fatalf("tools/call error = %v", err)
	}
	result := callResponse["result"].(map[string]any)
	content := result["content"].([]map[string]any)
	if content[0]["text"] != "delivered" {
		t.Fatalf("content = %#v, want delivered", content)
	}
	if result["structuredContent"].(map[string]any)["ok"] != true {
		t.Fatalf("structuredContent = %#v, want ok=true", result["structuredContent"])
	}
}

func TestHostCommandToolFailureReturnsMCPErrorResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	script := writeHostCommandScript(t, `#!/bin/sh
echo "host failed" >&2
exit 2
`)
	tool, err := NewHostCommandTool(HostCommandOptions{Name: "host_fail", Command: script})
	if err != nil {
		t.Fatalf("NewHostCommandTool() error = %v", err)
	}
	server := CreateSDKMCPServer(SDKMCPServerOptions{Name: "host", Tools: []Tool{tool}})

	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "host_fail",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("tools/call error = %v", err)
	}
	result := response["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("result = %#v, want MCP isError", result)
	}
	content := result["content"].([]map[string]any)
	if content[0]["text"] != "host failed" {
		t.Fatalf("content = %#v, want host failed", content)
	}
}

func TestHostCommandToolTimeoutReturnsMCPErrorResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	script := writeHostCommandScript(t, `#!/bin/sh
sleep 1
`)
	tool, err := NewHostCommandTool(HostCommandOptions{
		Name:    "host_timeout",
		Command: script,
		Timeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewHostCommandTool() error = %v", err)
	}
	server := CreateSDKMCPServer(SDKMCPServerOptions{Name: "host", Tools: []Tool{tool}})

	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "host_timeout",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("tools/call error = %v", err)
	}
	result := response["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("result = %#v, want MCP isError", result)
	}
	content := result["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "timed out") {
		t.Fatalf("content = %#v, want timeout message", content)
	}
}

func writeHostCommandScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "host-tool.sh")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	return path
}
