package runtimeinfo

import "testing"

func TestDecodeInitializeResponse(t *testing.T) {
	got := DecodeInitializeResponse(map[string]any{
		"commands": []any{
			map[string]any{"name": "plan", "argumentHint": "<task>", "allowedTools": "Read, Edit", "userInvocable": "true"},
		},
		"agents": []any{
			map[string]any{"name": "reviewer", "description": "Review code"},
		},
		"models": []any{
			map[string]any{"value": "model-a", "displayName": "Model A", "description": "Model A description"},
		},
		"account":                 map[string]any{"email": "dev@example.com", "subscriptionType": "pro"},
		"session_id":              "session-1",
		"output_style":            "concise",
		"available_output_styles": []any{"concise", "full"},
		"fast_mode_state":         "enabled",
		"protocol_capabilities":   []any{"hook_response_ack_v1"},
	})

	if len(got.Commands) != 1 || got.Commands[0].Name != "plan" {
		t.Fatalf("commands = %#v", got.Commands)
	}
	if len(got.Commands[0].AllowedTools) != 2 || got.Commands[0].AllowedTools[1] != "Edit" {
		t.Fatalf("allowed tools = %#v", got.Commands[0].AllowedTools)
	}
	if got.Commands[0].UserInvocable == nil || !*got.Commands[0].UserInvocable {
		t.Fatalf("user invocable = %#v", got.Commands[0].UserInvocable)
	}
	if len(got.Agents) != 1 || got.Agents[0].Name != "reviewer" {
		t.Fatalf("agents = %#v", got.Agents)
	}
	if len(got.Models) != 1 || got.Models[0].ID != "model-a" {
		t.Fatalf("models = %#v", got.Models)
	}
	if got.Account.Email != "dev@example.com" || got.Account.SubscriptionType != "pro" || got.OutputStyle != "concise" || got.SessionID != "session-1" {
		t.Fatalf("initialize response = %#v", got)
	}
	if len(got.ProtocolCapabilities) != 1 || got.ProtocolCapabilities[0] != "hook_response_ack_v1" {
		t.Fatalf("protocol capabilities = %#v", got.ProtocolCapabilities)
	}
}
