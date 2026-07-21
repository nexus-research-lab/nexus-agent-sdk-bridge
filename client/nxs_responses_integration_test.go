// 本文件验证 bridge 通过真实 nxs 子进程热切换到 OpenAI Responses。

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNXSResponsesProtocolHotReconfigure 使用显式本地 nxs binary 跑完整 process/control/provider 主链。
func TestNXSResponsesProtocolHotReconfigure(t *testing.T) {
	commandPath := strings.TrimSpace(os.Getenv("NEXUS_TEST_NXS_RESPONSES_COMMAND"))
	if commandPath == "" {
		t.Skip("set NEXUS_TEST_NXS_RESPONSES_COMMAND to run the nxs Responses integration test")
	}

	var (
		mu           sync.Mutex
		requestPaths []string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestPaths = append(requestPaths, request.URL.Path)
		mu.Unlock()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode upstream request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload["store"] != false || payload["model"] != "gpt-test" {
			t.Errorf("Responses payload = %#v", payload)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		response := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_bridge_integration", "model": "gpt-test", "status": "completed",
				"output": []any{map[string]any{
					"type": "message", "id": "msg_bridge_integration", "role": "assistant", "status": "completed",
					"content": []any{map[string]any{"type": "output_text", "text": "bridge-responses-ok"}},
				}},
				"usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
			},
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Errorf("encode upstream response: %v", err)
			return
		}
		_, _ = writer.Write([]byte("data: " + string(encoded) + "\n\n"))
	}))
	defer upstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cwd := t.TempDir()
	configDir := t.TempDir()
	optionsForProtocol := func(protocol string) Options {
		return NewOptions().
			WithRuntime(RuntimeNXS).
			WithCLIPath(commandPath).
			WithCWD(cwd).
			WithEnv(map[string]string{
				"NEXUS_API_PROVIDER":    "openai",
				"NEXUS_OPENAI_PROTOCOL": protocol,
				"OPENAI_BASE_URL":       upstream.URL,
				"OPENAI_API_KEY":        "test-key",
				"OPENAI_MODEL":          "gpt-test",
				"NEXUS_CONFIG_DIR":      configDir,
			})
	}
	session, err := NewSession(ctx, optionsForProtocol("chat_completions"))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() { _ = session.Close(context.Background()) }()
	if err := session.Reconfigure(ctx, optionsForProtocol("responses")); err != nil {
		t.Fatalf("Reconfigure(responses) error = %v", err)
	}
	stream, err := session.Send(ctx, "Reply with exactly: bridge-responses-ok")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if result.IsError || !strings.Contains(result.Result, "bridge-responses-ok") {
		t.Fatalf("result = %#v", result)
	}
	mu.Lock()
	paths := append([]string(nil), requestPaths...)
	mu.Unlock()
	if len(paths) != 1 || paths[0] != "/v1/responses" {
		t.Fatalf("upstream request paths = %#v, want [/v1/responses]", paths)
	}
}
