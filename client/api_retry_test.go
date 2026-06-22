package client

import (
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestAPIRetryMessageFromStderr(t *testing.T) {
	message, ok := apiRetryMessageFromStderr(
		`API error (attempt 4/11): 529 {"type":"overloaded_error","message":"busy"}`,
		"session-1",
	)
	if !ok {
		t.Fatal("apiRetryMessageFromStderr() ok = false, want true")
	}
	if message.Type != protocol.MessageTypeSystem || message.Subtype != "api_retry" {
		t.Fatalf("message type = %s/%s, want system/api_retry", message.Type, message.Subtype)
	}
	if message.SessionID != "session-1" {
		t.Fatalf("session_id = %q, want session-1", message.SessionID)
	}
	if got := message.System.Data["attempt"]; got != 4 {
		t.Fatalf("attempt = %#v, want 4", got)
	}
	if got := message.System.Data["max_retries"]; got != 11 {
		t.Fatalf("max_retries = %#v, want 11", got)
	}
	if got := message.System.Data["error"]; got != "rate_limit" {
		t.Fatalf("error = %#v, want rate_limit", got)
	}
	if got := message.System.Data["error_status"]; got != "529" {
		t.Fatalf("error_status = %#v, want 529", got)
	}
}

func TestNormalizeSystemAPIErrorMessage(t *testing.T) {
	message := normalizeAPIRetrySystemMessage(protocol.ReceivedMessage{
		Type:    protocol.MessageTypeSystem,
		Subtype: "api_error",
		System: &protocol.SystemMessage{
			Subtype: "api_error",
			Data: map[string]any{
				"retryAttempt": 7,
				"maxRetries":   11,
				"retryInMs":    3000,
				"error": map[string]any{
					"status": 529,
					"type":   "overloaded_error",
				},
			},
		},
	})

	if message.Subtype != "api_retry" || message.System.Subtype != "api_retry" {
		t.Fatalf("subtype = %s/%s, want api_retry", message.Subtype, message.System.Subtype)
	}
	for key, want := range map[string]any{
		"attempt":        7,
		"max_retries":    11,
		"retry_delay_ms": 3000,
		"error_status":   529,
		"error":          "rate_limit",
	} {
		if got := message.System.Data[key]; got != want {
			t.Fatalf("%s = %#v, want %#v", key, got, want)
		}
	}
}
