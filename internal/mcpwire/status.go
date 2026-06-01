package mcpwire

import (
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

// DecodeStatusResponse 解析 MCP 状态控制响应。
func DecodeStatusResponse(payload map[string]any) mcp.StatusResponse {
	items := jsonvalue.SliceValue(payload["mcp_servers"])
	servers := make([]mcp.ServerStatus, 0, len(items))
	for _, item := range items {
		server := jsonvalue.MapValue(item)
		if len(server) == 0 {
			continue
		}

		servers = append(servers, mcp.ServerStatus{
			Name:       jsonvalue.StringValue(server["name"]),
			Status:     jsonvalue.StringValue(server["status"]),
			ServerInfo: decodeServerInfo(server["server_info"]),
			Error:      jsonvalue.StringValue(server["error"]),
			Config:     jsonvalue.MapValue(server["config"]),
			Scope:      jsonvalue.StringValue(server["scope"]),
			Tools:      decodeTools(server["tools"]),
			Raw:        server,
		})
	}

	return mcp.StatusResponse{
		MCPServers: servers,
		Raw:        payload,
	}
}

// DecodeSetServersResult 解析动态 MCP server 更新结果。
func DecodeSetServersResult(payload map[string]any) mcp.SetServersResult {
	errorsResult := map[string]string{}
	for key, value := range jsonvalue.MapValue(payload["errors"]) {
		text := jsonvalue.StringValue(value)
		if text == "" {
			continue
		}
		errorsResult[key] = text
	}

	return mcp.SetServersResult{
		Added:   jsonvalue.StringSliceValue(payload["added"]),
		Removed: jsonvalue.StringSliceValue(payload["removed"]),
		Errors:  errorsResult,
		Raw:     payload,
	}
}

func decodeServerInfo(raw any) *mcp.ServerInfo {
	info := jsonvalue.MapValue(raw)
	if len(info) == 0 {
		return nil
	}
	return &mcp.ServerInfo{
		Name:    jsonvalue.StringValue(info["name"]),
		Version: jsonvalue.StringValue(info["version"]),
	}
}

func decodeTools(raw any) []mcp.ToolInfo {
	items := jsonvalue.SliceValue(raw)
	tools := make([]mcp.ToolInfo, 0, len(items))
	for _, item := range items {
		tool := jsonvalue.MapValue(item)
		if len(tool) == 0 {
			continue
		}
		tools = append(tools, mcp.ToolInfo{
			Name:        jsonvalue.StringValue(tool["name"]),
			Description: jsonvalue.StringValue(tool["description"]),
			InputSchema: jsonvalue.MapValue(tool["input_schema"]),
			Meta:        jsonvalue.MapValue(tool["_meta"]),
			Annotations: decodeToolAnnotations(tool["annotations"]),
		})
	}
	return tools
}

func decodeToolAnnotations(raw any) *mcp.ToolStatusAnnotations {
	data := jsonvalue.MapValue(raw)
	if len(data) == 0 {
		return nil
	}
	readOnlyHint := jsonvalue.BoolValue(data["read_only_hint"])
	readOnly := jsonvalue.BoolValue(data["read_only"])
	destructiveHint := jsonvalue.BoolValue(data["destructive_hint"])
	destructive := jsonvalue.BoolValue(data["destructive"])
	openWorldHint := jsonvalue.BoolValue(data["open_world_hint"])
	openWorld := jsonvalue.BoolValue(data["open_world"])

	return &mcp.ToolStatusAnnotations{
		ReadOnlyHint:    readOnlyHint || readOnly,
		DestructiveHint: destructiveHint || destructive,
		IdempotentHint:  jsonvalue.BoolValue(data["idempotent_hint"]),
		OpenWorldHint:   openWorldHint || openWorld,
		ReadOnly:        readOnly,
		Destructive:     destructive,
		OpenWorld:       openWorld,
	}
}
