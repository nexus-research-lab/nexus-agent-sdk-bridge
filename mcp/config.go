package mcp

import (
	"context"
)

// ServerConfig 表示 SDK 可接入的 MCP server 配置。
type ServerConfig interface {
	serverConfig()
}

// StdioServerConfig 表示 stdio MCP server。
type StdioServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}

func (StdioServerConfig) serverConfig() {}

// OAuthConfig 表示 HTTP/SSE MCP server 的 OAuth 元数据。
type OAuthConfig struct {
	ClientID              string
	CallbackPort          int
	AuthServerMetadataURL string
	XAA                   *bool
}

// SSEServerConfig 表示 SSE MCP server。
type SSEServerConfig struct {
	URL           string
	Headers       map[string]string
	HeadersHelper string
	OAuth         *OAuthConfig
}

func (SSEServerConfig) serverConfig() {}

// HTTPServerConfig 表示 HTTP MCP server。
type HTTPServerConfig struct {
	URL           string
	Headers       map[string]string
	HeadersHelper string
	OAuth         *OAuthConfig
}

func (HTTPServerConfig) serverConfig() {}

// SDKServerConfig 表示进程内 SDK MCP server。
type SDKServerConfig struct {
	Name     string
	Instance SDKMCPServer
}

func (SDKServerConfig) serverConfig() {}

// SDKMCPServer 表示由 SDK 进程托管的 MCP server。
type SDKMCPServer interface {
	HandleMessage(context.Context, map[string]any) (map[string]any, error)
}
