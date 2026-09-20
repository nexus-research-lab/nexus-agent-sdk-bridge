// INPUT: Claude native sandbox typed settings and ordinary CLI overrides。
// OUTPUT: Native command sandbox keeps Bash available while rejecting bypasses。
// POS: Bridge public contract and restart fingerprint regression.
package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func claudeNativeSandboxOptions() Options {
	enabled := true
	failIfUnavailable := true
	allowUnsandboxed := false
	return NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath("claude").
		WithSandbox(SandboxSettings{
			RequireClaudeNativeSandbox: true,
			Enabled:                    &enabled,
			FailIfUnavailable:          &failIfUnavailable,
			AllowUnsandboxedCommands:   &allowUnsandboxed,
			AutoAllowBashIfSandboxed:   &enabled,
		})
}

func TestClaudeNativeSandboxPreservesBashAndInstallsFailClosedSettings(t *testing.T) {
	options := claudeNativeSandboxOptions()
	resolved, err := options.buildResolvedOptions(true)
	if err != nil {
		t.Fatal(err)
	}
	args := buildProcessTransportArgs(resolved)
	if countArg(args, "--restricted") != 0 {
		t.Fatalf("native command sandbox unexpectedly removed Claude tools: %v", args)
	}
	settings := buildProcessTransportSettingsValue(resolved)
	for _, expected := range []string{`"enabled":true`, `"failIfUnavailable":true`, `"allowUnsandboxedCommands":false`} {
		if !strings.Contains(settings, expected) {
			t.Fatalf("settings %q missing %s", settings, expected)
		}
	}
	config := options.processConfig()
	if !config.RequireClaudeNativeSandbox || config.RequireClaudeRestricted {
		t.Fatalf("native process contract = %+v", config)
	}
	if !newSessionCore(options).supports(CapabilityClaudeNativeSandbox) {
		t.Fatal("native sandbox capability was not exposed")
	}
}

func TestClaudeNativeSandboxRejectsBypassAndWeakSettings(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantErr string
	}{
		{
			name:    "bypass mode",
			options: claudeNativeSandboxOptions().WithPermissionMode(permission.ModeBypassPermissions),
			wantErr: "cannot be combined with bypass permissions",
		},
		{
			name: "disabled",
			options: NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude").WithSandbox(SandboxSettings{
				RequireClaudeNativeSandbox: true,
				FailIfUnavailable:          boolPointer(true),
				AllowUnsandboxedCommands:   boolPointer(false),
			}),
			wantErr: "requires sandbox.enabled=true",
		},
		{
			name:    "override settings",
			options: claudeNativeSandboxOptions().WithExtraArgs(map[string]string{"settings": "{}"}),
			wantErr: "cannot be overridden",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.options.normalized()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestClaudeNativeSandboxRejectsOtherRuntimeAndOldRestrictedMode(t *testing.T) {
	for name, options := range map[string]Options{
		"nxs": NewOptions().WithRuntime(RuntimeNXS).WithCLIPath("nxs").WithSandbox(SandboxSettings{
			RequireClaudeNativeSandbox: true,
			Enabled:                    boolPointer(true),
			FailIfUnavailable:          boolPointer(true),
			AllowUnsandboxedCommands:   boolPointer(false),
		}),
		"mixed": claudeNativeSandboxOptions().WithSandbox(SandboxSettings{
			RequireClaudeNativeSandbox: true,
			RequireClaudeRestricted:    true,
			Enabled:                    boolPointer(true),
			FailIfUnavailable:          boolPointer(true),
			AllowUnsandboxedCommands:   boolPointer(false),
		}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := options.normalized()
			if err == nil {
				t.Fatal("invalid native sandbox contract accepted")
			}
			var unsupported *UnsupportedCapabilityError
			if name == "nxs" && (!errors.As(err, &unsupported) || unsupported.Capability != CapabilityClaudeNativeSandbox) {
				t.Fatalf("error = %v, want native capability rejection", err)
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }
