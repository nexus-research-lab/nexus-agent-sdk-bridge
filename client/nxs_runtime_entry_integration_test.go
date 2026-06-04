package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	sdktools "github.com/nexus-research-lab/nexus-agent-sdk-bridge/tools"
)

func TestNXSRuntimeEntryIntegration(t *testing.T) {
	runtimePath := os.Getenv("NXS_RUNTIME_PATH")
	if runtimePath == "" {
		t.Skip("set NXS_RUNTIME_PATH to run the nxs runtime entry integration test")
	}

	configDir := t.TempDir()
	var startedCommand string
	var startedArgs []string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := NewSession(ctx, NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath(runtimePath).
		WithCWD(t.TempDir()).
		WithEnv(map[string]string{
			"NEXUS_CONFIG_DIR": configDir,
		}).
		WithModel("test-model").
		WithDiagnostics(func(event DiagnosticEvent) {
			if event.Event != "process_start" {
				return
			}
			if command, ok := event.Attributes["command_path"].(string); ok {
				startedCommand = command
			}
			if args, ok := event.Attributes["args"].([]string); ok {
				startedArgs = append([]string(nil), args...)
			}
		}))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if startedCommand != runtimePath {
		t.Fatalf("started command = %q, want %q", startedCommand, runtimePath)
	}
	assertArgValue(t, startedArgs, "--output-format", "stream-json")
	assertArgValue(t, startedArgs, "--input-format", "stream-json")
	assertArg(t, startedArgs, "--verbose")

	result, err := session.Control().InitializationResult(ctx)
	if err != nil {
		t.Fatalf("InitializationResult() error = %v", err)
	}
	if result.Raw["pid"] == nil {
		t.Fatalf("initialization result missing pid: %#v", result.Raw)
	}
}

func TestNXSRuntimeBundledEntryIntegration(t *testing.T) {
	if os.Getenv("NEXUS_NXS_RUNTIME_MANIFEST_URL") == "" {
		t.Skip("set NEXUS_NXS_RUNTIME_MANIFEST_URL to run the downloaded nxs runtime entry integration test")
	}
	cacheDir := t.TempDir()
	t.Setenv("NEXUS_NXS_RUNTIME_CACHE_DIR", cacheDir)

	configDir := t.TempDir()
	var startedCommand string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := NewSession(ctx, NewOptions().
		WithRuntime(RuntimeNXS).
		WithCWD(t.TempDir()).
		WithEnv(map[string]string{
			"NEXUS_CONFIG_DIR": configDir,
		}).
		WithModel("test-model").
		WithDiagnostics(func(event DiagnosticEvent) {
			if event.Event != "process_start" {
				return
			}
			if command, ok := event.Attributes["command_path"].(string); ok {
				startedCommand = command
			}
		}))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if !strings.HasPrefix(startedCommand, cacheDir) {
		t.Fatalf("started command = %q, want bundled runtime under %q", startedCommand, cacheDir)
	}
	result, err := session.Control().InitializationResult(ctx)
	if err != nil {
		t.Fatalf("InitializationResult() error = %v", err)
	}
	if result.Raw["pid"] == nil {
		t.Fatalf("initialization result missing pid: %#v", result.Raw)
	}
}

func TestNXSRuntimeBundledEntryWithSDKMCPServerIntegration(t *testing.T) {
	if os.Getenv("NEXUS_NXS_RUNTIME_MANIFEST_URL") == "" {
		t.Skip("set NEXUS_NXS_RUNTIME_MANIFEST_URL to run the downloaded nxs runtime SDK MCP integration test")
	}
	cacheDir := t.TempDir()
	t.Setenv("NEXUS_NXS_RUNTIME_CACHE_DIR", cacheDir)

	configDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := sdktools.CreateSDKMCPServer(sdktools.SDKMCPServerOptions{
		Name:    "test_tools",
		Version: "1.0.0",
	})
	session, err := NewSession(ctx, NewOptions().
		WithRuntime(RuntimeNXS).
		WithCWD(t.TempDir()).
		WithEnv(map[string]string{
			"NEXUS_CONFIG_DIR": configDir,
		}).
		WithModel("test-model").
		WithSDKMCPServer("test_tools", server))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	result, err := session.Control().InitializationResult(ctx)
	if err != nil {
		t.Fatalf("InitializationResult() error = %v", err)
	}
	if result.Raw["pid"] == nil {
		t.Fatalf("initialization result missing pid: %#v", result.Raw)
	}
}

func TestNXSRuntimeControlSurfaceIntegration(t *testing.T) {
	runtimePath := os.Getenv("NXS_RUNTIME_PATH")
	if runtimePath == "" {
		t.Skip("set NXS_RUNTIME_PATH to run the nxs runtime control integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := NewSession(ctx, NewOptions().
		WithRuntime(RuntimeNXS).
		WithCLIPath(runtimePath).
		WithCWD(t.TempDir()).
		WithEnv(map[string]string{
			"NEXUS_CONFIG_DIR": t.TempDir(),
		}).
		WithModel("test-model"))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if err := session.Control().SetModel(ctx, "runtime-model"); err != nil {
		t.Fatalf("SetModel() error = %v", err)
	}
	if err := session.Control().SetPermissionMode(ctx, permission.ModeAcceptEdits); err != nil {
		t.Fatalf("SetPermissionMode() error = %v", err)
	}
	if err := session.Control().SetMaxThinkingTokens(ctx, 512); err != nil {
		t.Fatalf("SetMaxThinkingTokens() error = %v", err)
	}
	if err := session.Control().ApplyFlagSettings(ctx, map[string]any{"theme": "compact", "model": "flag-model"}); err != nil {
		t.Fatalf("ApplyFlagSettings() error = %v", err)
	}

	usage, err := session.Control().ContextUsage(ctx)
	if err != nil {
		t.Fatalf("ContextUsage() error = %v", err)
	}
	if usage.Model != "runtime-model" {
		t.Fatalf("ContextUsage().Model = %q, want runtime-model; raw=%#v", usage.Model, usage.Raw)
	}

	status, err := session.MCP().Status(ctx)
	if err != nil {
		t.Fatalf("MCP().Status() error = %v", err)
	}
	if _, ok := status.Raw["mcp_servers"]; !ok {
		t.Fatalf("MCP().Status().Raw missing mcp_servers: %#v", status.Raw)
	}
	setResult, err := session.MCP().SetServers(ctx, map[string]mcp.ServerConfig{})
	if err != nil {
		t.Fatalf("MCP().SetServers() error = %v", err)
	}
	if setResult.Errors == nil || setResult.Raw["errors"] == nil {
		t.Fatalf("SetServers() = %#v, want errors map in decoded and raw response", setResult)
	}

	reload, err := session.Control().ReloadPlugins(ctx)
	if err != nil {
		t.Fatalf("ReloadPlugins() error = %v", err)
	}
	for _, key := range []string{"commands", "agents", "plugins", "mcp_servers", "error_count"} {
		if _, ok := reload.Raw[key]; !ok {
			t.Fatalf("ReloadPlugins().Raw missing %s: %#v", key, reload.Raw)
		}
	}
	commands, err := session.Control().SupportedCommands(ctx)
	if err != nil {
		t.Fatalf("SupportedCommands() after ReloadPlugins() error = %v", err)
	}
	if len(commands) != len(reload.Commands) {
		t.Fatalf("SupportedCommands() = %d commands, want %d after reload", len(commands), len(reload.Commands))
	}

	settings, err := session.Control().GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	effective := settings.Effective
	if effective["model"] != "runtime-model" || effective["permission_mode"] != "acceptEdits" {
		t.Fatalf("GetSettings().Effective = %#v, want runtime model and permission", effective)
	}
	if !numericSettingEquals(effective["max_thinking_tokens"], 512) {
		t.Fatalf("GetSettings().Effective max_thinking_tokens = %#v, want 512", effective["max_thinking_tokens"])
	}
}

func TestNXSRuntimeOpenAIProviderIntegration(t *testing.T) {
	runtimePath := os.Getenv("NXS_RUNTIME_PATH")
	if runtimePath == "" {
		t.Skip("set NXS_RUNTIME_PATH to run the nxs runtime OpenAI provider integration test")
	}

	var requestPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("OpenAI path = %q, want /v1/chat/completions", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-openai-key" {
			t.Fatalf("Authorization header = %q, want bearer test key", request.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode OpenAI request: %v", err)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		writeOpenAIStreamEventForTest(t, writer, map[string]any{
			"id":    "chatcmpl-nxs-1",
			"model": "gpt-fake",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"role": "assistant"},
			}},
		})
		writeOpenAIStreamEventForTest(t, writer, map[string]any{
			"id":    "chatcmpl-nxs-1",
			"model": "gpt-fake",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": "openai provider ok"},
			}},
		})
		writeOpenAIStreamEventForTest(t, writer, map[string]any{
			"id":    "chatcmpl-nxs-1",
			"model": "gpt-fake",
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		})
		if _, err := writer.Write([]byte("data: [DONE]\n\n")); err != nil {
			t.Fatalf("write OpenAI done event: %v", err)
		}
	}))
	defer server.Close()

	env := map[string]string{
		"NEXUS_CONFIG_DIR":      t.TempDir(),
		"NEXUS_API_PROVIDER":    "openai",
		"OPENAI_API_KEY":        "test-openai-key",
		"OPENAI_BASE_URL":       server.URL,
		"OPENAI_MODEL":          "gpt-fake",
		"OPENAI_CUSTOM_HEADERS": "",
	}
	var startedCommand string
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	stream, err := Query(ctx, QueryRequest{
		Prompt: "只输出 openai provider ok",
		Options: NewOptions().
			WithRuntime(RuntimeNXS).
			WithCLIPath(runtimePath).
			WithCWD(t.TempDir()).
			WithEnv(env).
			WithModel("gpt-fake").
			WithMaxTurns(1).
			WithDiagnostics(func(event DiagnosticEvent) {
				if event.Event != "process_start" {
					return
				}
				if command, ok := event.Attributes["command_path"].(string); ok {
					startedCommand = command
				}
			}),
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer func() {
		if err := stream.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	var text strings.Builder
	var result *protocol.ResultMessage
	for result == nil {
		message, err := stream.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		if message.Assistant != nil {
			for _, block := range message.Assistant.Message.Content {
				if textBlock, ok := protocol.AsTextBlock(block); ok {
					text.WriteString(textBlock.Text)
				}
			}
		}
		if message.Result != nil {
			copied := *message.Result
			result = &copied
		}
	}

	if startedCommand != runtimePath {
		t.Fatalf("started command = %q, want %q", startedCommand, runtimePath)
	}
	output := strings.TrimSpace(text.String())
	if output == "" {
		output = strings.TrimSpace(result.Result)
	}
	if !strings.Contains(output, "openai provider ok") {
		t.Fatalf("agent output = %q, want OpenAI fake provider response; result=%#v", output, result)
	}
	if requestPayload["model"] != "gpt-fake" {
		t.Fatalf("OpenAI request model = %#v, want gpt-fake", requestPayload["model"])
	}
	if requestPayload["stream"] != true {
		t.Fatalf("OpenAI request stream = %#v, want true", requestPayload["stream"])
	}
}

func TestNXSRuntimeLiveAgentEffect(t *testing.T) {
	runtimePath := os.Getenv("NXS_RUNTIME_PATH")
	if runtimePath == "" {
		t.Skip("set NXS_RUNTIME_PATH to run the nxs runtime live test")
	}
	envFile := os.Getenv("NXS_LIVE_ENV_FILE")
	if envFile == "" {
		t.Skip("set NXS_LIVE_ENV_FILE to run the nxs runtime live test")
	}
	liveEnv, err := loadDotEnvForTest(envFile)
	if err != nil {
		t.Fatalf("load live env: %v", err)
	}
	if strings.TrimSpace(liveEnv["ANTHROPIC_API_KEY"]) == "" {
		t.Skip("ANTHROPIC_API_KEY is empty")
	}
	if strings.TrimSpace(liveEnv["ANTHROPIC_BASE_URL"]) == "" {
		t.Skip("ANTHROPIC_BASE_URL is empty")
	}

	configDir := t.TempDir()
	liveEnv["NEXUS_CONFIG_DIR"] = configDir
	var startedCommand string
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	stream, err := Query(ctx, QueryRequest{
		Prompt: "请只用一句中文回答：nxs runtime 已经真实连通并完成一次 agent 响应。不要调用工具。",
		Options: NewOptions().
			WithRuntime(RuntimeNXS).
			WithCLIPath(runtimePath).
			WithCWD(t.TempDir()).
			WithEnv(liveEnv).
			WithModel(firstNonEmptyForTest(liveEnv["NEXUS_EXAMPLE_MODEL"], liveEnv["ANTHROPIC_MODEL"], liveEnv["NEXUS_MODEL"], liveEnv["MODEL"], "glm-5.1")).
			WithMaxTurns(1).
			WithDiagnostics(func(event DiagnosticEvent) {
				if event.Event != "process_start" {
					return
				}
				if command, ok := event.Attributes["command_path"].(string); ok {
					startedCommand = command
				}
			}),
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer func() {
		if err := stream.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	var text strings.Builder
	var result *protocol.ResultMessage
	for result == nil {
		message, err := stream.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		if message.Assistant != nil {
			for _, block := range message.Assistant.Message.Content {
				if textBlock, ok := protocol.AsTextBlock(block); ok {
					text.WriteString(textBlock.Text)
				}
			}
		}
		if message.Result != nil {
			copied := *message.Result
			result = &copied
		}
	}

	if startedCommand != runtimePath {
		t.Fatalf("started command = %q, want %q", startedCommand, runtimePath)
	}
	output := strings.TrimSpace(text.String())
	if output == "" {
		output = strings.TrimSpace(result.Result)
	}
	if output == "" {
		t.Fatalf("agent output is empty; result=%#v", result)
	}
	t.Logf("runtime=%s", startedCommand)
	t.Logf("agent_output=%s", output)
	t.Logf("result_subtype=%s total_cost_usd=%f", result.Subtype, result.TotalCostUSD)
}

func writeOpenAIStreamEventForTest(t *testing.T, writer http.ResponseWriter, payload map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal OpenAI event: %v", err)
	}
	if _, err := writer.Write([]byte("data: ")); err != nil {
		t.Fatalf("write OpenAI event prefix: %v", err)
	}
	if _, err := writer.Write(encoded); err != nil {
		t.Fatalf("write OpenAI event payload: %v", err)
	}
	if _, err := writer.Write([]byte("\n\n")); err != nil {
		t.Fatalf("write OpenAI event suffix: %v", err)
	}
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func loadDotEnvForTest(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		env[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return env, nil
}

func firstNonEmptyForTest(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func numericSettingEquals(value any, expected int) bool {
	switch typed := value.(type) {
	case float64:
		return int(typed) == expected
	case int:
		return typed == expected
	case json.Number:
		return typed.String() == strconv.Itoa(expected)
	case string:
		return typed == strconv.Itoa(expected)
	default:
		return false
	}
}
