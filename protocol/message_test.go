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

func TestEncodeOutboundMessageWithOptions(t *testing.T) {
	message := NewUserTextMessageWithOptions("continue", OutboundMessageOptions{
		Meta:           true,
		HiddenFromUser: true,
		Purpose:        "host_continuation",
		Priority:       "internal",
		Metadata:       map[string]string{"task_id": "task-1"},
	})

	payload := EncodeOutboundMessage(message, "session-1")
	if payload["is_meta"] != true || payload["is_synthetic"] != true || payload["hidden_from_user"] != true {
		t.Fatalf("payload options = %#v, want meta/synthetic/hidden", payload)
	}
	if payload["purpose"] != "host_continuation" || payload["priority"] != "internal" {
		t.Fatalf("payload purpose/priority = %#v, want host continuation", payload)
	}
	metadata := payload["metadata"].(map[string]string)
	if metadata["task_id"] != "task-1" {
		t.Fatalf("metadata = %#v, want task_id", metadata)
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

func TestParseTaskProgressMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"task_progress",
		"session_id":"session-1",
		"task_id":"task-1",
		"tool_use_id":"tool-1",
		"agent_id":"agent-1",
		"agent_type":"worker",
		"description":"运行子任务",
		"last_tool_name":"Read",
		"parent_task_id":"parent-1",
		"summary":"已读取文件",
		"usage":{"total_tokens":123,"tool_uses":2,"duration_ms":456}
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(task_progress) error = %v", err)
	}
	if message.Type != MessageTypeTaskProgress {
		t.Fatalf("Type = %q, want task_progress", message.Type)
	}
	if message.TaskProgress == nil {
		t.Fatal("TaskProgress = nil")
	}
	if message.TaskProgress.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", message.TaskProgress.TaskID)
	}
	if message.TaskProgress.AgentID != "agent-1" || message.TaskProgress.AgentType != "worker" {
		t.Fatalf("Agent fields = %#v, want agent-1/worker", message.TaskProgress)
	}
	if message.TaskProgress.ParentTaskID != "parent-1" {
		t.Fatalf("ParentTaskID = %q, want parent-1", message.TaskProgress.ParentTaskID)
	}
	if message.TaskProgress.Usage.TotalTokens != 123 {
		t.Fatalf("TotalTokens = %d, want 123", message.TaskProgress.Usage.TotalTokens)
	}
}

func TestParseTaskStartedMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"task_started",
		"session_id":"session-1",
		"task_id":"task-1",
		"tool_use_id":"tool-1",
		"agent_id":"agent-1",
		"agent_type":"worker",
		"description":"运行子任务",
		"output_file":"/tmp/task.out",
		"parent_task_id":"parent-1",
		"prompt":"inspect code"
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(task_started) error = %v", err)
	}
	if message.Type != MessageTypeTaskStarted {
		t.Fatalf("Type = %q, want task_started", message.Type)
	}
	if message.TaskStarted == nil {
		t.Fatal("TaskStarted = nil")
	}
	if message.TaskStarted.AgentID != "agent-1" || message.TaskStarted.AgentType != "worker" {
		t.Fatalf("Agent fields = %#v, want agent-1/worker", message.TaskStarted)
	}
	if message.TaskStarted.OutputFile != "/tmp/task.out" || message.TaskStarted.ParentTaskID != "parent-1" {
		t.Fatalf("TaskStarted = %#v, want output file and parent task id", message.TaskStarted)
	}
}

func TestParseSystemTaskNotificationMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"system",
		"subtype":"task_notification",
		"session_id":"session-1",
		"task_id":"task-1",
		"tool_use_id":"tool-1",
		"agent_id":"agent-1",
		"agent_type":"worker",
		"parent_task_id":"parent-1",
		"status":"completed",
		"output_file":"/tmp/task.out",
		"transcript_path":"/tmp/subagent.jsonl",
		"usage":{"total_tokens":123,"tool_uses":2,"duration_ms":456},
		"summary":"完成"
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(system task_notification) error = %v", err)
	}
	if message.System == nil || message.System.TaskNotification == nil {
		t.Fatal("System.TaskNotification = nil")
	}
	if message.System.TaskNotification.Status != "completed" {
		t.Fatalf("Status = %q, want completed", message.System.TaskNotification.Status)
	}
	if message.System.TaskNotification.AgentID != "agent-1" || message.System.TaskNotification.AgentType != "worker" {
		t.Fatalf("Agent fields = %#v, want agent-1/worker", message.System.TaskNotification)
	}
	if message.System.TaskNotification.ParentTaskID != "parent-1" {
		t.Fatalf("ParentTaskID = %q, want parent-1", message.System.TaskNotification.ParentTaskID)
	}
	if message.System.TaskNotification.TranscriptPath != "/tmp/subagent.jsonl" {
		t.Fatalf("TranscriptPath = %q, want transcript path", message.System.TaskNotification.TranscriptPath)
	}
	if message.System.TaskNotification.Usage.ToolUses != 2 {
		t.Fatalf("Usage = %#v, want tool uses", message.System.TaskNotification.Usage)
	}
}

func TestParseTopLevelTaskNotificationMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"task_notification",
		"session_id":"session-1",
		"task_id":"task-1",
		"agent_id":"agent-1",
		"agent_type":"worker",
		"status":"failed",
		"output_file":"/tmp/task.out",
		"transcript_path":"/tmp/subagent.jsonl",
		"summary":"失败"
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(task_notification) error = %v", err)
	}
	if message.Type != MessageTypeTaskNotification {
		t.Fatalf("Type = %q, want task_notification", message.Type)
	}
	if message.TaskNotification == nil {
		t.Fatal("TaskNotification = nil")
	}
	if message.TaskNotification.AgentID != "agent-1" || message.TaskNotification.AgentType != "worker" {
		t.Fatalf("Agent fields = %#v, want agent-1/worker", message.TaskNotification)
	}
	if message.TaskNotification.TranscriptPath != "/tmp/subagent.jsonl" {
		t.Fatalf("TranscriptPath = %q, want transcript path", message.TaskNotification.TranscriptPath)
	}
}

func TestParseSystemTaskUpdatedMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"system",
		"subtype":"task_updated",
		"session_id":"session-1",
		"task_id":"task-1",
		"patch":{
			"status":"killed",
			"description":"停止子任务",
			"end_time":1710000000000,
			"total_paused_ms":42,
			"error":"user stopped",
			"is_backgrounded":true
		}
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(system task_updated) error = %v", err)
	}
	if message.Type != MessageTypeSystem {
		t.Fatalf("Type = %q, want system", message.Type)
	}
	if message.System == nil || message.System.TaskUpdated == nil {
		t.Fatal("System.TaskUpdated = nil")
	}
	if message.System.TaskUpdated.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", message.System.TaskUpdated.TaskID)
	}
	if message.System.TaskUpdated.Status != "killed" {
		t.Fatalf("Status = %q, want killed", message.System.TaskUpdated.Status)
	}
	if message.System.TaskUpdated.Patch.TotalPausedMS != 42 {
		t.Fatalf("TotalPausedMS = %d, want 42", message.System.TaskUpdated.Patch.TotalPausedMS)
	}
	if !message.System.TaskUpdated.Patch.IsBackgrounded {
		t.Fatal("IsBackgrounded = false, want true")
	}
}

func TestParseTopLevelTaskUpdatedMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"task_updated",
		"session_id":"session-1",
		"task_id":"task-1",
		"patch":{"status":"completed","description":"完成子任务"}
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(task_updated) error = %v", err)
	}
	if message.Type != MessageTypeTaskUpdated {
		t.Fatalf("Type = %q, want task_updated", message.Type)
	}
	if message.TaskUpdated == nil {
		t.Fatal("TaskUpdated = nil")
	}
	if message.TaskUpdated.Status != "completed" {
		t.Fatalf("Status = %q, want completed", message.TaskUpdated.Status)
	}
	if message.TaskUpdated.Patch.Description != "完成子任务" {
		t.Fatalf("Description = %q, want 完成子任务", message.TaskUpdated.Patch.Description)
	}
}

func TestParseUnknownMessagePreservesWireType(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"thinking_tokens",
		"session_id":"session-1",
		"estimated_tokens":128
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(thinking_tokens) error = %v", err)
	}
	if message.Type != MessageType("thinking_tokens") {
		t.Fatalf("Type = %q, want thinking_tokens", message.Type)
	}
	if message.Raw["estimated_tokens"] != float64(128) {
		t.Fatalf("Raw estimated_tokens = %#v, want 128", message.Raw["estimated_tokens"])
	}
}

func TestParseToolProgressMessage(t *testing.T) {
	message, err := ParseMessage([]byte(`{
		"type":"tool_progress",
		"session_id":"session-1",
		"tool_use_id":"agent-msg-1",
		"parent_tool_use_id":"call-agent",
		"tool_name":"Agent",
		"elapsed_time_seconds":3.5,
		"task_id":"agent-1",
		"data":{"type":"agent_progress","agent_id":"agent-1"}
	}`))
	if err != nil {
		t.Fatalf("ParseMessage(tool_progress) error = %v", err)
	}
	if message.Type != MessageTypeToolProgress {
		t.Fatalf("Type = %q, want tool_progress", message.Type)
	}
	if message.ToolProgress == nil {
		t.Fatal("ToolProgress = nil")
	}
	if message.ToolProgress.ToolUseID != "agent-msg-1" || message.ToolProgress.ToolName != "Agent" {
		t.Fatalf("ToolProgress = %#v, want tool id/name", message.ToolProgress)
	}
	if message.ToolProgress.ParentToolUseID == nil || *message.ToolProgress.ParentToolUseID != "call-agent" {
		t.Fatalf("ParentToolUseID = %#v, want call-agent", message.ToolProgress.ParentToolUseID)
	}
	if message.ToolProgress.TaskID != "agent-1" || message.ToolProgress.ElapsedTimeSeconds != 3.5 {
		t.Fatalf("ToolProgress = %#v, want task id and elapsed", message.ToolProgress)
	}
	if message.ToolProgress.Additional["data"] == nil {
		t.Fatalf("Additional = %#v, want raw data", message.ToolProgress.Additional)
	}
}

func TestParsePassiveSDKMessages(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want MessageType
	}{
		{
			name: "stream request start",
			raw:  `{"type":"stream_request_start","session_id":"session-1"}`,
			want: MessageTypeStreamRequestStart,
		},
		{
			name: "tool use summary",
			raw:  `{"type":"tool_use_summary","session_id":"session-1","summary":"Read x2","preceding_tool_use_ids":["tool-1","tool-2"]}`,
			want: MessageTypeToolUseSummary,
		},
		{
			name: "prompt suggestion",
			raw:  `{"type":"prompt_suggestion","session_id":"session-1","suggestion":"继续检查测试"}`,
			want: MessageTypePromptSuggestion,
		},
		{
			name: "auth status",
			raw:  `{"type":"auth_status","session_id":"session-1","isAuthenticating":true,"output":["登录中"],"error":"需要授权"}`,
			want: MessageTypeAuthStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := ParseMessage([]byte(tt.raw))
			if err != nil {
				t.Fatalf("ParseMessage() error = %v", err)
			}
			if message.Type != tt.want {
				t.Fatalf("Type = %q, want %q", message.Type, tt.want)
			}
			if message.Type == MessageTypeUnknown {
				t.Fatalf("message 被错误降级成 unknown: %+v", message)
			}
		})
	}
}

func TestParseKnownPassiveMessageKeepsRawPayload(t *testing.T) {
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
	if message.Raw["type"] != "stream_request_start" {
		t.Fatalf("Raw[type] = %#v, want stream_request_start", message.Raw["type"])
	}
}
