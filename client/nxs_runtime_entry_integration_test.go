package client

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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
