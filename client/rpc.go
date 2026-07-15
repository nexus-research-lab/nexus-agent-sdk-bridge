package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpwire"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const defaultInterruptControlTimeout = 5 * time.Second

func (c *sessionCore) interrupt(ctx context.Context) error {
	return c.interruptWithReason(ctx, "")
}

func (c *sessionCore) interruptWithReason(ctx context.Context, reason string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	if c.transport == nil {
		return ErrNotConnected
	}
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return c.sendInterruptControlRequest(ctx, reason)
	}
	if err := c.transport.Interrupt(); err != nil {
		if !errors.Is(err, transport.ErrInterruptUnsupported) {
			return err
		}
		return c.sendInterruptControlRequest(ctx, "")
	}
	return nil
}

func (c *sessionCore) sendInterruptControlRequest(ctx context.Context, reason string) error {
	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "interrupt",
			Reason:  strings.TrimSpace(reason),
		},
		interruptControlTimeout(c.options.Runtime.InitializeTimeout),
	)
	return err
}

func interruptControlTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > defaultInterruptControlTimeout {
		return defaultInterruptControlTimeout
	}
	return timeout
}

// setPermissionMode 修改权限模式。
func (c *sessionCore) setPermissionMode(ctx context.Context, mode permission.Mode) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	normalizedMode := normalizePermissionMode(mode)
	if normalizedMode == permission.ModeBypassPermissions && !c.options.allowsDangerouslySkipPermissions() {
		return ErrBypassPermissionsNotAllowed
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "set_permission_mode",
			Mode:    normalizedMode,
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return err
	}
	c.options.Runtime.PermissionMode = normalizedMode
	if normalizedMode == permission.ModeBypassPermissions {
		c.options.Runtime.AllowDangerouslySkipPermissions = true
	}
	return nil
}

// setModel 修改后续会话轮次的模型。
func (c *sessionCore) setModel(ctx context.Context, model string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "set_model",
			Model:   model,
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err == nil {
		c.options.Model = model
	}
	return err
}

// setMaxThinkingTokens 修改扩展思考预算。
func (c *sessionCore) setMaxThinkingTokens(ctx context.Context, maxThinkingTokens int) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:           "set_max_thinking_tokens",
			MaxThinkingTokens: pointerTo(maxThinkingTokens),
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err == nil {
		c.options.Runtime.MaxThinkingTokens = maxThinkingTokens
	}
	return err
}

func (c *sessionCore) updateEnvironment(ctx context.Context, environment map[string]string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	if normalizedRuntimeKind(c.options.Runtime.Kind) != RuntimeNXS {
		return ErrUnsupportedCapability
	}
	if len(environment) == 0 {
		return nil
	}
	_, err := c.sendControlRequest(ctx, protocol.ControlRequest{
		Subtype:   "update_environment_variables",
		Variables: cloneStringMap(environment),
	}, c.options.Runtime.InitializeTimeout)
	if err != nil {
		return err
	}
	if c.options.Env == nil {
		c.options.Env = map[string]string{}
	}
	for key, value := range environment {
		c.options.Env[key] = value
	}
	return nil
}

// getContextUsage 获取当前上下文使用情况。
func (c *sessionCore) getContextUsage(ctx context.Context) (ContextUsageResponse, error) {
	if !c.isConnected() {
		return ContextUsageResponse{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "get_context_usage",
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return ContextUsageResponse{}, err
	}
	return decodeContextUsageResponse(response), nil
}

func normalizePermissionMode(mode permission.Mode) permission.Mode {
	if mode == "" {
		return permission.ModeDefault
	}
	return mode
}

// rewindFiles 将文件回滚到指定用户消息时刻。
func (c *sessionCore) rewindFiles(ctx context.Context, userMessageID string, dryRun bool) (RewindFilesResult, error) {
	if !c.isConnected() {
		return RewindFilesResult{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:       "rewind_files",
			UserMessageID: userMessageID,
			DryRun:        pointerTo(dryRun),
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return RewindFilesResult{}, err
	}
	return decodeRewindFilesResult(response), nil
}

func (c *sessionCore) removeMessages(ctx context.Context, uuids []string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	normalizedUUIDs := normalizeMessageUUIDs(uuids)
	if len(normalizedUUIDs) == 0 {
		return errors.New("client: message uuid is required")
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:      "remove_messages",
			MessageUUIDs: normalizedUUIDs,
		},
		c.options.Runtime.InitializeTimeout,
	)
	return err
}

func normalizeMessageUUIDs(uuids []string) []string {
	if len(uuids) == 0 {
		return nil
	}
	result := make([]string, 0, len(uuids))
	seen := make(map[string]struct{}, len(uuids))
	for _, uuid := range uuids {
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			continue
		}
		if _, exists := seen[uuid]; exists {
			continue
		}
		seen[uuid] = struct{}{}
		result = append(result, uuid)
	}
	return result
}

func (c *sessionCore) stopTask(ctx context.Context, taskID string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("client: task id is required")
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "stop_task",
			TaskID:  taskID,
		},
		c.options.Runtime.InitializeTimeout,
	)
	return err
}

func (c *sessionCore) sendTaskMessage(ctx context.Context, taskID string, message string, summary string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("client: task id is required")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("client: message is required")
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "send_task_message",
			TaskID:  taskID,
			Payload: map[string]any{
				"message": message,
				"summary": strings.TrimSpace(summary),
			},
		},
		c.options.Runtime.InitializeTimeout,
	)
	return err
}

// tryAutoDream 请求 nxs 检查并在满足条件时执行一次记忆巩固。
func (c *sessionCore) tryAutoDream(ctx context.Context) (AutoDreamResult, error) {
	if !c.isConnected() {
		return AutoDreamResult{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{Subtype: "run_auto_dream"},
		0,
	)
	if err != nil {
		return AutoDreamResult{}, err
	}
	return decodeAutoDreamResult(response), nil
}

func (c *sessionCore) applyFlagSettings(ctx context.Context, settings map[string]any) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:  "apply_flag_settings",
			Settings: jsonvalue.CloneMapPreserveTypedSlices(settings),
		},
		c.options.Runtime.InitializeTimeout,
	)
	return err
}

func (c *sessionCore) getSettings(ctx context.Context) (SettingsResponse, error) {
	if !c.isConnected() {
		return SettingsResponse{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "get_settings",
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return SettingsResponse{}, err
	}
	return decodeSettingsResponse(response), nil
}

func (c *sessionCore) initializationResult(ctx context.Context) (InitializationResult, error) {
	_ = ctx
	if !c.isConnected() {
		return InitializationResult{}, ErrNotConnected
	}
	result := initializationResultFromRuntime(c.lifecycleState().initializeResponseValue())
	raw := result.Raw
	if raw == nil {
		raw = map[string]any{}
		result.Raw = raw
	}
	raw["session_id"] = c.currentSessionID()
	raw["cwd"] = c.options.CWD
	raw["model"] = c.options.Model
	return result, nil
}

func (c *sessionCore) supportedCommands(ctx context.Context) ([]SlashCommand, error) {
	result, err := c.initializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return result.Commands, nil
}

func (c *sessionCore) supportedModels(ctx context.Context) ([]ModelInfo, error) {
	result, err := c.initializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return result.Models, nil
}

func (c *sessionCore) supportedAgents(ctx context.Context) ([]AgentInfo, error) {
	result, err := c.initializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return result.Agents, nil
}

func (c *sessionCore) accountInfo(ctx context.Context) (AccountInfo, error) {
	result, err := c.initializationResult(ctx)
	if err != nil {
		return AccountInfo{}, err
	}
	return result.Account, nil
}

func (c *sessionCore) serverInfo(ctx context.Context) (map[string]any, error) {
	result, err := c.initializationResult(ctx)
	if err != nil {
		return nil, err
	}
	return jsonvalue.CloneMap(result.Raw), nil
}

func (o Options) resolvedMCPServers() map[string]mcp.ServerConfig {
	return mcpwire.ResolveServers(o.MCP.Servers, o.MCP.SDKServers)
}

func (o Options) sdkMCPServerRegistry() map[string]mcp.SDKMCPServer {
	return mcpwire.SDKServerRegistry(o.MCP.Servers, o.MCP.SDKServers)
}

// getMCPStatus 获取 MCP 服务器状态。
func (c *sessionCore) getMCPStatus(ctx context.Context) (mcp.StatusResponse, error) {
	if !c.isConnected() {
		return mcp.StatusResponse{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "mcp_status",
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return mcp.StatusResponse{}, err
	}
	decoded := mcpwire.DecodeStatusResponse(response)
	return decoded, nil
}

// setMCPServers 动态替换当前 SDK 管理的 MCP 服务集合。
func (c *sessionCore) setMCPServers(ctx context.Context, servers map[string]mcp.ServerConfig) (mcp.SetServersResult, error) {
	if !c.isConnected() {
		return mcp.SetServersResult{}, ErrNotConnected
	}

	serializedServers, sdkServers, err := mcpwire.SerializeServers(servers)
	if err != nil {
		return mcp.SetServersResult{}, err
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "mcp_set_servers",
			Servers: serializedServers,
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return mcp.SetServersResult{}, err
	}

	c.replaceSDKMCPServers(sdkServers)
	decoded := mcpwire.DecodeSetServersResult(response)
	return decoded, nil
}

// mcpAuthenticate 为指定 MCP 服务启动认证流程。
func (c *sessionCore) mcpAuthenticate(ctx context.Context, serverName string) (map[string]any, error) {
	if !c.isConnected() {
		return nil, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:    "mcp_authenticate",
			ServerName: serverName,
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// mcpClearAuth 清除指定 MCP 服务的认证状态。
func (c *sessionCore) mcpClearAuth(ctx context.Context, serverName string) (map[string]any, error) {
	if !c.isConnected() {
		return nil, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:    "mcp_clear_auth",
			ServerName: serverName,
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// submitMCPOAuthCallbackURL 提交 MCP OAuth 回调 URL。
func (c *sessionCore) submitMCPOAuthCallbackURL(ctx context.Context, serverName string, callbackURL string) (map[string]any, error) {
	if !c.isConnected() {
		return nil, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:     "mcp_oauth_callback_url",
			ServerName:  serverName,
			CallbackURL: callbackURL,
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *sessionCore) reconnectMCPServer(ctx context.Context, serverName string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return errors.New("client: mcp server name is required")
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:    "mcp_reconnect",
			ServerName: serverName,
		},
		c.options.Runtime.InitializeTimeout,
	)
	return err
}

func (c *sessionCore) setMCPServerEnabled(ctx context.Context, serverName string, enabled bool) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return errors.New("client: mcp server name is required")
	}

	_, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype:    "mcp_toggle",
			ServerName: serverName,
			Enabled:    pointerTo(enabled),
		},
		c.options.Runtime.InitializeTimeout,
	)
	return err
}

func (c *sessionCore) resolveMCPMessage(ctx context.Context, request map[string]any) (map[string]any, error) {
	serverName := jsonvalue.StringValue(request["server_name"])
	if serverName == "" {
		return nil, errors.New("mcp server name is missing")
	}

	server, ok := c.sdkMCPServer(serverName)
	if !ok {
		return nil, fmt.Errorf("sdk mcp server not found: %s", serverName)
	}

	message := jsonvalue.MapValue(request["message"])
	if len(message) == 0 {
		return nil, errors.New("mcp message is missing")
	}

	return server.HandleMessage(ctx, message)
}

func (c *sessionCore) replaceSDKMCPServers(servers map[string]mcp.SDKMCPServer) {
	filtered := make(map[string]mcp.SDKMCPServer, len(servers))
	for name, server := range servers {
		if server == nil {
			continue
		}
		filtered[name] = server
	}
	c.sdkMCPServerRegistry().replace(filtered)
}

func (c *sessionCore) currentSDKMCPServers() map[string]mcp.SDKMCPServer {
	return c.sdkMCPServerRegistry().snapshot()
}

func (c *sessionCore) sdkMCPServer(name string) (mcp.SDKMCPServer, bool) {
	return c.sdkMCPServerRegistry().get(name)
}
