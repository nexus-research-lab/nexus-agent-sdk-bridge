package hook

import (
	"reflect"
	"testing"
)

func TestEventConstantsMatchOfficialSDKSurface(t *testing.T) {
	got := []Event{
		EventPreToolUse,
		EventPostToolUse,
		EventPostToolUseFailure,
		EventNotification,
		EventUserPromptSubmit,
		EventSessionStart,
		EventSessionEnd,
		EventStop,
		EventStopFailure,
		EventSubagentStart,
		EventSubagentStop,
		EventPreCompact,
		EventPostCompact,
		EventPermissionRequest,
		EventSetup,
		EventTaskCreated,
		EventTaskCompleted,
		EventElicitation,
		EventElicitationResult,
		EventConfigChange,
		EventWorktreeCreate,
		EventWorktreeRemove,
		EventInstructionsLoaded,
		EventCwdChanged,
		EventFileChanged,
	}
	want := []string{
		"PreToolUse",
		"PostToolUse",
		"PostToolUseFailure",
		"Notification",
		"UserPromptSubmit",
		"SessionStart",
		"SessionEnd",
		"Stop",
		"StopFailure",
		"SubagentStart",
		"SubagentStop",
		"PreCompact",
		"PostCompact",
		"PermissionRequest",
		"Setup",
		"TaskCreated",
		"TaskCompleted",
		"Elicitation",
		"ElicitationResult",
		"ConfigChange",
		"WorktreeCreate",
		"WorktreeRemove",
		"InstructionsLoaded",
		"CwdChanged",
		"FileChanged",
	}
	if len(got) != len(want) {
		t.Fatalf("events length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if string(got[i]) != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOutputToMapDoesNotSerializeOnApplied(t *testing.T) {
	output := Output{
		SystemMessage: "continue",
		OnApplied:     func(AppliedAck) {},
	}
	if got, want := output.ToMap(), map[string]any{"system_message": "continue"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Output.ToMap() = %#v, want %#v", got, want)
	}
}
