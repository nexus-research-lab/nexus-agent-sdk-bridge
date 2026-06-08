package client

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestClaudeRuntimeEntryPromptBeforeSystemInitIntegration(t *testing.T) {
	if os.Getenv("NEXUS_CLAUDE_RUNTIME_INTEGRATION") == "" {
		t.Skip("set NEXUS_CLAUDE_RUNTIME_INTEGRATION=1 to run the Claude runtime entry integration test")
	}

	commandPath := strings.TrimSpace(os.Getenv("CLAUDE_RUNTIME_PATH"))
	if commandPath == "" {
		commandPath = strings.TrimSpace(os.Getenv("NEXUS_CLAUDE_COMMAND_PATH"))
	}
	if commandPath == "" {
		resolved, err := exec.LookPath("claude")
		if err != nil {
			t.Skip("claude command not found")
		}
		commandPath = resolved
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := NewSession(ctx, NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath(commandPath).
		WithCWD(t.TempDir()).
		WithEnv(map[string]string{
			"CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK": "1",
			"NEXUS_CONFIG_DIR":                    t.TempDir(),
		}).
		WithModel("glm-5.1").
		WithPermissionMode(permission.ModeBypassPermissions))
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() {
		if err := session.Close(context.Background()); err != nil {
			t.Logf("Close() error after runtime smoke: %v", err)
		}
	}()

	stream, err := session.Send(ctx, "只回复 OK")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	for {
		message, err := stream.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		if message.Type == protocol.MessageTypeSystem && message.Subtype == "init" {
			if message.SessionID == "" {
				t.Fatalf("system init missing session id: %#v", message.Raw)
			}
			if got := session.ID(); got == "" {
				t.Fatalf("session ID not populated after system init")
			}
			return
		}
		if message.Type == protocol.MessageTypeResult {
			t.Fatalf("received result before system init: %#v", message.Raw)
		}
	}
}
