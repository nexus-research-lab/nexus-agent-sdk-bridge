// INPUT: Claude Code 原生受限启动选项。
// OUTPUT: 类型化准入、--restricted 参数、能力和重启证据。
// POS: 仅覆盖 Bridge 适配边界，不证明 Claude SDK 的 IO 隔离。
package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func claudeRestrictedOptions() Options {
	return NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath("claude").
		WithSandbox(SandboxSettings{RequireClaudeRestricted: true})
}

func TestClaudeRestrictedLaunchContractAddsFlag(t *testing.T) {
	options := claudeRestrictedOptions()
	resolved, err := options.buildResolvedOptions(true)
	if err != nil {
		t.Fatal(err)
	}
	if got := countArg(buildProcessTransportArgs(resolved), "--restricted"); got != 1 {
		t.Fatalf("Claude args contain --restricted %d times, want once: %v", got, buildProcessTransportArgs(resolved))
	}
	if _, leaked := sandboxSettingsMap(resolved.Sandbox)["requireClaudeRestricted"]; leaked {
		t.Fatal("typed host requirement leaked into Claude ordinary settings")
	}
	fingerprint, err := options.OptionsFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	without := NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude")
	withoutFingerprint, err := without.OptionsFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint.SandboxContract == withoutFingerprint.SandboxContract {
		t.Fatal("Claude restricted requirement was omitted from restart-sensitive fingerprint")
	}
	if config := options.processConfig(); !config.RequireClaudeRestricted {
		t.Fatal("Claude restricted process admission did not carry the typed probe requirement")
	}
}

func TestClaudeRestrictedCapabilityIsTypedAndRuntimeScoped(t *testing.T) {
	core := newSessionCore(claudeRestrictedOptions())
	if !core.supports(CapabilityClaudeRestricted) {
		t.Fatal("Claude restricted capability not reported for typed launch contract")
	}
	fullAccess := NewOptions().
		WithRuntime(RuntimeClaude).
		WithCLIPath("claude").
		WithPermissionMode(permission.ModeBypassPermissions)
	fullCore := newSessionCore(fullAccess)
	if fullCore.supports(CapabilityClaudeRestricted) {
		t.Fatal("Full Access unexpectedly claimed Claude restricted capability")
	}
	resolved, err := fullAccess.buildResolvedOptions(true)
	if err != nil {
		t.Fatal(err)
	}
	if countArg(buildProcessTransportArgs(resolved), "--restricted") != 0 {
		t.Fatal("Full Access unexpectedly received --restricted")
	}
	if config := fullAccess.processConfig(); config.RequireClaudeRestricted {
		t.Fatal("Full Access unexpectedly enabled the Claude restricted probe")
	}
}

func TestClaudeRestrictedRejectsUnsupportedOrConflictingOptions(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		wantCap Capability
		wantErr string
	}{
		{
			name:    "nxs",
			options: NewOptions().WithRuntime(RuntimeNXS).WithCLIPath("nxs").WithSandbox(SandboxSettings{RequireClaudeRestricted: true}),
			wantCap: CapabilityClaudeRestricted,
		},
		{
			name: "bypass",
			options: claudeRestrictedOptions().WithPermissionMode(
				permission.ModeBypassPermissions,
			),
			wantErr: "cannot be combined with bypass permissions",
		},
		{
			name:    "untyped extra",
			options: NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude").WithExtraBoolArgs("restricted"),
			wantErr: "must be requested with SandboxSettings.RequireClaudeRestricted",
		},
		{
			name:    "duplicate typed extra",
			options: claudeRestrictedOptions().WithExtraBoolArgs("--restricted"),
			wantErr: "exactly once",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.options.normalized()
			if err == nil {
				t.Fatal("invalid Claude restricted options were accepted")
			}
			if test.wantCap != "" {
				var unsupported *UnsupportedCapabilityError
				if !errors.As(err, &unsupported) || unsupported.Capability != test.wantCap {
					t.Fatalf("error = %v, want %s capability rejection", err, test.wantCap)
				}
			}
			if test.wantErr != "" && !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestClaudeRestrictedChangeRequiresRuntimeReplacement(t *testing.T) {
	current := NewOptions().WithRuntime(RuntimeClaude).WithCLIPath("claude")
	next := claudeRestrictedOptions()
	if reason, required := restartReasonForReconfigure(current, next); !required || reason != RestartReasonSandboxPolicyChanged {
		t.Fatalf("restricted launch change = (%s, %v), want sandbox policy restart", reason, required)
	}
}

func TestClaudeRestrictedInvalidOptionFailsBeforeTransport(t *testing.T) {
	transport := newScriptedTransport()
	core := newSessionCoreWithTransport(Options{
		Transport: transport,
		Runtime: RuntimeOptions{
			Kind:           RuntimeClaude,
			PermissionMode: permission.ModeBypassPermissions,
		},
		Sandbox: &SandboxSettings{RequireClaudeRestricted: true},
	}, transport)
	if err := core.Connect(context.Background()); err == nil {
		t.Fatal("bypass and Claude restricted were admitted")
	}
	if core.isConnected() {
		t.Fatal("invalid Claude restricted session remained connected")
	}
	select {
	case write := <-transport.writes:
		t.Fatalf("invalid Claude restricted option reached transport: %v", write)
	default:
	}
}

func countArg(args []string, want string) int {
	count := 0
	for _, arg := range args {
		if arg == want {
			count++
		}
	}
	return count
}
