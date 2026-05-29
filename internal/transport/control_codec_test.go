package transport

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type captureTransport struct {
	payload any
}

func (t *captureTransport) Start(context.Context) error { return nil }

func (t *captureTransport) ReadJSON() (map[string]any, error) { return nil, nil }

func (t *captureTransport) WriteJSON(payload any) error {
	t.payload = payload
	return nil
}

func (t *captureTransport) EndInput() error { return nil }

func (t *captureTransport) Interrupt() error { return nil }

func (t *captureTransport) Wait() error { return nil }

func (t *captureTransport) Close() error { return nil }

func TestControlCodecFormatsMCPReconnectForClaude(t *testing.T) {
	inner := &captureTransport{}
	codec := newControlCodecTransport(inner)

	err := codec.WriteJSON(protocol.NewControlRequestEnvelope("request-1", protocol.ControlRequest{
		Subtype:    "mcp_reconnect",
		ServerName: "filesystem",
	}))
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	payload := inner.payload.(map[string]any)
	request := payload["request"].(map[string]any)
	if request["serverName"] != "filesystem" {
		t.Fatalf("serverName = %#v, want filesystem", request["serverName"])
	}
	if _, exists := request["server_name"]; exists {
		t.Fatalf("server_name should not be emitted for Claude wire: %#v", request)
	}
}

func TestControlCodecPreservesSDKSnakeCaseForStopTask(t *testing.T) {
	inner := &captureTransport{}
	codec := newControlCodecTransport(inner)

	err := codec.WriteJSON(protocol.NewControlRequestEnvelope("request-1", protocol.ControlRequest{
		Subtype: "stop_task",
		TaskID:  "task-1",
		Mode:    permission.ModeDefault,
	}))
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	payload := inner.payload.(map[string]any)
	request := payload["request"].(map[string]any)
	if request["task_id"] != "task-1" {
		t.Fatalf("task_id = %#v, want task-1", request["task_id"])
	}
}
