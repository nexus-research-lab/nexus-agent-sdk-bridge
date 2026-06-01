package tools

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpserver"
)

// Resource defines an MCP resource exposed by an SDK-hosted MCP server.
type Resource struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
	Text        string
	Blob        []byte
}

// SDKMCPServerOptions configures an in-process SDK MCP server.
type SDKMCPServerOptions struct {
	Name      string
	Version   string
	Tools     []Tool
	Resources []Resource
}

// SimpleSDKMCPServer is a minimal in-process MCP server with tools/resources.
type SimpleSDKMCPServer struct {
	server *mcpserver.SimpleServer
}

// CreateSDKMCPServer creates an in-process SDK MCP server.
func CreateSDKMCPServer(options SDKMCPServerOptions) *SimpleSDKMCPServer {
	version := options.Version
	if version == "" {
		version = "0.0.0"
	}
	server := mcpserver.CreateServer(options.Name, version, sdkTools(options.Tools))
	if len(options.Resources) > 0 {
		server.WithResources(sdkResources(options.Resources))
	}
	return &SimpleSDKMCPServer{server: server}
}

// HandleMessage handles one MCP JSON-RPC message.
func (s *SimpleSDKMCPServer) HandleMessage(ctx context.Context, message map[string]any) (map[string]any, error) {
	if s == nil || s.server == nil {
		return nil, errors.New("tools: sdk mcp server is nil")
	}
	return s.server.HandleMessage(ctx, message)
}

func sdkTools(definitions []Tool) []mcpserver.Tool {
	tools := make([]mcpserver.Tool, 0, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		tools = append(tools, sdkTool(definition))
	}
	return tools
}

func sdkResources(resources []Resource) []mcpserver.Resource {
	result := make([]mcpserver.Resource, 0, len(resources))
	for _, resource := range resources {
		result = append(result, mcpserver.Resource{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MIMEType:    resource.MIMEType,
			Text:        resource.Text,
			Blob:        append([]byte(nil), resource.Blob...),
		})
	}
	return result
}
