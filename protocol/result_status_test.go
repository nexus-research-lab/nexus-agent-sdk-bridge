package protocol

import "testing"

func TestClassifyTerminal(t *testing.T) {
	cases := []struct {
		name           string
		subtype        string
		terminalReason string
		want           TerminalCategory
	}{
		{name: "success subtype", subtype: "success", want: TerminalCategorySuccess},
		{name: "completed reason", terminalReason: "completed", want: TerminalCategorySuccess},
		{name: "interrupted", terminalReason: "user_interrupt", want: TerminalCategoryInterrupted},
		{name: "limit", terminalReason: "max_output_tokens", want: TerminalCategoryLimit},
		{name: "cancelled", subtype: "cancelled", want: TerminalCategoryCancelled},
		{name: "error", subtype: "error", want: TerminalCategoryError},
		{name: "unknown", subtype: "mystery", want: TerminalCategoryUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyTerminal(tc.subtype, tc.terminalReason); got != tc.want {
				t.Fatalf("ClassifyTerminal() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResultMessageTerminalCategoryUsesIsErrorFallback(t *testing.T) {
	result := ResultMessage{Subtype: "mystery", IsError: true}
	if got := result.TerminalCategory(); got != TerminalCategoryError {
		t.Fatalf("TerminalCategory() = %q, want error", got)
	}
	if !TerminalCategoryInterrupted.IsUserInterrupted() {
		t.Fatal("interrupted IsUserInterrupted() = false, want true")
	}
	if !TerminalCategoryError.IsRetryable() {
		t.Fatal("error IsRetryable() = false, want true")
	}
}
