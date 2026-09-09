package client

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

type subagentWireTransport struct {
	failingControlTransport
	payload map[string]any
}

func (t *subagentWireTransport) WriteJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &t.payload); err != nil {
		return err
	}
	return errors.New("stop before dispatch")
}
func TestSubagentControlWireKeepsCallerAtRequestRoot(t *testing.T) {
	transport := &subagentWireTransport{}
	core := newSessionCoreWithTransport(Options{}, transport)
	core.lifecycle.setConnected(true)
	core.lifecycle.setInitializeResponse(runtimeinfo.InitializeResponse{ProtocolCapabilities: []string{subagentControlProtocolCapability}})
	session := &Session{core: core}
	_, err := session.Control().ControlSubagent(context.Background(), "mcp-original", "spawn", map[string]any{"prompt": "evidence"})
	if err == nil {
		t.Fatal("expected stopped transport")
	}
	request, ok := transport.payload["request"].(map[string]any)
	if !ok {
		t.Fatalf("request not sent: %#v %v", transport.payload, err)
	}
	if request["tool_use_id"] != "mcp-original" || request["operation"] != "spawn" || request["subtype"] != "subagent_control" {
		t.Fatalf("wire fields misplaced: %#v", request)
	}
	if request["input"].(map[string]any)["prompt"] != "evidence" || request["payload"] != nil {
		t.Fatalf("unexpected nesting: %#v", request)
	}
}
