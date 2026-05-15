package tools

import "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

// Resource defines an MCP resource exposed by an SDK-hosted MCP server.
type Resource = mcp.Resource

// SDKMCPServerOptions configures an in-process SDK MCP server.
type SDKMCPServerOptions struct {
	Name      string
	Version   string
	Tools     []Tool
	Resources []Resource
}

// SimpleSDKMCPServer is a minimal in-process MCP server with tools/resources.
type SimpleSDKMCPServer = mcp.SimpleSDKMCPServer

// CreateSDKMCPServer creates an in-process SDK MCP server.
func CreateSDKMCPServer(options SDKMCPServerOptions) *SimpleSDKMCPServer {
	version := options.Version
	if version == "" {
		version = "0.0.0"
	}
	server := mcp.CreateSDKMCPServer(options.Name, version, sdkTools(options.Tools))
	if len(options.Resources) > 0 {
		server.WithResources(options.Resources)
	}
	return server
}

func sdkTools(definitions []Tool) []mcp.Tool {
	tools := make([]mcp.Tool, 0, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		tools = append(tools, sdkTool(definition))
	}
	return tools
}
