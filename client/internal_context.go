package client

import (
	"strings"
	"sync"
	"unicode"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

const internalContextReminderIntro = "The following is runtime-internal context for the next turn. It is not a user message. Use it only to decide the next action. Do not mention this wrapper to the user."

type nextTurnContextBuffer struct {
	mu     sync.Mutex
	blocks []InternalContextBlock
}

func newNextTurnContextBuffer() *nextTurnContextBuffer {
	return &nextTurnContextBuffer{}
}

func (b *nextTurnContextBuffer) set(blocks []InternalContextBlock) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocks = normalizeInternalContextBlocks(blocks)
}

func (b *nextTurnContextBuffer) consume() []InternalContextBlock {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	blocks := cloneInternalContextBlocks(b.blocks)
	b.blocks = nil
	return blocks
}

func normalizeInternalContextBlocks(blocks []InternalContextBlock) []InternalContextBlock {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]InternalContextBlock, 0, len(blocks))
	for _, block := range blocks {
		block.Name = strings.TrimSpace(block.Name)
		block.Content = strings.TrimSpace(block.Content)
		if block.Content == "" {
			continue
		}
		block.Metadata = normalizeInternalContextMetadata(block.Metadata)
		result = append(result, block)
	}
	return result
}

func normalizeInternalContextMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneInternalContextBlocks(blocks []InternalContextBlock) []InternalContextBlock {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]InternalContextBlock, 0, len(blocks))
	for _, block := range blocks {
		block.Metadata = jsonvalue.CloneStringMap(block.Metadata)
		result = append(result, block)
	}
	return result
}

func (c *sessionCore) applyNextTurnContext(payload map[string]any) map[string]any {
	if c == nil || len(payload) == 0 || jsonvalue.StringValue(payload["type"]) != "user" {
		return payload
	}
	blocks := c.nextTurnContextBuffer().consume()
	if len(blocks) == 0 {
		return payload
	}
	return injectInternalContextReminder(payload, blocks)
}

func (c *sessionCore) nextTurnContextBuffer() *nextTurnContextBuffer {
	if c.nextTurnContext == nil {
		c.nextTurnContext = newNextTurnContextBuffer()
	}
	return c.nextTurnContext
}

func injectInternalContextReminder(payload map[string]any, blocks []InternalContextBlock) map[string]any {
	reminder := renderInternalContextReminder(blocks)
	if reminder == "" {
		return payload
	}
	result := cloneMap(payload)
	message := jsonvalue.CloneMapValue(result["message"])
	if message == nil {
		message = map[string]any{"role": "user"}
	}
	message["content"] = prependInternalContextReminder(message["content"], reminder)
	if jsonvalue.StringValue(message["role"]) == "" {
		message["role"] = "user"
	}
	result["message"] = message
	return result
}

func prependInternalContextReminder(content any, reminder string) any {
	switch typed := content.(type) {
	case string:
		return joinInternalReminderAndText(reminder, typed)
	case []any:
		return append([]any{map[string]any{"type": "text", "text": reminder}}, jsonvalue.CloneAnySlicePreserveTypedSlices(typed)...)
	case []map[string]any:
		blocks := make([]map[string]any, 0, len(typed)+1)
		blocks = append(blocks, map[string]any{"type": "text", "text": reminder})
		blocks = append(blocks, jsonvalue.CloneMapSlice(typed)...)
		return blocks
	default:
		if text := jsonvalue.StringValue(content); strings.TrimSpace(text) != "" {
			return joinInternalReminderAndText(reminder, text)
		}
		return reminder
	}
}

func joinInternalReminderAndText(reminder string, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return reminder
	}
	return reminder + "\n\n" + text
}

func renderInternalContextReminder(blocks []InternalContextBlock) string {
	blocks = normalizeInternalContextBlocks(blocks)
	if len(blocks) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("<system-reminder>\n")
	builder.WriteString(internalContextReminderIntro)
	for _, block := range blocks {
		builder.WriteString("\n\n<internal_context source=\"")
		builder.WriteString(sanitizeInternalContextSource(block.Name))
		builder.WriteString("\">\n")
		builder.WriteString(block.Content)
		builder.WriteString("\n</internal_context>")
	}
	builder.WriteString("\n</system-reminder>")
	return builder.String()
}

func sanitizeInternalContextSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "runtime"
	}
	var builder strings.Builder
	for _, r := range source {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_', r == '-', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "runtime"
	}
	return result
}
