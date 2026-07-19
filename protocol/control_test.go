package protocol

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

func TestControlRequestUsesClaudeMixedCasing(t *testing.T) {
	cases := []struct {
		name    string
		request ControlRequest
		want    map[string]any
	}{
		{
			name: "initialize camel fields",
			request: ControlRequest{
				Subtype:                "initialize",
				SDKMCPServers:          []string{"docs"},
				JSONSchema:             map[string]any{"type": "object"},
				SystemPrompt:           "system",
				AppendSystemPrompt:     "append",
				ExcludeDynamicSections: boolPointer(true),
				AgentProgressSummaries: boolPointer(true),
			},
			want: map[string]any{
				"sdkMcpServers":          []any{"docs"},
				"jsonSchema":             map[string]any{"type": "object"},
				"systemPrompt":           "system",
				"appendSystemPrompt":     "append",
				"excludeDynamicSections": true,
				"agentProgressSummaries": true,
			},
		},
		{
			name: "rewind snake fields",
			request: ControlRequest{
				Subtype:       "rewind_files",
				UserMessageID: "user-1",
				DryRun:        boolPointer(true),
			},
			want: map[string]any{
				"user_message_id": "user-1",
				"dry_run":         true,
			},
		},
		{
			name: "mcp camel fields",
			request: ControlRequest{
				Subtype:     "mcp_oauth_callback_url",
				ServerName:  "github",
				CallbackURL: "https://example.test/callback",
			},
			want: map[string]any{
				"serverName":  "github",
				"callbackUrl": "https://example.test/callback",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(NewControlRequestEnvelope("request-1", tc.request))
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			request := payload["request"].(map[string]any)
			for key, want := range tc.want {
				if got := request[key]; !reflect.DeepEqual(got, want) {
					t.Fatalf("request[%q] = %#v, want %#v", key, got, want)
				}
			}
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func TestControlEnvelopeConstructors(t *testing.T) {
	request := NewControlRequestEnvelope("request-1", ControlRequest{Subtype: "initialize"})
	if request.Type != "control_request" || request.RequestID != "request-1" || request.Request.Subtype != "initialize" {
		t.Fatalf("NewControlRequestEnvelope() = %#v, want request envelope", request)
	}
	cancel := NewControlCancelRequest("request-1")
	if cancel.Type != "control_cancel_request" || cancel.RequestID != "request-1" {
		t.Fatalf("NewControlCancelRequest() = %#v, want cancel envelope", cancel)
	}

	success := NewControlSuccessResponse("request-1", map[string]any{"ok": true})
	if success.Type != "control_response" ||
		success.Response.Subtype != "success" ||
		success.Response.RequestID != "request-1" ||
		success.Response.Response["ok"] != true {
		t.Fatalf("NewControlSuccessResponse() = %#v, want success response", success)
	}

	failure := NewControlErrorResponse("request-2", "failed")
	if failure.Type != "control_response" ||
		failure.Response.Subtype != "error" ||
		failure.Response.RequestID != "request-2" ||
		failure.Response.Error != "failed" {
		t.Fatalf("NewControlErrorResponse() = %#v, want error response", failure)
	}

	ack := DecodeControlAck(map[string]any{
		"type":            "control_ack",
		"request_id":      "request-3",
		"request_subtype": "hook_callback",
		"stage":           "applied",
		"hook_event_name": "PostToolUse",
		"tool_use_id":     "tool-1",
		"session_id":      "session-1",
	})
	if ack.Type != "control_ack" || ack.RequestID != "request-3" || ack.Stage != "applied" ||
		ack.HookEventName != "PostToolUse" || ack.ToolUseID != "tool-1" || ack.SessionID != "session-1" {
		t.Fatalf("DecodeControlAck() = %#v, want applied hook ack", ack)
	}
}

func TestElicitationAndUserDialogHelpersMatchControlShapes(t *testing.T) {
	request := DecodeElicitationRequest(map[string]any{
		"mcp_server_name":  "demo",
		"message":          "Open the auth URL",
		"mode":             "url",
		"url":              "https://example.com/auth",
		"elicitation_id":   "eli-1",
		"requested_schema": map[string]any{"type": "object"},
		"display_name":     "Demo",
	})
	if request.ServerName != "demo" ||
		request.Mode != string(ElicitationModeURL) ||
		request.ElicitationID != "eli-1" ||
		request.DisplayName != "Demo" ||
		request.RequestedSchema["type"] != "object" {
		t.Fatalf("DecodeElicitationRequest() = %#v, want snake_case fields", request)
	}

	response := ElicitationResponse{
		Action:  "bogus",
		Content: map[string]any{"approved": true},
	}
	payload := response.ContentMap()
	if payload["action"] != string(ElicitationActionDecline) {
		t.Fatalf("Elicitation ContentMap action = %#v, want default decline", payload["action"])
	}
	content := payload["content"].(map[string]any)
	content["approved"] = false
	if response.Content["approved"] != true {
		t.Fatalf("Elicitation ContentMap leaked content mutation: %#v", response.Content)
	}

	dialogRequest := DecodeUserDialogRequest(map[string]any{
		"dialog_kind": "confirm",
		"tool_use_id": "tool-1",
		"payload":     map[string]any{"title": "Allow?"},
	})
	if dialogRequest.DialogKind != "confirm" || dialogRequest.ToolUseID != "tool-1" || dialogRequest.Payload["title"] != "Allow?" {
		t.Fatalf("DecodeUserDialogRequest() = %#v, want dialog kind/tool/payload", dialogRequest)
	}
	dialogResponse := SubmitUserDialog(map[string]any{"approved": true})
	dialogPayload := dialogResponse.ContentMap()
	if dialogPayload["action"] != string(UserDialogActionSubmit) || jsonvalue.MapValue(dialogPayload["payload"])["approved"] != true {
		t.Fatalf("SubmitUserDialog() = %#v, want submit payload", dialogPayload)
	}
	jsonvalue.MapValue(dialogPayload["payload"])["approved"] = false
	if jsonvalue.MapValue(dialogResponse["payload"])["approved"] != true {
		t.Fatalf("UserDialog ContentMap leaked payload mutation: %#v", dialogResponse)
	}
}
