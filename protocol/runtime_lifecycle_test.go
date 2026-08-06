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

func TestDecodeMessageDerivesAgentProgressSubagentLifecycle(t *testing.T) {
	message, err := DecodeMessage(map[string]any{
		"type":               "tool_progress",
		"uuid":               "progress-1",
		"session_id":         "session-1",
		"tool_use_id":        "agent-message-1",
		"parent_tool_use_id": "call-agent-1",
		"tool_name":          "Agent",
		"task_id":            "task-1",
		"data": map[string]any{
			"type":             "agent_progress",
			"agent_id":         "child-1",
			"agent_type":       "Explore",
			"description":      "Collect primary sources",
			"child_session_id": "child-session-1",
			"prompt":           "must not be copied",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(message.RuntimeLifecycle) != 2 {
		t.Fatalf("lifecycle count = %d, want Tool progress and Subagent start", len(message.RuntimeLifecycle))
	}
	event := message.RuntimeLifecycle[1]
	if event.NodeKind != RuntimeLifecycleNodeSubagent ||
		event.Phase != RuntimeLifecycleStarted ||
		event.SubjectID != "task-1" ||
		event.AgentID != "child-1" ||
		event.ChildSessionID != "child-session-1" ||
		event.Metadata["tool_use_id"] != "call-agent-1" {
		t.Fatalf("unexpected Agent progress lifecycle: %+v", event)
	}
	if _, leaked := event.Metadata["prompt"]; leaked {
		t.Fatalf("Agent prompt leaked into lifecycle: %+v", event.Metadata)
	}
}

func TestDecodeMessageDerivesStructuredOutputSubagentLifecycle(t *testing.T) {
	message, err := DecodeMessage(map[string]any{
		"type":       "attachment",
		"uuid":       "attachment-1",
		"session_id": "session-1",
		"attachment": map[string]any{
			"type": "structured_output",
			"data": map[string]any{
				"agentId":     "child-1",
				"agentType":   "Explore",
				"description": "Collected sources",
				"status":      "completed",
				"toolUseId":   "call-agent-1",
				"output":      "must not be copied",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(message.RuntimeLifecycle) != 1 {
		t.Fatalf("lifecycle count = %d, want one Subagent finish", len(message.RuntimeLifecycle))
	}
	event := message.RuntimeLifecycle[0]
	if event.NodeKind != RuntimeLifecycleNodeSubagent ||
		event.Phase != RuntimeLifecycleFinished ||
		event.SubjectID != "child-1" ||
		event.Status != "succeeded" || event.Failed ||
		event.Metadata["tool_use_id"] != "call-agent-1" {
		t.Fatalf("unexpected structured output lifecycle: %+v", event)
	}
	if _, leaked := event.Metadata["output"]; leaked {
		t.Fatalf("Subagent output leaked into lifecycle: %+v", event.Metadata)
	}
}
