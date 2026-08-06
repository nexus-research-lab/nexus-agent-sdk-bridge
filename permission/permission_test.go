package permission

import "testing"

func TestPermissionDecisionToMapIncludesApprovalFeedbackAndContentBlocks(t *testing.T) {
	decision := Decision{
		Behavior:       BehaviorAllow,
		AcceptFeedback: "approved with context",
		ContentBlocks: []map[string]any{
			{"type": "text", "text": "extra note"},
			{"type": "image", "data": "aW1n", "mime_type": "image/png"},
		},
	}

	payload := decision.ToMap()
	if payload["acceptFeedback"] != "approved with context" {
		t.Fatalf("acceptFeedback = %#v, want approval feedback", payload["acceptFeedback"])
	}
	blocks, ok := payload["contentBlocks"].([]map[string]any)
	if !ok || len(blocks) != 2 {
		t.Fatalf("contentBlocks = %#v, want cloned content block list", payload["contentBlocks"])
	}
	blocks[0]["text"] = "mutated"
	if decision.ContentBlocks[0]["text"] != "extra note" {
		t.Fatalf("decision content block mutated through ToMap clone: %#v", decision.ContentBlocks)
	}
}

func TestDenyWithErrorCodeEmitsStableCode(t *testing.T) {
	decision := DenyWithErrorCode(
		"等待用户确认超时",
		ErrorCodeRequestTimeout,
		true,
	)

	payload := decision.ToMap()
	if payload["behavior"] != string(BehaviorDeny) {
		t.Fatalf("behavior = %#v, want deny", payload["behavior"])
	}
	if payload["errorCode"] != string(ErrorCodeRequestTimeout) {
		t.Fatalf("errorCode = %#v, want request timeout", payload["errorCode"])
	}
	if payload["message"] != "等待用户确认超时" || payload["interrupt"] != true {
		t.Fatalf("deny payload = %#v, want message and interrupt", payload)
	}
}
