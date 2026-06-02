package protocol

import "github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"

// TokenUsage 表示跨 provider 归一化后的 token 使用量。
type TokenUsage struct {
	InputTokens              int64          `json:"input_tokens,omitempty"`
	OutputTokens             int64          `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int64          `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64          `json:"cache_read_input_tokens,omitempty"`
	ReasoningTokens          int64          `json:"reasoning_tokens,omitempty"`
	TotalTokens              int64          `json:"total_tokens,omitempty"`
	Raw                      map[string]any `json:"raw,omitempty"`
}

// ParseTokenUsage 将动态 usage JSON 解析为稳定的 token 结构。
func ParseTokenUsage(raw any) (TokenUsage, bool) {
	payload := jsonvalue.MapValue(raw)
	if len(payload) == 0 {
		return TokenUsage{}, false
	}

	usage := TokenUsage{Raw: jsonvalue.CloneMap(payload)}
	var matched bool
	read := func(keys ...string) int64 {
		for _, key := range keys {
			if value, ok := jsonvalue.Int64Value(payload[key]); ok {
				matched = true
				return value
			}
		}
		return 0
	}

	usage.InputTokens = read("input_tokens", "prompt_tokens")
	usage.OutputTokens = read("output_tokens", "completion_tokens")
	usage.CacheCreationInputTokens = read("cache_creation_input_tokens", "cache_creation_tokens")
	usage.CacheReadInputTokens = read("cache_read_input_tokens", "cache_read_tokens")
	usage.ReasoningTokens = read("reasoning_tokens", "reasoning_output_tokens")
	usage.TotalTokens = read("total_tokens")
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens + usage.ReasoningTokens
	}
	return usage, matched
}

// TokenUsageFromResult 从 result message 中提取 usage；优先使用顶层 usage，缺失时聚合 model_usage。
func TokenUsageFromResult(result ResultMessage) (TokenUsage, bool) {
	if usage, ok := ParseTokenUsage(result.Usage); ok {
		return usage, true
	}

	modelUsage := jsonvalue.MapValue(result.ModelUsage)
	if len(modelUsage) == 0 {
		return TokenUsage{}, false
	}
	total := TokenUsage{Raw: jsonvalue.CloneMap(modelUsage)}
	var matched bool
	for _, raw := range modelUsage {
		usage, ok := ParseTokenUsage(raw)
		if !ok {
			continue
		}
		total = total.Add(usage)
		matched = true
	}
	return total, matched
}

// Add 合并两段 usage。
func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
	u.ReasoningTokens += other.ReasoningTokens
	u.TotalTokens += other.TotalTokens
	if u.Raw == nil && other.Raw != nil {
		u.Raw = jsonvalue.CloneMap(other.Raw)
	}
	return u
}

// IsZero 判断 usage 是否为空。
func (u TokenUsage) IsZero() bool {
	return u.InputTokens == 0 &&
		u.OutputTokens == 0 &&
		u.CacheCreationInputTokens == 0 &&
		u.CacheReadInputTokens == 0 &&
		u.ReasoningTokens == 0 &&
		u.TotalTokens == 0
}

// TokenUsage 返回 result 的归一化 token usage。
func (m ResultMessage) TokenUsage() (TokenUsage, bool) {
	return TokenUsageFromResult(m)
}
