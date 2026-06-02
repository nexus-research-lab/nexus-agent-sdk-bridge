package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpserver"
)

// Context provides SDK runtime context for one custom tool call.
type Context struct {
	ToolUseID string
	SessionID string
	RoundID   string
	Source    string
	Metadata  map[string]string
}

// Tool defines a Go-native SDK custom tool.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Call(context.Context, map[string]any, *Context) (Result, error)
	IsReadOnly(map[string]any) bool
	IsConcurrencySafe(map[string]any) bool
}

// Handler handles one SDK-hosted custom tool call.
type Handler func(context.Context, map[string]any, *Context) (Result, error)

// TypedHandler handles one SDK-hosted custom tool call with typed input.
type TypedHandler[T any] func(context.Context, T, *Context) (Result, error)

// Result is the MCP tool result returned by an SDK-hosted custom tool.
type Result struct {
	Content           []map[string]any
	StructuredContent map[string]any
	Meta              map[string]any
	IsError           bool
}

// Metadata captures optional MCP and Anthropic metadata for a custom tool.
type Metadata struct {
	SearchHint  string
	AlwaysLoad  bool
	Annotations *Annotations
}

// MetadataProvider can be implemented by custom Tool values to provide
// optional MCP/Anthropic metadata.
type MetadataProvider interface {
	ToolMetadata() Metadata
}

// Annotations captures MCP tool annotations plus Anthropic metadata.
type Annotations struct {
	ReadOnlyHint       bool
	DestructiveHint    bool
	IdempotentHint     bool
	OpenWorldHint      bool
	ReadOnly           bool
	Destructive        bool
	OpenWorld          bool
	MaxResultSizeChars int
}

// Definition describes one SDK-hosted custom tool.
type Definition struct {
	name            string
	description     string
	inputSchema     map[string]any
	searchHint      string
	alwaysLoad      bool
	annotations     *Annotations
	handler         Handler
	readOnly        bool
	concurrencySafe bool
}

// ToolOption configures a custom tool definition.
type ToolOption func(*Definition)

// WithAnnotations sets MCP annotations for a custom tool.
func WithAnnotations(annotations Annotations) ToolOption {
	return func(definition *Definition) {
		copy := annotations
		definition.annotations = &copy
	}
}

// WithSearchHint sets the Anthropic search hint metadata for a custom tool.
func WithSearchHint(searchHint string) ToolOption {
	return func(definition *Definition) {
		definition.searchHint = searchHint
	}
}

// WithAlwaysLoad marks a custom tool as always loaded instead of deferred.
func WithAlwaysLoad(alwaysLoad bool) ToolOption {
	return func(definition *Definition) {
		definition.alwaysLoad = alwaysLoad
	}
}

// ReadOnly marks a custom tool as read-only.
func ReadOnly() ToolOption {
	return func(definition *Definition) {
		definition.readOnly = true
		if definition.annotations == nil {
			definition.annotations = &Annotations{}
		}
		definition.annotations.ReadOnly = true
		definition.annotations.ReadOnlyHint = true
	}
}

// Concurrent marks a custom tool as safe to run concurrently.
func Concurrent() ToolOption {
	return func(definition *Definition) {
		definition.concurrencySafe = true
	}
}

// New creates an SDK custom tool from an explicit JSON Schema.
func New(
	name string,
	description string,
	inputSchema map[string]any,
	handler Handler,
	options ...ToolOption,
) Definition {
	definition := Definition{
		name:        name,
		description: description,
		inputSchema: inputSchema,
		handler:     handler,
	}
	for _, option := range options {
		if option != nil {
			option(&definition)
		}
	}
	return definition
}

// NewTyped creates an SDK custom tool with a JSON Schema inferred from T.
func NewTyped[T any](
	name string,
	description string,
	handler TypedHandler[T],
	options ...ToolOption,
) (Definition, error) {
	schema, err := JSONSchemaFor[T]()
	if err != nil {
		return Definition{}, err
	}
	return New(name, description, schema, func(ctx context.Context, input map[string]any, toolCtx *Context) (Result, error) {
		var decoded T
		payload, err := json.Marshal(input)
		if err != nil {
			return Result{}, fmt.Errorf("tools: marshal typed tool input failed: %w", err)
		}
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return Result{}, fmt.Errorf("tools: decode typed tool input failed: %w", err)
		}
		return handler(ctx, decoded, toolCtx)
	}, options...), nil
}

// NewTypedTool creates an SDK custom tool with a JSON Schema inferred from T.
//
// Deprecated: use NewTyped for new custom SDK tools.
func NewTypedTool[T any](
	name string,
	description string,
	handler func(context.Context, T) (Result, error),
	options ...ToolOption,
) (Definition, error) {
	return NewTyped(name, description, func(ctx context.Context, input T, _ *Context) (Result, error) {
		return handler(ctx, input)
	}, options...)
}

// JSONSchemaFor infers a JSON Schema from a Go type.
func JSONSchemaFor[T any]() (map[string]any, error) {
	return mcpserver.JSONSchemaFor[T]()
}

// Name returns the tool's unique name.
func (d Definition) Name() string {
	return d.name
}

// Description returns the model-visible tool description.
func (d Definition) Description() string {
	return d.description
}

// InputSchema returns the JSON Schema for this tool's input.
func (d Definition) InputSchema() map[string]any {
	return d.inputSchema
}

// Call executes the tool.
func (d Definition) Call(ctx context.Context, input map[string]any, toolCtx *Context) (Result, error) {
	if d.handler == nil {
		return Result{}, fmt.Errorf("tools: tool %q handler is nil", d.name)
	}
	return d.handler(ctx, input, toolCtx)
}

// IsReadOnly returns whether the tool only reads state.
func (d Definition) IsReadOnly(map[string]any) bool {
	if d.readOnly {
		return true
	}
	return d.annotations != nil && (d.annotations.ReadOnly || d.annotations.ReadOnlyHint)
}

// IsConcurrencySafe returns whether this tool can run concurrently.
func (d Definition) IsConcurrencySafe(map[string]any) bool {
	return d.concurrencySafe
}

// ToolMetadata returns optional MCP/Anthropic metadata for this definition.
func (d Definition) ToolMetadata() Metadata {
	return Metadata{
		SearchHint:  d.searchHint,
		AlwaysLoad:  d.alwaysLoad,
		Annotations: cloneAnnotations(d.annotations),
	}
}

func sdkTool(tool Tool) mcpserver.Tool {
	metadata := metadataForTool(tool)
	return mcpserver.Tool{
		Name:        tool.Name(),
		Description: tool.Description(),
		InputSchema: tool.InputSchema(),
		SearchHint:  metadata.SearchHint,
		AlwaysLoad:  metadata.AlwaysLoad,
		Annotations: sdkAnnotations(metadata.Annotations),
		Handler: func(ctx context.Context, input map[string]any) (mcpserver.ToolResult, error) {
			result, err := tool.Call(ctx, input, &Context{})
			if err != nil {
				return mcpserver.ToolResult{}, err
			}
			return mcpserver.ToolResult{
				Content:           result.Content,
				StructuredContent: result.StructuredContent,
				Meta:              result.Meta,
				IsError:           result.IsError,
			}, nil
		},
	}
}

func metadataForTool(tool Tool) Metadata {
	var metadata Metadata
	if provider, ok := tool.(MetadataProvider); ok {
		metadata = provider.ToolMetadata()
	}
	if metadata.Annotations == nil {
		metadata.Annotations = &Annotations{}
	}
	if tool.IsReadOnly(nil) {
		metadata.Annotations.ReadOnly = true
		metadata.Annotations.ReadOnlyHint = true
	}
	return metadata
}

func cloneAnnotations(annotations *Annotations) *Annotations {
	if annotations == nil {
		return nil
	}
	copy := *annotations
	return &copy
}

func sdkAnnotations(annotations *Annotations) *mcpserver.ToolAnnotations {
	if annotations == nil {
		return nil
	}
	return &mcpserver.ToolAnnotations{
		ReadOnlyHint:       annotations.ReadOnlyHint,
		DestructiveHint:    annotations.DestructiveHint,
		IdempotentHint:     annotations.IdempotentHint,
		OpenWorldHint:      annotations.OpenWorldHint,
		ReadOnly:           annotations.ReadOnly,
		Destructive:        annotations.Destructive,
		OpenWorld:          annotations.OpenWorld,
		MaxResultSizeChars: annotations.MaxResultSizeChars,
	}
}

// Text returns a text-only successful tool result.
func Text(text string) Result {
	return Result{
		Content: []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

// Error returns a text-only error tool result.
func Error(text string) Result {
	result := Text(text)
	result.IsError = true
	return result
}
