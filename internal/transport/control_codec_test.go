package transport

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type captureTransport struct {
	payload     any
	readPayload map[string]any
}

func (t *captureTransport) Start(context.Context) error { return nil }

func (t *captureTransport) ReadJSON() (map[string]any, error) { return t.readPayload, nil }

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

func TestControlCodecFormatsStopTaskForClaude(t *testing.T) {
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
	if _, exists := request["taskId"]; exists {
		t.Fatalf("taskId should not be emitted for Claude wire: %#v", request)
	}
}

func TestControlCodecFormatsRuntimeControlsForClaude(t *testing.T) {
	cases := []struct {
		name      string
		request   protocol.ControlRequest
		wantKey   string
		wantValue any
		blocked   string
	}{
		{
			name: "set thinking tokens",
			request: protocol.ControlRequest{
				Subtype:           "set_max_thinking_tokens",
				MaxThinkingTokens: intPointer(512),
			},
			wantKey:   "maxThinkingTokens",
			wantValue: float64(512),
			blocked:   "max_thinking_tokens",
		},
		{
			name: "rewind files",
			request: protocol.ControlRequest{
				Subtype:       "rewind_files",
				UserMessageID: "user-1",
				DryRun:        boolPointer(true),
			},
			wantKey:   "userMessageId",
			wantValue: "user-1",
			blocked:   "user_message_id",
		},
		{
			name: "mcp oauth callback",
			request: protocol.ControlRequest{
				Subtype:     "mcp_oauth_callback_url",
				ServerName:  "github",
				CallbackURL: "http://localhost/callback",
			},
			wantKey:   "callbackUrl",
			wantValue: "http://localhost/callback",
			blocked:   "callback_url",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner := &captureTransport{}
			codec := newControlCodecTransport(inner)

			if err := codec.WriteJSON(protocol.NewControlRequestEnvelope("request-1", tc.request)); err != nil {
				t.Fatalf("WriteJSON() error = %v", err)
			}

			payload := inner.payload.(map[string]any)
			request := payload["request"].(map[string]any)
			if request[tc.wantKey] != tc.wantValue {
				t.Fatalf("%s = %#v, want %#v in %#v", tc.wantKey, request[tc.wantKey], tc.wantValue, request)
			}
			if _, exists := request[tc.blocked]; exists {
				t.Fatalf("%s should not be emitted for Claude wire: %#v", tc.blocked, request)
			}
		})
	}
}

func TestControlCodecFormatsInitializeAgentsForClaude(t *testing.T) {
	inner := &captureTransport{}
	codec := newControlCodecTransport(inner)

	err := codec.WriteJSON(protocol.NewControlRequestEnvelope("request-1", protocol.ControlRequest{
		Subtype: "initialize",
		Agents: map[string]any{
			"reviewer": map[string]any{
				"prompt":               "be strict",
				"disallowed_tools":     []any{"Bash"},
				"mcp_servers":          []any{"docs"},
				"required_mcp_servers": []any{"docs"},
				"initial_prompt":       "start",
				"max_turns":            3,
				"permission_mode":      "plan",
			},
		},
	}))
	if err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	payload := inner.payload.(map[string]any)
	request := payload["request"].(map[string]any)
	agents := request["agents"].(map[string]any)
	reviewer := agents["reviewer"].(map[string]any)
	if reviewer["systemPrompt"] != "be strict" || reviewer["mcpServers"] == nil || reviewer["requiredMcpServers"] == nil {
		t.Fatalf("reviewer = %#v, want Claude camel agent fields", reviewer)
	}
	if _, exists := reviewer["mcp_servers"]; exists {
		t.Fatalf("mcp_servers should not be emitted for Claude wire: %#v", reviewer)
	}
}

func TestControlCodecNormalizesInitializeResponseFromClaude(t *testing.T) {
	inner := &captureTransport{}
	codec := newControlCodecTransport(inner)
	if err := codec.WriteJSON(protocol.NewControlRequestEnvelope("init-1", protocol.ControlRequest{Subtype: "initialize"})); err != nil {
		t.Fatalf("WriteJSON(request) error = %v", err)
	}
	inner.readPayload = map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": "init-1",
			"response": map[string]any{
				"outputStyle":           "default",
				"availableOutputStyles": []any{"default"},
				"commands": []any{
					map[string]any{"name": "review", "argumentHint": "<target>"},
				},
				"models": []any{
					map[string]any{"value": "glm-5.1", "displayName": "GLM 5.1"},
				},
				"account": map[string]any{
					"subscriptionType": "pro",
					"apiProvider":      "anthropic",
					"apiKeySource":     "env",
				},
			},
		},
	}

	payload, err := codec.ReadJSON()
	if err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	body := payload["response"].(map[string]any)["response"].(map[string]any)
	if body["output_style"] != "default" || body["available_output_styles"] == nil {
		t.Fatalf("initialize body = %#v, want snake output fields", body)
	}
	command := body["commands"].([]map[string]any)[0]
	if command["argument_hint"] != "<target>" {
		t.Fatalf("command = %#v, want snake argument_hint", command)
	}
	model := body["models"].([]map[string]any)[0]
	if model["display_name"] != "GLM 5.1" {
		t.Fatalf("model = %#v, want snake display_name", model)
	}
	account := body["account"].(map[string]any)
	if account["subscription_type"] != "pro" || account["api_provider"] != "anthropic" || account["api_key_source"] != "env" {
		t.Fatalf("account = %#v, want snake account fields", account)
	}
}

func TestControlCodecNormalizesSettingsResponseFromClaude(t *testing.T) {
	inner := &captureTransport{}
	codec := newControlCodecTransport(inner)
	if err := codec.WriteJSON(protocol.NewControlRequestEnvelope("settings-1", protocol.ControlRequest{Subtype: "get_settings"})); err != nil {
		t.Fatalf("WriteJSON(request) error = %v", err)
	}
	inner.readPayload = map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": "settings-1",
			"response": map[string]any{
				"effective": map[string]any{
					"permissionMode":    "acceptEdits",
					"maxThinkingTokens": float64(512),
					"allowedTools":      []any{"Read"},
					"disallowedTools":   []any{"Bash"},
				},
			},
		},
	}

	payload, err := codec.ReadJSON()
	if err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	effective := payload["response"].(map[string]any)["response"].(map[string]any)["effective"].(map[string]any)
	if effective["permission_mode"] != "acceptEdits" || effective["max_thinking_tokens"] != float64(512) {
		t.Fatalf("effective = %#v, want snake settings fields", effective)
	}
	if _, exists := effective["permissionMode"]; exists {
		t.Fatalf("permissionMode should be normalized away: %#v", effective)
	}
}

func TestControlCodecFormatsHookCallbackResponseForClaude(t *testing.T) {
	inner := &captureTransport{}
	codec := newControlCodecTransport(inner)

	if err := codec.WriteJSON(protocol.NewControlRequestEnvelope("hook-1", protocol.ControlRequest{
		Subtype: "hook_callback",
	})); err != nil {
		t.Fatalf("WriteJSON(request) error = %v", err)
	}
	if err := codec.WriteJSON(protocol.NewControlSuccessResponse("hook-1", map[string]any{
		"hook_specific_output": map[string]any{
			"hook_event_name":    "PostToolUse",
			"additional_context": "stop before the next tool",
			"decision": map[string]any{
				"behavior":      "allow",
				"updated_input": map[string]any{"command": "git status --short"},
				"updated_permissions": []any{
					map[string]any{
						"type":        "addRules",
						"behavior":    "allow",
						"destination": "session",
						"rules": []any{
							map[string]any{"tool_name": "Bash", "rule_content": "git status"},
						},
					},
				},
			},
		},
	})); err != nil {
		t.Fatalf("WriteJSON(response) error = %v", err)
	}

	payload := inner.payload.(protocol.ControlResponseEnvelope)
	body := payload.Response.Response
	specific := body["hookSpecificOutput"].(map[string]any)
	if specific["additionalContext"] != "stop before the next tool" {
		t.Fatalf("additionalContext = %#v", specific["additionalContext"])
	}
	if _, exists := specific["additional_context"]; exists {
		t.Fatalf("additional_context should not be emitted for Claude wire: %#v", specific)
	}
	decision := specific["decision"].(map[string]any)
	if decision["updatedInput"].(map[string]any)["command"] != "git status --short" {
		t.Fatalf("updatedInput = %#v", decision["updatedInput"])
	}
	updates := decision["updatedPermissions"].([]any)
	update := updates[0].(map[string]any)
	rules := update["rules"].([]any)
	rule := rules[0].(map[string]any)
	if rule["toolName"] != "Bash" || rule["ruleContent"] != "git status" {
		t.Fatalf("rule = %#v", rule)
	}
}

func TestNewProcessTransportSkipsClaudeCodecForSnakeWire(t *testing.T) {
	native := NewProcessTransport(ProcessConfig{ControlWireDialect: ControlWireDialectSnake})
	if _, ok := native.(*ProcessManager); !ok {
		t.Fatalf("native transport = %T, want *ProcessManager", native)
	}

	claude := NewProcessTransport(ProcessConfig{})
	if _, ok := claude.(*controlCodecTransport); !ok {
		t.Fatalf("claude transport = %T, want *controlCodecTransport", claude)
	}
}

func intPointer(value int) *int {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}
