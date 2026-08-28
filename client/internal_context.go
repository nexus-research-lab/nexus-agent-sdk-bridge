package client

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const internalContextReminderIntro = "The following runtime-internal context applies to the next user message. It is not user-authored. Use it to handle that message; on later turns, treat it only as historical context. Do not mention this wrapper to the user."

type nextTurnContextBuffer struct {
	mu     sync.Mutex
	blocks []InternalContextBlock
}

func (b *nextTurnContextBuffer) set(blocks []InternalContextBlock) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocks = normalizeInternalContextBlocks(blocks)
}

func (b *nextTurnContextBuffer) bind() []InternalContextBlock {
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
	slices.SortFunc(result, compareInternalContextBlocks)
	return result
}

func compareInternalContextBlocks(left InternalContextBlock, right InternalContextBlock) int {
	if order := cmp.Compare(right.Priority, left.Priority); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Name, right.Name); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Content, right.Content); order != 0 {
		return order
	}
	return cmp.Compare(internalContextMetadataKey(left.Metadata), internalContextMetadataKey(right.Metadata))
}

func internalContextMetadataKey(metadata map[string]string) string {
	var builder strings.Builder
	for _, key := range slices.Sorted(maps.Keys(metadata)) {
		builder.WriteString(key)
		builder.WriteByte(0)
		builder.WriteString(metadata[key])
		builder.WriteByte(0)
	}
	return builder.String()
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
	if len(payload) == 0 || jsonvalue.StringValue(payload["type"]) != "user" {
		return payload
	}
	if normalizedRuntimeKind(c.options.Runtime.Kind) == RuntimeClaude {
		return payload
	}
	blocks := c.nextTurnContext.bind()
	if len(blocks) == 0 {
		return payload
	}
	return attachInternalContext(payload, renderInternalContextReminder(blocks))
}

func attachInternalContext(payload map[string]any, reminder string) map[string]any {
	if reminder == "" {
		return payload
	}
	result := cloneMap(payload)
	result[protocol.InternalContextPayloadKey] = reminder
	return result
}

func (c *sessionCore) claudeInternalContextHook(context.Context, hook.Input, string) (hook.Output, error) {
	content := renderInternalContext(c.nextTurnContext.bind())
	if content == "" {
		return hook.Output{}, nil
	}
	return hook.Output{SpecificOutput: &hook.SpecificOutput{
		HookEventName:     hook.EventUserPromptSubmit,
		AdditionalContext: content,
	}}, nil
}

func renderInternalContextReminder(blocks []InternalContextBlock) string {
	content := renderInternalContext(blocks)
	if content == "" {
		return ""
	}
	return "<system-reminder>\n" + content + "\n</system-reminder>"
}

func renderInternalContext(blocks []InternalContextBlock) string {
	blocks = normalizeInternalContextBlocks(blocks)
	if len(blocks) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(internalContextReminderIntro)
	for _, block := range blocks {
		builder.WriteString("\n\n<internal_context source=\"")
		builder.WriteString(sanitizeInternalContextSource(block.Name))
		builder.WriteString("\">\n")
		builder.WriteString(block.Content)
		builder.WriteString("\n</internal_context>")
	}
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
