package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpserver"
)

// ServerConfig describes an MCP server that can be serialized into the SDK
// initialization payload.
type ServerConfig interface {
	buildCLIConfig(alias string) (map[string]any, error)
	sdkServer() SDKMCPServer
}

// StdioServerConfig describes a stdio MCP server.
type StdioServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}

func (c StdioServerConfig) buildCLIConfig(alias string) (map[string]any, error) {
	if strings.TrimSpace(c.Command) == "" {
		return nil, fmt.Errorf("mcp: server %q command is empty", alias)
	}
	config := map[string]any{
		"command": c.Command,
	}
	if len(c.Args) > 0 {
		config["args"] = c.Args
	}
	if len(c.Env) > 0 {
		config["env"] = c.Env
	}
	return config, nil
}

func (c StdioServerConfig) sdkServer() SDKMCPServer {
	return nil
}

// OAuthConfig describes OAuth metadata for HTTP/SSE MCP servers.
type OAuthConfig struct {
	ClientID              string
	CallbackPort          int
	AuthServerMetadataURL string
	XAA                   *bool
}

func (c OAuthConfig) buildCLIConfig(alias string) (map[string]any, error) {
	config := map[string]any{}
	if clientID := strings.TrimSpace(c.ClientID); clientID != "" {
		config["clientId"] = clientID
	}
	if c.CallbackPort < 0 {
		return nil, fmt.Errorf("mcp: server %q oauth callback port must be positive", alias)
	}
	if c.CallbackPort > 0 {
		config["callbackPort"] = c.CallbackPort
	}
	if metadataURL := strings.TrimSpace(c.AuthServerMetadataURL); metadataURL != "" {
		parsed, err := url.Parse(metadataURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("mcp: server %q oauth authServerMetadataUrl must be an https URL", alias)
		}
		config["authServerMetadataUrl"] = metadataURL
	}
	if c.XAA != nil {
		config["xaa"] = *c.XAA
	}
	return config, nil
}

// SSEServerConfig describes an SSE MCP server.
type SSEServerConfig struct {
	URL           string
	Headers       map[string]string
	HeadersHelper string
	OAuth         *OAuthConfig
}

func (c SSEServerConfig) buildCLIConfig(alias string) (map[string]any, error) {
	if strings.TrimSpace(c.URL) == "" {
		return nil, fmt.Errorf("mcp: server %q url is empty", alias)
	}
	config := map[string]any{
		"type": "sse",
		"url":  c.URL,
	}
	if len(c.Headers) > 0 {
		config["headers"] = c.Headers
	}
	if headersHelper := strings.TrimSpace(c.HeadersHelper); headersHelper != "" {
		config["headersHelper"] = headersHelper
	}
	if c.OAuth != nil {
		oauthConfig, err := c.OAuth.buildCLIConfig(alias)
		if err != nil {
			return nil, err
		}
		if len(oauthConfig) > 0 {
			config["oauth"] = oauthConfig
		}
	}
	return config, nil
}

func (c SSEServerConfig) sdkServer() SDKMCPServer {
	return nil
}

// HTTPServerConfig describes an HTTP MCP server.
type HTTPServerConfig struct {
	URL           string
	Headers       map[string]string
	HeadersHelper string
	OAuth         *OAuthConfig
}

func (c HTTPServerConfig) buildCLIConfig(alias string) (map[string]any, error) {
	if strings.TrimSpace(c.URL) == "" {
		return nil, fmt.Errorf("mcp: server %q url is empty", alias)
	}
	config := map[string]any{
		"type": "http",
		"url":  c.URL,
	}
	if len(c.Headers) > 0 {
		config["headers"] = c.Headers
	}
	if headersHelper := strings.TrimSpace(c.HeadersHelper); headersHelper != "" {
		config["headersHelper"] = headersHelper
	}
	if c.OAuth != nil {
		oauthConfig, err := c.OAuth.buildCLIConfig(alias)
		if err != nil {
			return nil, err
		}
		if len(oauthConfig) > 0 {
			config["oauth"] = oauthConfig
		}
	}
	return config, nil
}

func (c HTTPServerConfig) sdkServer() SDKMCPServer {
	return nil
}

// WebSocketServerConfig describes a WebSocket MCP server.
type WebSocketServerConfig struct {
	URL           string
	Headers       map[string]string
	HeadersHelper string
}

func (c WebSocketServerConfig) buildCLIConfig(alias string) (map[string]any, error) {
	if strings.TrimSpace(c.URL) == "" {
		return nil, fmt.Errorf("mcp: server %q url is empty", alias)
	}
	config := map[string]any{
		"type": "ws",
		"url":  c.URL,
	}
	if len(c.Headers) > 0 {
		config["headers"] = c.Headers
	}
	if headersHelper := strings.TrimSpace(c.HeadersHelper); headersHelper != "" {
		config["headersHelper"] = headersHelper
	}
	return config, nil
}

func (c WebSocketServerConfig) sdkServer() SDKMCPServer {
	return nil
}

// SDKServerConfig describes an in-process SDK MCP server.
type SDKServerConfig struct {
	Name     string
	Instance SDKMCPServer
}

func (c SDKServerConfig) buildCLIConfig(alias string) (map[string]any, error) {
	if c.Instance == nil {
		return nil, fmt.Errorf("mcp: sdk server %q instance is nil", alias)
	}
	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = alias
	}
	return map[string]any{
		"type": "sdk",
		"name": name,
	}, nil
}

func (c SDKServerConfig) sdkServer() SDKMCPServer {
	return c.Instance
}

// ResolveServers merges explicit MCP server configs with separately supplied
// SDK MCP servers.
func ResolveServers(servers map[string]ServerConfig, sdkServers map[string]SDKMCPServer) map[string]ServerConfig {
	result := map[string]ServerConfig{}
	for name, config := range servers {
		result[name] = config
	}
	for name, server := range sdkServers {
		if server == nil {
			continue
		}
		if _, exists := result[name]; exists {
			continue
		}
		result[name] = SDKServerConfig{
			Name:     name,
			Instance: server,
		}
	}
	return result
}

// SDKServerRegistry extracts in-process SDK MCP servers from both explicit
// SDK server maps and normal MCP server configs.
func SDKServerRegistry(servers map[string]ServerConfig, sdkServers map[string]SDKMCPServer) map[string]SDKMCPServer {
	result := map[string]SDKMCPServer{}
	for name, server := range sdkServers {
		if server != nil {
			result[name] = server
		}
	}
	for name, config := range servers {
		if config == nil {
			continue
		}
		server := config.sdkServer()
		if server == nil {
			continue
		}
		result[name] = server
	}
	return result
}

// SerializeServers serializes MCP server configs for the SDK initialization
// payload and returns the in-process SDK MCP servers that must be hosted by the
// SDK process.
func SerializeServers(servers map[string]ServerConfig) (map[string]any, map[string]SDKMCPServer, error) {
	serialized := map[string]any{}
	sdkServers := map[string]SDKMCPServer{}

	for name, config := range servers {
		if config == nil {
			continue
		}

		cliConfig, err := config.buildCLIConfig(name)
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(stringValue(cliConfig["scope"])) == "" {
			cliConfig["scope"] = "dynamic"
		}
		serialized[name] = cliConfig

		if sdkServer := config.sdkServer(); sdkServer != nil {
			sdkServers[name] = sdkServer
		}
	}

	return serialized, sdkServers, nil
}

// CloneServerConfigs returns a shallow copy of a server config map.
func CloneServerConfigs(input map[string]ServerConfig) map[string]ServerConfig {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]ServerConfig, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

// CloneSDKServers returns a shallow copy of an SDK MCP server map.
func CloneSDKServers(input map[string]SDKMCPServer) map[string]SDKMCPServer {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]SDKMCPServer, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

// SDKMCPServer is an in-process MCP server hosted by the SDK.
type SDKMCPServer = mcpserver.Server

// ToolHandler handles one MCP tool call.
type ToolHandler = mcpserver.ToolHandler

// ToolAnnotations captures MCP tool annotations.
type ToolAnnotations = mcpserver.ToolAnnotations

// Tool describes a tool exposed by a SimpleSDKMCPServer.
type Tool = mcpserver.Tool

// ToolResult is the result of an MCP tool call.
type ToolResult = mcpserver.ToolResult

// Resource describes an MCP resource hosted by an SDK MCP server.
type Resource = mcpserver.Resource

// SimpleSDKMCPServer is the SDK's minimal in-process MCP server.
type SimpleSDKMCPServer = mcpserver.SimpleServer

// NewSimpleSDKMCPServer creates an in-process MCP server.
func NewSimpleSDKMCPServer(name string, version string, tools []Tool) *SimpleSDKMCPServer {
	return mcpserver.NewSimpleServer(name, version, tools)
}

// CreateSDKMCPServer creates an in-process SDK MCP server.
func CreateSDKMCPServer(name string, version string, tools []Tool) *SimpleSDKMCPServer {
	return mcpserver.CreateServer(name, version, tools)
}

// CreateSDKMCPServerWithResources creates an SDK MCP server with resources.
func CreateSDKMCPServerWithResources(name string, version string, tools []Tool, resources []Resource) *SimpleSDKMCPServer {
	return mcpserver.CreateServerWithResources(name, version, tools, resources)
}

// NewTypedMCPTool creates a typed MCP tool with inferred JSON Schema.
func NewTypedMCPTool[T any](
	name string,
	description string,
	handler func(context.Context, T) (ToolResult, error),
	annotations *ToolAnnotations,
) (Tool, error) {
	return mcpserver.NewTypedTool(name, description, handler, annotations)
}

// JSONSchemaFor infers a JSON Schema from a Go type.
func JSONSchemaFor[T any]() (map[string]any, error) {
	return mcpserver.JSONSchemaFor[T]()
}
