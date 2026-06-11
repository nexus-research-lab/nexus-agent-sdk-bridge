package mcpwire

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

// ResolveServers 合并显式 MCP server 配置和旧式 SDK server map。
func ResolveServers(servers map[string]mcp.ServerConfig, sdkServers map[string]mcp.SDKMCPServer) map[string]mcp.ServerConfig {
	result := map[string]mcp.ServerConfig{}
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
		result[name] = mcp.SDKServerConfig{
			Name:     name,
			Instance: server,
		}
	}
	return result
}

// SDKServerRegistry 提取需要由 SDK 进程托管的 MCP server。
func SDKServerRegistry(servers map[string]mcp.ServerConfig, sdkServers map[string]mcp.SDKMCPServer) map[string]mcp.SDKMCPServer {
	result := map[string]mcp.SDKMCPServer{}
	for name, server := range sdkServers {
		if server != nil {
			result[name] = server
		}
	}
	for name, config := range servers {
		sdkConfig, ok := config.(mcp.SDKServerConfig)
		if !ok || sdkConfig.Instance == nil {
			continue
		}
		result[name] = sdkConfig.Instance
	}
	return result
}

// SerializeServers 将公开 MCP 配置编码为 bridge wire payload。
func SerializeServers(servers map[string]mcp.ServerConfig) (map[string]any, map[string]mcp.SDKMCPServer, error) {
	serialized := map[string]any{}
	sdkServers := map[string]mcp.SDKMCPServer{}

	for name, config := range servers {
		if config == nil {
			continue
		}

		payload, server, err := serializeServer(name, config)
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(jsonvalue.StringValue(payload["scope"])) == "" {
			payload["scope"] = "dynamic"
		}
		serialized[name] = payload
		if server != nil {
			sdkServers[name] = server
		}
	}

	return serialized, sdkServers, nil
}

// MarshalConfig 将公开 MCP 配置编码为 Claude/nxs CLI 可读取的 mcpServers JSON。
func MarshalConfig(servers map[string]mcp.ServerConfig) ([]byte, map[string]mcp.SDKMCPServer, error) {
	serialized, sdkServers, err := SerializeServers(servers)
	if err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(map[string]any{
		"mcpServers": serialized,
	})
	if err != nil {
		return nil, nil, err
	}
	return data, sdkServers, nil
}

// CloneServerConfigs 返回 MCP server 配置 map 的浅拷贝。
func CloneServerConfigs(input map[string]mcp.ServerConfig) map[string]mcp.ServerConfig {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]mcp.ServerConfig, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

// CloneSDKServers 返回 SDK MCP server map 的浅拷贝。
func CloneSDKServers(input map[string]mcp.SDKMCPServer) map[string]mcp.SDKMCPServer {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]mcp.SDKMCPServer, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func serializeServer(alias string, config mcp.ServerConfig) (map[string]any, mcp.SDKMCPServer, error) {
	switch typed := config.(type) {
	case mcp.StdioServerConfig:
		return serializeStdioServer(alias, typed)
	case mcp.SSEServerConfig:
		return serializeHTTPSSEServer(alias, "sse", typed.URL, typed.Headers, typed.HeadersHelper, typed.OAuth)
	case mcp.HTTPServerConfig:
		return serializeHTTPSSEServer(alias, "http", typed.URL, typed.Headers, typed.HeadersHelper, typed.OAuth)
	case mcp.SDKServerConfig:
		return serializeSDKServer(alias, typed)
	default:
		return nil, nil, fmt.Errorf("mcp: server %q has unsupported config type %T", alias, config)
	}
}

func serializeStdioServer(alias string, config mcp.StdioServerConfig) (map[string]any, mcp.SDKMCPServer, error) {
	if strings.TrimSpace(config.Command) == "" {
		return nil, nil, fmt.Errorf("mcp: server %q command is empty", alias)
	}
	payload := map[string]any{
		"command": config.Command,
	}
	if len(config.Args) > 0 {
		payload["args"] = config.Args
	}
	if len(config.Env) > 0 {
		payload["env"] = config.Env
	}
	return payload, nil, nil
}

func serializeHTTPSSEServer(
	alias string,
	serverType string,
	serverURL string,
	headers map[string]string,
	headersHelper string,
	oauth *mcp.OAuthConfig,
) (map[string]any, mcp.SDKMCPServer, error) {
	if strings.TrimSpace(serverURL) == "" {
		return nil, nil, fmt.Errorf("mcp: server %q url is empty", alias)
	}
	payload := map[string]any{
		"type": serverType,
		"url":  serverURL,
	}
	if len(headers) > 0 {
		payload["headers"] = headers
	}
	if helper := strings.TrimSpace(headersHelper); helper != "" {
		payload["headersHelper"] = helper
	}
	if oauth != nil {
		oauthConfig, err := serializeOAuthConfig(alias, *oauth)
		if err != nil {
			return nil, nil, err
		}
		if len(oauthConfig) > 0 {
			payload["oauth"] = oauthConfig
		}
	}
	return payload, nil, nil
}

func serializeSDKServer(alias string, config mcp.SDKServerConfig) (map[string]any, mcp.SDKMCPServer, error) {
	if config.Instance == nil {
		return nil, nil, fmt.Errorf("mcp: sdk server %q instance is nil", alias)
	}
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = alias
	}
	return map[string]any{
		"type": "sdk",
		"name": name,
	}, config.Instance, nil
}

func serializeOAuthConfig(alias string, config mcp.OAuthConfig) (map[string]any, error) {
	payload := map[string]any{}
	if clientID := strings.TrimSpace(config.ClientID); clientID != "" {
		payload["clientId"] = clientID
	}
	if config.CallbackPort < 0 {
		return nil, fmt.Errorf("mcp: server %q oauth callback port must be positive", alias)
	}
	if config.CallbackPort > 0 {
		payload["callbackPort"] = config.CallbackPort
	}
	if metadataURL := strings.TrimSpace(config.AuthServerMetadataURL); metadataURL != "" {
		parsed, err := url.Parse(metadataURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("mcp: server %q oauth authServerMetadataUrl must be an https URL", alias)
		}
		payload["authServerMetadataUrl"] = metadataURL
	}
	if config.XAA != nil {
		payload["xaa"] = *config.XAA
	}
	return payload, nil
}
