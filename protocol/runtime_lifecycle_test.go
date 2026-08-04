package protocol

import "testing"

func TestDecodeMessageDerivesToolLifecycleWithoutCopyingPayload(t *testing.T) {
	started, err := DecodeMessage(map[string]any{
		"type":       "assistant",
		"uuid":       "assistant-1",
		"session_id": "session-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type":  "tool_use",
				"id":    "tool-1",
				"name":  "search",
				"input": map[string]any{"secret": "must-not-be-copied"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("DecodeMessage(started) error = %v", err)
	}
	if len(started.RuntimeLifecycle) != 1 {
		t.Fatalf("started lifecycle count = %d, want 1", len(started.RuntimeLifecycle))
	}
	event := started.RuntimeLifecycle[0]
	if event.NodeKind != RuntimeLifecycleNodeTool ||
		event.Phase != RuntimeLifecycleStarted ||
		event.SubjectID != "tool-1" || event.Name != "search" ||
		event.Status != "running" {
		t.Fatalf("unexpected started event: %+v", event)
	}
	if event.Metadata != nil {
		t.Fatalf("tool payload leaked into lifecycle metadata: %+v", event.Metadata)
	}

	finished, err := DecodeMessage(map[string]any{
		"type":       "user",
		"uuid":       "user-1",
		"session_id": "session-1",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type":        "tool_result",
				"tool_use_id": "tool-1",
				"is_error":    true,
				"content":     "private result",
			}},
		},
	})
	if err != nil {
		t.Fatalf("DecodeMessage(finished) error = %v", err)
	}
	if len(finished.RuntimeLifecycle) != 1 {
		t.Fatalf("finished lifecycle count = %d, want 1", len(finished.RuntimeLifecycle))
	}
	event = finished.RuntimeLifecycle[0]
	if event.Phase != RuntimeLifecycleFinished || event.Status != "failed" || !event.Failed {
		t.Fatalf("unexpected finished event: %+v", event)
	}
}

func TestDecodeMessageDerivesSystemSubagentLifecycle(t *testing.T) {
	message, err := DecodeMessage(map[string]any{
		"type":             "system",
		"subtype":          "task_started",
		"uuid":             "task-message-1",
		"session_id":       "session-1",
		"task_id":          "task-1",
		"tool_use_id":      "tool-1",
		"agent_id":         "researcher",
		"agent_type":       "Explore",
		"description":      "Collect primary sources",
		"child_session_id": "child-session-1",
	})
	if err != nil {
		t.Fatalf("DecodeMessage(task_started) error = %v", err)
	}
	if len(message.RuntimeLifecycle) != 1 {
		t.Fatalf("task lifecycle count = %d, want 1; system=%+v", len(message.RuntimeLifecycle), message.System)
	}
	event := message.RuntimeLifecycle[0]
	if event.NodeKind != RuntimeLifecycleNodeSubagent ||
		event.Phase != RuntimeLifecycleStarted ||
		event.SubjectID != "task-1" ||
		event.AgentID != "researcher" ||
		event.ChildSessionID != "child-session-1" {
		t.Fatalf("unexpected task event: %+v", event)
	}
	if event.Metadata["tool_use_id"] != "tool-1" {
		t.Fatalf("task tool identity missing: %+v", event.Metadata)
	}
}
