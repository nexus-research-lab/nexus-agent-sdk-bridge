package client

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

func TestRuntimeLaunchSnapshotRedactsSensitiveValues(t *testing.T) {
	options := NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath("claude").
		WithCWD("/work").
		WithSystemPrompt("secret prompt").
		WithEnv(map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "secret-token",
			"ANTHROPIC_MODEL":      "glm-4.5-air",
		}).
		WithMCPServer("remote", mcpHTTPServerForSnapshotTest())

	snapshot, err := options.RuntimeLaunchSnapshot()
	if err != nil {
		t.Fatalf("RuntimeLaunchSnapshot() error = %v", err)
	}
	if snapshot.Transport != "process" || snapshot.CommandPath != "claude" {
		t.Fatalf("snapshot launch = %+v", snapshot)
	}
	if !stringSliceContains(snapshot.EnvKeys, "ANTHROPIC_AUTH_TOKEN") ||
		!stringSliceContains(snapshot.EnvKeys, "CLAUDE_CONFIG_DIR") ||
		!stringSliceContains(snapshot.EnvKeys, "NEXUS_CONFIG_DIR") {
		t.Fatalf("env keys = %+v", snapshot.EnvKeys)
	}
	if !stringSliceContains(snapshot.ExplicitEnvKeys, "ANTHROPIC_AUTH_TOKEN") {
		t.Fatalf("explicit env keys = %+v", snapshot.ExplicitEnvKeys)
	}
	if !stringSliceContains(snapshot.SDKEnvKeys, "CLAUDE_CONFIG_DIR") {
		t.Fatalf("sdk env keys = %+v", snapshot.SDKEnvKeys)
	}
	if !stringSliceContains(snapshot.Args, "<redacted>") {
		t.Fatalf("args should contain redacted system prompt: %+v", snapshot.Args)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	raw := string(payload)
	for _, forbidden := range []string{"secret-token", "secret prompt", "top-secret"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("snapshot leaked %q: %s", forbidden, raw)
		}
	}
	if snapshot.Fingerprint.ProcessEnv == "" ||
		snapshot.Fingerprint.ToolPolicy == "" ||
		snapshot.Fingerprint.RuntimeControls == "" {
		t.Fatalf("fingerprint is incomplete: %+v", snapshot.Fingerprint)
	}
}

func TestOptionsFingerprintChangesWhenRestartSensitiveOptionsChange(t *testing.T) {
	current, err := NewOptions().
		WithCLIPath("nxs").
		WithAllowedTools("Read").
		WithEnv(map[string]string{"ANTHROPIC_AUTH_TOKEN": "old-token"}).
		OptionsFingerprint()
	if err != nil {
		t.Fatalf("current fingerprint: %v", err)
	}
	next, err := NewOptions().
		WithCLIPath("nxs").
		WithAllowedTools("Read", "Write").
		WithEnv(map[string]string{"ANTHROPIC_AUTH_TOKEN": "new-token"}).
		OptionsFingerprint()
	if err != nil {
		t.Fatalf("next fingerprint: %v", err)
	}
	if current.ProcessEnv == next.ProcessEnv {
		t.Fatal("process env fingerprint should change")
	}
	if current.ToolPolicy == next.ToolPolicy {
		t.Fatal("tool policy fingerprint should change")
	}
	if current.RestartSensitive == next.RestartSensitive {
		t.Fatal("restart-sensitive fingerprint should change")
	}
}

func TestOptionsFingerprintChangesWhenSkillConfigChanges(t *testing.T) {
	current, err := NewOptions().
		WithCLIPath("nxs").
		WithSkills("imagegen").
		WithAdditionalDirectories("/tmp/platform-skills").
		OptionsFingerprint()
	if err != nil {
		t.Fatalf("current fingerprint: %v", err)
	}
	next, err := NewOptions().
		WithCLIPath("nxs").
		WithSkills("imagegen", "ima-skill").
		WithAdditionalDirectories("/tmp/platform-skills").
		OptionsFingerprint()
	if err != nil {
		t.Fatalf("next fingerprint: %v", err)
	}
	if current.RestartSensitive == next.RestartSensitive {
		t.Fatal("skill config should change restart-sensitive fingerprint")
	}
}

func TestRuntimeLaunchSnapshotDirectConnectRedactsToken(t *testing.T) {
	snapshot, err := NewOptions().
		WithDirectConnect(DirectConnectOptions{
			URL:        "cc://127.0.0.1:1234/secret-token",
			SessionKey: "session-key",
		}).
		RuntimeLaunchSnapshot()
	if err != nil {
		t.Fatalf("RuntimeLaunchSnapshot() error = %v", err)
	}
	if snapshot.Transport != "direct_connect" || snapshot.DirectConnect == nil {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if !snapshot.DirectConnect.AuthTokenPresent || snapshot.DirectConnect.AuthTokenFingerprint == "" {
		t.Fatalf("direct connect auth token fingerprint missing: %+v", snapshot.DirectConnect)
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(payload), "secret-token") {
		t.Fatalf("snapshot leaked direct-connect token: %s", payload)
	}
}

func mcpHTTPServerForSnapshotTest() mcp.HTTPServerConfig {
	return mcp.HTTPServerConfig{
		URL: "https://example.test/mcp",
		Headers: map[string]string{
			"Authorization": "top-secret",
		},
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
