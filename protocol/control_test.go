package protocol

import (
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

func TestControlEnvelopeConstructors(t *testing.T) {
	request := NewControlRequestEnvelope("request-1", ControlRequest{Subtype: "initialize"})
	if request.Type != "control_request" || request.RequestID != "request-1" || request.Request.Subtype != "initialize" {
		t.Fatalf("NewControlRequestEnvelope() = %#v, want request envelope", request)
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
