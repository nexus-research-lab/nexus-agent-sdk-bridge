package protocol

import "testing"

func TestEncodeOutboundMessages(t *testing.T) {
	message := NewUserBlocksMessage(
		NewTextContent("hello"),
		NewToolResultContent("tool-1", "done", false),
		NewImageContent("ZmFrZS1pbWFnZQ==", "image/png"),
		NewDocumentContent(map[string]any{"type": "base64", "data": "JVBERi0x"}, "application/pdf", "spec"),
		NewSearchResultContent("tool result", "Spec", "https://example.com/spec", "demo"),
		NewResourceLinkContent("设计文档", "file:///tmp/spec.md", "调试输出"),
	).WithParentToolUseID("tool-parent")

	payload := EncodeOutboundMessage(message, "session-1")
	if payload["type"] != "user" {
		t.Fatalf("payload[type] = %#v, want user", payload["type"])
	}
	if payload["session_id"] != "session-1" {
		t.Fatalf("payload[session_id] = %#v, want session-1", payload["session_id"])
	}
	messagePayload := payload["message"].(map[string]any)
	content := messagePayload["content"].([]map[string]any)
	if len(content) != 6 {
		t.Fatalf("len(content) = %d, want 6", len(content))
	}
	if content[0]["type"] != "text" || content[1]["tool_use_id"] != "tool-1" {
		t.Fatalf("content prefix = %#v, want text + tool_result", content[:2])
	}
	if content[2]["type"] != "image" || content[3]["type"] != "document" || content[4]["type"] != "search_result" || content[5]["type"] != "resource_link" {
		t.Fatalf("content suffix = %#v, want image/document/search_result/resource_link", content[2:])
	}
}

func TestDecodeAssistantMessageDoesNotInventError(t *testing.T) {
	message, err := DecodeMessage(map[string]any{
		"type":       "assistant",
		"session_id": "session-1",
		"message": map[string]any{
			"role":  "assistant",
			"model": "model-1",
			"content": []any{
				map[string]any{"type": "text", "text": "done"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if message.Assistant == nil {
		t.Fatal("Assistant = nil")
	}
	if message.Assistant.Error != "" {
		t.Fatalf("Assistant.Error = %q, want empty", message.Assistant.Error)
	}
}

func TestParseResultMessage(t *testing.T) {
	resultMessage, err := ParseMessage([]byte(`{
		"type":"result",
		"subtype":"success",
		"session_id":"session-1",
		"uuid":"uuid-result",
		"duration_ms":100,
		"duration_api_ms":90,
		"is_error":false,
		"num_turns":1,
		"result":"done",
		"terminal_reason":"completed",
		"total_cost_usd":1.25,
		"model_usage":{"nexus-sonnet":{"input_tokens":10,"output_tokens":20}},
		"permission_denials":[{"tool_name":"Bash","tool_use_id":"tool-1","tool_input":{"command":"rm -rf /"}}]
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(result) error = %v", err)
	}
	if resultMessage.Result == nil {
		t.Fatal("resultMessage.Result is nil")
	}
	if resultMessage.Result.Result != "done" {
		t.Fatalf("result = %q, want done", resultMessage.Result.Result)
	}
	if resultMessage.Result.TerminalReason != "completed" {
		t.Fatalf("terminal reason = %q, want completed", resultMessage.Result.TerminalReason)
	}
	if resultMessage.Result.TotalCostUSD != 1.25 {
		t.Fatalf("total cost = %v, want 1.25", resultMessage.Result.TotalCostUSD)
	}
	modelUsage := resultMessage.Result.ModelUsage["nexus-sonnet"].(map[string]any)
	if got := modelUsage["input_tokens"]; got != float64(10) {
		t.Fatalf("input_tokens = %#v, want 10", got)
	}
	if len(resultMessage.Result.PermissionDenials) != 1 {
		t.Fatalf("len(permission denials) = %d, want 1", len(resultMessage.Result.PermissionDenials))
	}
}

func TestParseResultMessageIgnoresLegacyAliases(t *testing.T) {
	resultMessage, err := ParseMessage([]byte(`{
		"type":"result",
		"subtype":"success",
		"session_id":"session-1",
		"uuid":"uuid-result-legacy",
		"modelUsage":{"nexus-sonnet":{"input_tokens":10}}
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(result legacy aliases) error = %v", err)
	}
	if resultMessage.Result == nil {
		t.Fatal("resultMessage.Result is nil")
	}
	if len(resultMessage.Result.ModelUsage) != 0 {
		t.Fatalf("ModelUsage = %#v, want legacy alias ignored", resultMessage.Result.ModelUsage)
	}
}

func TestParseStreamRequestStartMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"stream_request_start",
		"session_id":"session-1"
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(stream_request_start) error = %v", err)
	}
	if message.Type != MessageTypeStreamRequestStart {
		t.Fatalf("Type = %q, want stream_request_start", message.Type)
	}
	if message.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want session-1", message.SessionID)
	}
}
