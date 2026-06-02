package protocol

import "strings"

// TerminalCategory 表示运行时终止结果的跨 provider 分类。
type TerminalCategory string

const (
	TerminalCategorySuccess     TerminalCategory = "success"
	TerminalCategoryInterrupted TerminalCategory = "interrupted"
	TerminalCategoryLimit       TerminalCategory = "limit"
	TerminalCategoryError       TerminalCategory = "error"
	TerminalCategoryCancelled   TerminalCategory = "cancelled"
	TerminalCategoryUnknown     TerminalCategory = "unknown"
)

// ClassifyTerminal 将底层 subtype / terminal reason 归类为稳定终止类型。
func ClassifyTerminal(subtype string, terminalReason string) TerminalCategory {
	candidates := []string{normalizeTerminalText(subtype), normalizeTerminalText(terminalReason)}
	for _, candidate := range candidates {
		switch candidate {
		case "success", "completed", "complete", "end_turn", "stop":
			return TerminalCategorySuccess
		case "interrupted", "interrupt", "user_interrupt", "user_interrupted", "cancelled_by_user", "canceled_by_user":
			return TerminalCategoryInterrupted
		case "max_turns", "max_tokens", "max_output_tokens", "context_limit", "context_length", "token_limit", "limit":
			return TerminalCategoryLimit
		case "cancelled", "canceled", "abort", "aborted":
			return TerminalCategoryCancelled
		case "error", "failed", "failure", "api_error":
			return TerminalCategoryError
		}
	}
	return TerminalCategoryUnknown
}

// IsRetryable 判断终止类型是否通常值得自动重试。
func (c TerminalCategory) IsRetryable() bool {
	return c == TerminalCategoryError || c == TerminalCategoryLimit || c == TerminalCategoryUnknown
}

// IsUserInterrupted 判断终止类型是否来自用户显式中断。
func (c TerminalCategory) IsUserInterrupted() bool {
	return c == TerminalCategoryInterrupted || c == TerminalCategoryCancelled
}

// TerminalCategory 返回 result 的归一化终止分类。
func (m ResultMessage) TerminalCategory() TerminalCategory {
	category := ClassifyTerminal(m.Subtype, m.TerminalReason)
	if category == TerminalCategoryUnknown && m.IsError {
		return TerminalCategoryError
	}
	return category
}

func normalizeTerminalText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
