package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

// Server is an in-process MCP server hosted by the SDK.
type Server interface {
	HandleMessage(context.Context, map[string]any) (map[string]any, error)
}

// ToolHandler handles one MCP tool call.
type ToolHandler func(context.Context, map[string]any) (ToolResult, error)

// ToolAnnotations captures MCP tool annotations plus Anthropic metadata.
type ToolAnnotations struct {
	ReadOnlyHint       bool
	DestructiveHint    bool
	IdempotentHint     bool
	OpenWorldHint      bool
	ReadOnly           bool
	Destructive        bool
	OpenWorld          bool
	MaxResultSizeChars int
}

// Tool defines an MCP tool exposed by a SimpleServer.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	SearchHint  string
	AlwaysLoad  bool
	Annotations *ToolAnnotations
	Handler     ToolHandler
}

// ToolResult is the result of an MCP tool call.
type ToolResult struct {
	Content           []map[string]any
	StructuredContent map[string]any
	Meta              map[string]any
	IsError           bool
}

// Resource defines an MCP resource exposed by a SimpleServer.
type Resource struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
	Text        string
	Blob        []byte
}

// SimpleServer is a minimal in-process MCP server with tools and resources.
type SimpleServer struct {
	name          string
	resourceOrder []string
	resources     map[string]Resource
	version       string
	toolOrder     []string
	tools         map[string]Tool
}

// NewSimpleServer creates an in-process MCP server.
func NewSimpleServer(name string, version string, tools []Tool) *SimpleServer {
	if version == "" {
		version = "1.0.0"
	}

	toolMap := make(map[string]Tool, len(tools))
	toolOrder := make([]string, 0, len(tools))
	for _, tool := range tools {
		if _, exists := toolMap[tool.Name]; !exists {
			toolOrder = append(toolOrder, tool.Name)
		}
		toolMap[tool.Name] = tool
	}

	return &SimpleServer{
		name:          name,
		resourceOrder: []string{},
		resources:     map[string]Resource{},
		version:       version,
		toolOrder:     toolOrder,
		tools:         toolMap,
	}
}

// CreateServer creates an in-process SDK MCP server.
func CreateServer(name string, version string, tools []Tool) *SimpleServer {
	return NewSimpleServer(name, version, tools)
}

// WithResources attaches resources to the current SDK MCP server.
func (s *SimpleServer) WithResources(resources []Resource) *SimpleServer {
	resourceMap := make(map[string]Resource, len(resources))
	resourceOrder := make([]string, 0, len(resources))
	for _, resource := range resources {
		if resource.URI == "" {
			continue
		}
		if _, exists := resourceMap[resource.URI]; !exists {
			resourceOrder = append(resourceOrder, resource.URI)
		}
		resourceMap[resource.URI] = resource
	}
	s.resourceOrder = resourceOrder
	s.resources = resourceMap
	return s
}

// CreateServerWithResources creates an SDK MCP server with tools and resources.
func CreateServerWithResources(name string, version string, tools []Tool, resources []Resource) *SimpleServer {
	return NewSimpleServer(name, version, tools).WithResources(resources)
}

// HandleMessage handles one MCP JSON-RPC message.
func (s *SimpleServer) HandleMessage(ctx context.Context, message map[string]any) (map[string]any, error) {
	method := jsonvalue.StringValue(message["method"])
	messageID := message["id"]

	switch method {
	case "initialize":
		capabilities := map[string]any{
			"tools": map[string]any{},
		}
		if len(s.resources) > 0 {
			capabilities["resources"] = map[string]any{}
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    capabilities,
				"serverInfo": map[string]any{
					"name":    s.name,
					"version": s.version,
				},
			},
		}, nil
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.tools))
		for _, name := range s.toolOrder {
			tool, ok := s.tools[name]
			if !ok {
				continue
			}
			schema := tool.InputSchema
			if schema == nil {
				schema = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
			tools = append(tools, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": schema,
			})
			toolPayload := tools[len(tools)-1]
			if annotations := toolAnnotationsPayload(tool.Annotations); len(annotations) > 0 {
				toolPayload["annotations"] = annotations
			}
			if meta := toolMetaPayload(tool); len(meta) > 0 {
				toolPayload["_meta"] = meta
			}
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"result": map[string]any{
				"tools": tools,
			},
		}, nil
	case "tools/call":
		params := mapValue(message["params"])
		name := jsonvalue.StringValue(params["name"])
		tool, ok := s.tools[name]
		if !ok {
			return jsonRPCError(messageID, -32601, fmt.Sprintf("tool %q not found", name)), nil
		}

		result, err := tool.Handler(ctx, mapValue(params["arguments"]))
		if err != nil {
			return jsonRPCError(messageID, -32603, err.Error()), nil
		}

		response := map[string]any{
			"content": result.Content,
		}
		if len(result.StructuredContent) > 0 {
			response["structuredContent"] = result.StructuredContent
		}
		if len(result.Meta) > 0 {
			response["_meta"] = result.Meta
		}
		if result.IsError {
			response["isError"] = true
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"result":  response,
		}, nil
	case "notifications/initialized":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"result":  map[string]any{},
		}, nil
	case "resources/list":
		resources := make([]map[string]any, 0, len(s.resources))
		for _, uri := range s.resourceOrder {
			resource, ok := s.resources[uri]
			if !ok {
				continue
			}
			resources = append(resources, map[string]any{
				"uri":         resource.URI,
				"name":        resource.Name,
				"mimeType":    resource.MIMEType,
				"description": resource.Description,
			})
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"result": map[string]any{
				"resources": resources,
			},
		}, nil
	case "resources/read":
		params := mapValue(message["params"])
		uri := jsonvalue.StringValue(params["uri"])
		resource, ok := s.resources[uri]
		if !ok {
			return jsonRPCError(messageID, -32602, fmt.Sprintf("resource %q not found", uri)), nil
		}
		entry := map[string]any{
			"uri":      resource.URI,
			"mimeType": resource.MIMEType,
		}
		if len(resource.Blob) > 0 {
			entry["blob"] = base64.StdEncoding.EncodeToString(resource.Blob)
		} else {
			entry["text"] = resource.Text
		}
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      messageID,
			"result": map[string]any{
				"contents": []map[string]any{entry},
			},
		}, nil
	default:
		return jsonRPCError(messageID, -32601, fmt.Sprintf("method %q not found", method)), nil
	}
}

func toolAnnotationsPayload(annotations *ToolAnnotations) map[string]any {
	if annotations == nil {
		return nil
	}
	payload := map[string]any{}
	if annotations.ReadOnlyHint || annotations.ReadOnly {
		payload["readOnlyHint"] = true
	}
	if annotations.DestructiveHint || annotations.Destructive {
		payload["destructiveHint"] = true
	}
	if annotations.IdempotentHint {
		payload["idempotentHint"] = true
	}
	if annotations.OpenWorldHint || annotations.OpenWorld {
		payload["openWorldHint"] = true
	}
	return payload
}

func toolMetaPayload(tool Tool) map[string]any {
	payload := map[string]any{}
	if tool.SearchHint != "" {
		payload["anthropic/searchHint"] = tool.SearchHint
	}
	if tool.AlwaysLoad {
		payload["anthropic/alwaysLoad"] = true
	}
	if tool.Annotations != nil && tool.Annotations.MaxResultSizeChars > 0 {
		payload["anthropic/maxResultSizeChars"] = tool.Annotations.MaxResultSizeChars
	}
	return payload
}

func jsonRPCError(messageID any, code int, message string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      messageID,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
}

func mapValue(raw any) map[string]any {
	if value, ok := jsonvalue.AnyMap(raw); ok {
		return value
	}
	return nil
}
