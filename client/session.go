package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const defaultSessionID = "default"

// sessionClient 表示可多轮交互的 Nexus Agent SDK bridge 客户端。
type sessionClient struct {
	runner
}

// runner 持有底层会话运行态，公开 API 通过 Session 收口。
type runner struct {
	options          Options
	lifecycle        *sessionLifecycle
	streams          *sessionStreams
	transport        Transport
	transportFactory func(Options) Transport
	customTransport  bool

	pendingRequests  *pendingControlRequests
	inflightRequests *inflightControlRequests
	hookCallbacks    *hookCallbackRegistry
	sdkMCPServers    *registry[mcp.SDKMCPServer]
}

func newSessionClient(options Options) *sessionClient {
	return newSessionClientWithTransport(options, nil)
}

func newSessionClientWithTransport(options Options, customTransport Transport) *sessionClient {
	return &sessionClient{runner: newRunner(options, customTransport, nil, customTransport != nil)}
}

func newRunner(
	options Options,
	transport Transport,
	transportFactory func(Options) Transport,
	customTransport bool,
) runner {
	return runner{
		options:          options,
		lifecycle:        newSessionLifecycle(),
		streams:          newSessionStreams(64),
		transport:        transport,
		transportFactory: transportFactory,
		customTransport:  customTransport,
		pendingRequests:  newPendingControlRequests(),
		inflightRequests: newInflightControlRequests(),
		hookCallbacks:    newHookCallbackRegistry(),
		sdkMCPServers:    newRegistry[mcp.SDKMCPServer](),
	}
}

func (c *sessionClient) lifecycleState() *sessionLifecycle {
	if c.lifecycle == nil {
		c.lifecycle = newSessionLifecycle()
	}
	return c.lifecycle
}

func (c *sessionClient) streamState() *sessionStreams {
	if c.streams == nil {
		c.streams = newSessionStreams(64)
	}
	return c.streams
}

func (c *sessionClient) hookCallbackRegistry() *hookCallbackRegistry {
	if c.hookCallbacks == nil {
		c.hookCallbacks = newHookCallbackRegistry()
	}
	return c.hookCallbacks
}

func (c *sessionClient) sdkMCPServerRegistry() *registry[mcp.SDKMCPServer] {
	if c.sdkMCPServers == nil {
		c.sdkMCPServers = newRegistry[mcp.SDKMCPServer]()
	}
	return c.sdkMCPServers
}

func joinErrors(left error, right error) error {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	return errors.Join(left, right)
}

func pointerTo[T any](value T) *T {
	return &value
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Session 表示一个可多轮交互的 SDK 会话。
type Session struct {
	client *sessionClient
}

// NewSession 创建并初始化新会话。
func NewSession(ctx context.Context, options Options) (*Session, error) {
	return newSession(ctx, options)
}

// ResumeSession 恢复既有会话。
func ResumeSession(ctx context.Context, sessionID string, options Options) (*Session, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("client: resume session id is required")
	}

	options.Session.ResumeID = sessionID
	options.Session.ContinueLatest = false
	return newSession(ctx, options)
}

func newSession(ctx context.Context, options Options) (*Session, error) {
	options, transport := applyBackend(options)
	instance := newSessionClientWithTransport(options, transport)
	if err := instance.Connect(ctx); err != nil {
		return nil, err
	}
	return &Session{client: instance}, nil
}

// Send 发送一条文本用户消息并返回本轮响应流。
func (s *Session) Send(ctx context.Context, prompt string) (*Stream, error) {
	return s.SendWithOptions(ctx, prompt, protocol.OutboundMessageOptions{})
}

// SendWithOptions 发送一条带附加语义的文本用户消息并返回本轮响应流。
func (s *Session) SendWithOptions(ctx context.Context, prompt string, options protocol.OutboundMessageOptions) (*Stream, error) {
	if s == nil || s.client == nil {
		return nil, ErrNotConnected
	}
	if err := s.client.SendWithOptions(ctx, prompt, nil, "", options); err != nil {
		return nil, err
	}
	return &Stream{client: s.client}, nil
}

// SendMessage 发送一条强类型用户消息并返回本轮响应流。
func (s *Session) SendMessage(ctx context.Context, message protocol.OutboundMessage) (*Stream, error) {
	return s.SendMessageWithOptions(ctx, message, protocol.OutboundMessageOptions{})
}

// SendMessageWithOptions 发送一条带附加语义的强类型用户消息并返回本轮响应流。
func (s *Session) SendMessageWithOptions(ctx context.Context, message protocol.OutboundMessage, options protocol.OutboundMessageOptions) (*Stream, error) {
	if s == nil || s.client == nil {
		return nil, ErrNotConnected
	}
	if err := s.client.SendMessageWithOptions(ctx, message, "", options); err != nil {
		return nil, err
	}
	return &Stream{client: s.client}, nil
}

// ID 返回当前会话 ID；新会话在收到真实 session_id 前返回空字符串。
func (s *Session) ID() string {
	if s == nil || s.client == nil {
		return ""
	}
	sessionID := s.client.SessionID()
	if sessionID == defaultSessionID {
		return ""
	}
	return sessionID
}

// Close 关闭会话。
func (s *Session) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}

// Recv 读取会话底层下一条消息。
func (s *Session) Recv(ctx context.Context) (protocol.ReceivedMessage, error) {
	if s == nil || s.client == nil {
		return protocol.ReceivedMessage{}, ErrNotConnected
	}
	return (&Stream{client: s.client}).Recv(ctx)
}

// Wait 等待会话底层传输结束。
func (s *Session) Wait() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Wait()
}

// Interrupt 中断当前执行。
func (s *Session) Interrupt(ctx context.Context) error {
	if s == nil || s.client == nil {
		return ErrNotConnected
	}
	return s.client.interrupt(ctx)
}

// Control 返回当前会话的运行期控制能力。
func (s *Session) Control() *SessionControl {
	return &SessionControl{session: s}
}

// MCP 返回当前会话的 MCP 控制能力。
func (s *Session) MCP() *SessionMCP {
	return &SessionMCP{session: s}
}

func (s *Session) activeClient() (*sessionClient, error) {
	if s == nil || s.client == nil {
		return nil, ErrNotConnected
	}
	return s.client, nil
}

// SessionControl 承载会话运行期控制能力。
type SessionControl struct {
	session *Session
}

func (c *SessionControl) activeClient() (*sessionClient, error) {
	if c == nil {
		return nil, ErrNotConnected
	}
	return c.session.activeClient()
}

// SetPermissionMode 修改权限模式。
func (c *SessionControl) SetPermissionMode(ctx context.Context, mode permission.Mode) error {
	client, err := c.activeClient()
	if err != nil {
		return err
	}
	return client.setPermissionMode(ctx, mode)
}

// SetModel 修改当前模型。
func (c *SessionControl) SetModel(ctx context.Context, model string) error {
	client, err := c.activeClient()
	if err != nil {
		return err
	}
	return client.setModel(ctx, model)
}

// SetMaxThinkingTokens 修改最大思考 token。
func (c *SessionControl) SetMaxThinkingTokens(ctx context.Context, maxThinkingTokens int) error {
	client, err := c.activeClient()
	if err != nil {
		return err
	}
	return client.setMaxThinkingTokens(ctx, maxThinkingTokens)
}

// SetNextTurnContext 尝试为下一轮设置内部上下文；当前 process backend 不支持时返回 ErrUnsupportedCapability。
func (c *SessionControl) SetNextTurnContext(ctx context.Context, blocks []InternalContextBlock) error {
	client, err := c.activeClient()
	if err != nil {
		return err
	}
	return client.setNextTurnContext(ctx, blocks)
}

// ContextUsage 获取当前上下文使用量。
func (c *SessionControl) ContextUsage(ctx context.Context) (protocol.ContextUsageResponse, error) {
	client, err := c.activeClient()
	if err != nil {
		return protocol.ContextUsageResponse{}, err
	}
	return client.getContextUsage(ctx)
}

// RewindFiles 回滚指定用户消息造成的文件变更。
func (c *SessionControl) RewindFiles(
	ctx context.Context,
	userMessageID string,
	dryRun bool,
) (protocol.RewindFilesResult, error) {
	client, err := c.activeClient()
	if err != nil {
		return protocol.RewindFilesResult{}, err
	}
	return client.rewindFiles(ctx, userMessageID, dryRun)
}

// SessionMCP 承载会话 MCP 控制能力。
type SessionMCP struct {
	session *Session
}

func (m *SessionMCP) activeClient() (*sessionClient, error) {
	if m == nil {
		return nil, ErrNotConnected
	}
	return m.session.activeClient()
}

// Status 获取 MCP 服务器状态。
func (m *SessionMCP) Status(ctx context.Context) (protocol.MCPStatusResponse, error) {
	client, err := m.activeClient()
	if err != nil {
		return protocol.MCPStatusResponse{}, err
	}
	return client.getMCPStatus(ctx)
}

// SetServers 动态替换当前 SDK 管理的 MCP 服务集合。
func (m *SessionMCP) SetServers(ctx context.Context, servers map[string]mcp.ServerConfig) (protocol.MCPSetServersResult, error) {
	client, err := m.activeClient()
	if err != nil {
		return protocol.MCPSetServersResult{}, err
	}
	return client.setMCPServers(ctx, servers)
}

// Authenticate 为指定 MCP 服务启动认证流程。
func (m *SessionMCP) Authenticate(ctx context.Context, serverName string) (map[string]any, error) {
	client, err := m.activeClient()
	if err != nil {
		return nil, err
	}
	return client.mcpAuthenticate(ctx, serverName)
}

// ClearAuth 清除指定 MCP 服务的认证状态。
func (m *SessionMCP) ClearAuth(ctx context.Context, serverName string) (map[string]any, error) {
	client, err := m.activeClient()
	if err != nil {
		return nil, err
	}
	return client.mcpClearAuth(ctx, serverName)
}

// SubmitOAuthCallbackURL 提交 MCP OAuth 回调 URL。
func (m *SessionMCP) SubmitOAuthCallbackURL(ctx context.Context, serverName string, callbackURL string) (map[string]any, error) {
	client, err := m.activeClient()
	if err != nil {
		return nil, err
	}
	return client.submitMCPOAuthCallbackURL(ctx, serverName, callbackURL)
}
func (c *sessionClient) Connect(ctx context.Context) error {
	lifecycle := c.lifecycleState()
	lifecycle.lockConnection()
	if lifecycle.connectedLocked() {
		lifecycle.unlockConnection()
		return nil
	}
	c.resetLifecycleIfNeededLocked()

	normalizedOptions, err := c.options.normalized()
	if err != nil {
		lifecycle.unlockConnection()
		return err
	}
	c.options = normalizedOptions
	c.replaceSDKMCPServers(c.options.sdkMCPServerRegistry())

	if c.transport == nil {
		c.transport, err = c.buildTransport(c.options)
		if err != nil {
			lifecycle.setConnectedLocked(false)
			lifecycle.unlockConnection()
			return err
		}
	}
	lifecycle.setConnectedLocked(true)
	lifecycle.unlockConnection()

	if err := c.transport.Start(ctx); err != nil {
		lifecycle.setConnected(false)
		return classifyTransportStartError(c.options, err)
	}

	go c.readLoop()

	response, err := c.sendControlRequest(
		ctx,
		c.buildInitializeRequest(),
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		_ = c.Disconnect(ctx)
		return err
	}

	lifecycle.setInitializeResponse(protocol.DecodeInitializeResponse(response))
	return nil
}

// Wait 等待会话结束。
func (c *sessionClient) Wait() error {
	streams := c.streamState()
	<-streams.readDone
	if c.lifecycleState().inputStreamActiveValue() && !transport.ChannelClosed(streams.inputClosed) {
		c.finishInputStream()
	}

	var result error
	if c.transport != nil {
		result = c.transport.Wait()
	}
	if c.getReadError() == nil {
		result = withLastErrorResult(result, c.lifecycleState().lastErrorResultValue())
	}
	return classifyProcessExitError(joinErrors(c.getReadError(), result))
}

// Disconnect 断开连接。
func (c *sessionClient) Disconnect(ctx context.Context) error {
	if !c.isConnected() && c.transport == nil {
		return nil
	}
	streams := c.streamState()
	var disconnectErr error
	c.lifecycleState().closeOnceDo(func() {
		c.lifecycleState().setConnected(false)

		if c.transport != nil {
			disconnectErr = joinErrors(disconnectErr, c.transport.Close())
		}

		<-streams.readDone
		disconnectErr = joinErrors(disconnectErr, c.getReadError())
	})
	return disconnectErr
}

// SessionID 返回当前会话 ID。
func (c *sessionClient) SessionID() string {
	return c.lifecycleState().sessionIDValue()
}

func (c *sessionClient) isConnected() bool {
	return c.lifecycleState().isConnected()
}

func (c *sessionClient) resetLifecycleIfNeededLocked() {
	streams := c.streamState()
	if !transport.ChannelClosed(streams.readDone) {
		return
	}
	streams.reset()
	c.lifecycleState().resetRuntimeState("")
	if !c.customTransport {
		c.transport = nil
	}
}

func (c *sessionClient) markDisconnected() {
	c.lifecycleState().setConnected(false)
}

func (c *sessionClient) markTransportFailed(err error) {
	if err != nil {
		c.setReadError(err)
	}
	c.markDisconnected()
	c.failPendingRequests(joinErrors(c.getReadError(), ErrNotConnected))
}

func (c *sessionClient) currentSessionID() string {
	return c.lifecycleState().currentSessionID(defaultSessionID, "", c.options.Session.ResumeID)
}

func (c *sessionClient) buildTransport(options Options) (Transport, error) {
	if c.transportFactory == nil {
		if options.directConnect != nil {
			directConnectConfig, err := options.directConnectConfig()
			if err != nil {
				return nil, err
			}
			return transport.NewDirectConnectTransport(directConnectConfig), nil
		}
		return transport.NewProcessTransport(options.processConfig()), nil
	}
	transport := c.transportFactory(options)
	if transport == nil {
		return nil, errors.New("client: transport factory returned nil")
	}
	return transport, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return jsonvalue.CloneMap(input)
}

const defaultInterruptControlTimeout = 5 * time.Second

func (c *sessionClient) interrupt(ctx context.Context) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	if c.transport == nil {
		return ErrNotConnected
	}
	if err := c.transport.Interrupt(); err != nil {
		if !errors.Is(err, transport.ErrInterruptUnsupported) {
			return err
		}
		_, controlErr := c.sendControlRequest(
			ctx,
			protocol.ControlRequest{
				Subtype: "interrupt",
			},
			interruptControlTimeout(c.options.Runtime.InitializeTimeout),
		)
		return controlErr
	}
	return nil
}

func interruptControlTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > defaultInterruptControlTimeout {
		return defaultInterruptControlTimeout
	}
	return timeout
}

// setPermissionMode 修改权限模式。
func (c *sessionClient) setPermissionMode(ctx context.Context, mode permission.Mode) error {
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
func (c *sessionClient) setModel(ctx context.Context, model string) error {
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
func (c *sessionClient) setMaxThinkingTokens(ctx context.Context, maxThinkingTokens int) error {
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

// getContextUsage 获取当前上下文使用情况。
func (c *sessionClient) getContextUsage(ctx context.Context) (protocol.ContextUsageResponse, error) {
	if !c.isConnected() {
		return protocol.ContextUsageResponse{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "get_context_usage",
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return protocol.ContextUsageResponse{}, err
	}
	return protocol.DecodeContextUsageResponse(response), nil
}

func normalizePermissionMode(mode permission.Mode) permission.Mode {
	if mode == "" {
		return permission.ModeDefault
	}
	return mode
}

// rewindFiles 将文件回滚到指定用户消息时刻。
func (c *sessionClient) rewindFiles(ctx context.Context, userMessageID string, dryRun bool) (protocol.RewindFilesResult, error) {
	if !c.isConnected() {
		return protocol.RewindFilesResult{}, ErrNotConnected
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
		return protocol.RewindFilesResult{}, err
	}
	return protocol.DecodeRewindFilesResult(response), nil
}
func (o Options) resolvedMCPServers() map[string]mcp.ServerConfig {
	return mcp.ResolveServers(o.MCP.Servers, o.MCP.SDKServers)
}

func (o Options) sdkMCPServerRegistry() map[string]mcp.SDKMCPServer {
	return mcp.SDKServerRegistry(o.MCP.Servers, o.MCP.SDKServers)
}

// getMCPStatus 获取 MCP 服务器状态。
func (c *sessionClient) getMCPStatus(ctx context.Context) (protocol.MCPStatusResponse, error) {
	if !c.isConnected() {
		return protocol.MCPStatusResponse{}, ErrNotConnected
	}

	response, err := c.sendControlRequest(
		ctx,
		protocol.ControlRequest{
			Subtype: "mcp_status",
		},
		c.options.Runtime.InitializeTimeout,
	)
	if err != nil {
		return protocol.MCPStatusResponse{}, err
	}
	decoded := protocol.DecodeMCPStatusResponse(response)
	return decoded, nil
}

// setMCPServers 动态替换当前 SDK 管理的 MCP 服务集合。
func (c *sessionClient) setMCPServers(ctx context.Context, servers map[string]mcp.ServerConfig) (protocol.MCPSetServersResult, error) {
	if !c.isConnected() {
		return protocol.MCPSetServersResult{}, ErrNotConnected
	}

	serializedServers, sdkServers, err := mcp.SerializeServers(servers)
	if err != nil {
		return protocol.MCPSetServersResult{}, err
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
		return protocol.MCPSetServersResult{}, err
	}

	c.replaceSDKMCPServers(sdkServers)
	decoded := protocol.DecodeMCPSetServersResult(response)
	return decoded, nil
}

// mcpAuthenticate 为指定 MCP 服务启动认证流程。
func (c *sessionClient) mcpAuthenticate(ctx context.Context, serverName string) (map[string]any, error) {
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
func (c *sessionClient) mcpClearAuth(ctx context.Context, serverName string) (map[string]any, error) {
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
func (c *sessionClient) submitMCPOAuthCallbackURL(ctx context.Context, serverName string, callbackURL string) (map[string]any, error) {
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

func (c *sessionClient) resolveMCPMessage(ctx context.Context, request map[string]any) (map[string]any, error) {
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

func (c *sessionClient) replaceSDKMCPServers(servers map[string]mcp.SDKMCPServer) {
	filtered := make(map[string]mcp.SDKMCPServer, len(servers))
	for name, server := range servers {
		if server == nil {
			continue
		}
		filtered[name] = server
	}
	c.sdkMCPServerRegistry().replace(filtered)
}

func (c *sessionClient) currentSDKMCPServers() map[string]mcp.SDKMCPServer {
	return c.sdkMCPServerRegistry().snapshot()
}

func (c *sessionClient) sdkMCPServer(name string) (mcp.SDKMCPServer, bool) {
	return c.sdkMCPServerRegistry().get(name)
}
func (c *sessionClient) ConnectWithPrompt(ctx context.Context, prompt string) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	return c.Query(ctx, prompt)
}

// ConnectWithMessages 建立连接并在后台流式发送强类型 SDK 消息。
func (c *sessionClient) ConnectWithMessages(ctx context.Context, messages <-chan protocol.OutboundMessage) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	c.lifecycleState().setInputStreamActive(true)
	c.startMessageStream(messages)
	return nil
}

// ConnectWithRawMessages 建立连接并在后台流式发送原始 SDK 消息。
func (c *sessionClient) ConnectWithRawMessages(ctx context.Context, messages <-chan map[string]any) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	c.lifecycleState().setInputStreamActive(true)
	c.startRawMessageStream(messages)
	return nil
}

// Query 发送一条字符串 prompt。
func (c *sessionClient) Query(ctx context.Context, prompt string) error {
	return c.Send(ctx, prompt, nil, "")
}

// Send 发送一条用户消息。
func (c *sessionClient) Send(ctx context.Context, prompt string, parentToolUseID *string, sessionID string) error {
	return c.SendWithOptions(ctx, prompt, parentToolUseID, sessionID, protocol.OutboundMessageOptions{})
}

// SendWithOptions 发送一条带附加语义的用户消息。
func (c *sessionClient) SendWithOptions(ctx context.Context, prompt string, parentToolUseID *string, sessionID string, options protocol.OutboundMessageOptions) error {
	return c.sendContentWithOptions(ctx, prompt, parentToolUseID, sessionID, options)
}

// SendMessage 发送一条强类型 SDK 消息。
func (c *sessionClient) SendMessage(ctx context.Context, message protocol.OutboundMessage, sessionID string) error {
	return c.SendMessageWithOptions(ctx, message, sessionID, protocol.OutboundMessageOptions{})
}

// SendMessageWithOptions 发送一条带附加语义的强类型 SDK 消息。
func (c *sessionClient) SendMessageWithOptions(ctx context.Context, message protocol.OutboundMessage, sessionID string, options protocol.OutboundMessageOptions) error {
	if message == nil {
		return nil
	}
	return c.SendRawMessage(ctx, protocol.EncodeOutboundMessageWithOptions(message, sessionID, options), sessionID)
}

// SendRawMessage 发送一条原始 SDK 消息。
func (c *sessionClient) SendRawMessage(ctx context.Context, message map[string]any, sessionID string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	if len(message) == 0 {
		return nil
	}

	payload := cloneMap(message)
	if payload["session_id"] == nil {
		effectiveSessionID := sessionID
		if effectiveSessionID == "" {
			effectiveSessionID = c.currentSessionID()
		}
		payload["session_id"] = effectiveSessionID
	}

	if c.transport == nil {
		return ErrNotConnected
	}
	if err := c.transport.WriteJSON(payload); err != nil {
		if transport.IsTransportWriteFailure(err) {
			c.markTransportFailed(fmt.Errorf("client: send sdk message failed: %w", err))
		}
		return fmt.Errorf("client: send sdk message failed: %w", err)
	}
	return nil
}

func (c *sessionClient) startMessageStream(messages <-chan protocol.OutboundMessage) {
	streams := c.streamState()
	go func() {
		for {
			select {
			case <-streams.readDone:
				c.finishInputStreamFor(streams)
				return
			case message, ok := <-messages:
				if !ok {
					c.finishInputStreamFor(streams)
					return
				}
				if err := c.SendMessage(context.Background(), message, ""); err != nil {
					c.setReadError(err)
					_ = c.Disconnect(context.Background())
					return
				}
			}
		}
	}()
}

func (c *sessionClient) startRawMessageStream(messages <-chan map[string]any) {
	streams := c.streamState()
	go func() {
		for {
			select {
			case <-streams.readDone:
				c.finishInputStreamFor(streams)
				return
			case message, ok := <-messages:
				if !ok {
					c.finishInputStreamFor(streams)
					return
				}
				if err := c.SendRawMessage(context.Background(), message, ""); err != nil {
					c.setReadError(err)
					_ = c.Disconnect(context.Background())
					return
				}
			}
		}
	}()
}

func (c *sessionClient) finishInputStream() {
	c.finishInputStreamFor(c.streamState())
}

func (c *sessionClient) finishInputStreamFor(streams *sessionStreams) {
	select {
	case <-streams.firstResult:
	case <-streams.readDone:
	}
	_ = c.CloseInput()
	c.lifecycleState().inputCloseOnceDo(func() {
		c.lifecycleState().setInputStreamActive(false)
		close(streams.inputClosed)
	})
}

func (c *sessionClient) sendContent(ctx context.Context, content any, parentToolUseID *string, sessionID string) error {
	return c.sendContentWithOptions(ctx, content, parentToolUseID, sessionID, protocol.OutboundMessageOptions{})
}

func (c *sessionClient) sendContentWithOptions(ctx context.Context, content any, parentToolUseID *string, sessionID string, options protocol.OutboundMessageOptions) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	effectiveSessionID := sessionID
	if effectiveSessionID == "" {
		effectiveSessionID = c.currentSessionID()
	}

	payload := map[string]any{
		"type":               "user",
		"session_id":         effectiveSessionID,
		"parent_tool_use_id": parentToolUseID,
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}
	payload = protocol.ApplyOutboundMessageOptions(payload, options)

	return c.SendRawMessage(ctx, payload, effectiveSessionID)
}

func (c *sessionClient) setNextTurnContext(ctx context.Context, blocks []InternalContextBlock) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	if !c.supports(CapabilityInternalContext) {
		return &UnsupportedCapabilityError{Capability: CapabilityInternalContext}
	}
	return &UnsupportedCapabilityError{Capability: CapabilityInternalContext}
}

// ReceiveMessages 返回消息流。
func (c *sessionClient) ReceiveMessages(ctx context.Context) <-chan protocol.ReceivedMessage {
	return c.streamState().messages
}

// Messages 返回消息流。
func (c *sessionClient) Messages() <-chan protocol.ReceivedMessage {
	return c.streamState().messages
}

// ReceiveResponse 读取直到首个 result 的消息流。
func (c *sessionClient) ReceiveResponse(ctx context.Context) <-chan protocol.ReceivedMessage {
	response := make(chan protocol.ReceivedMessage, 16)
	go func() {
		defer close(response)
		streams := c.streamState()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-streams.messages:
				if !ok {
					return
				}
				response <- message
				if message.Type == protocol.MessageTypeResult {
					return
				}
			}
		}
	}()
	return response
}

// CloseInput 主动关闭输入流。
func (c *sessionClient) CloseInput() error {
	if c.transport == nil {
		return ErrNotConnected
	}
	return c.transport.EndInput()
}

func (c *sessionClient) readLoop() {
	streams := c.streamState()
	defer c.markDisconnected()
	defer close(streams.readDone)
	defer close(streams.messages)

	for {
		payload, err := c.transport.ReadJSON()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.setReadError(fmt.Errorf("client: read message failed: %w", err))
			}
			c.failPendingRequests(joinErrors(c.getReadError(), ErrNotConnected))
			return
		}

		switch payloadType := jsonvalue.StringValue(payload["type"]); payloadType {
		case "control_response":
			c.handleControlResponse(payload)
		case "control_request":
			go c.handleControlRequest(payload)
		case "control_cancel_request":
			c.handleControlCancelRequest(payload)
		default:
			message, decodeErr := protocol.DecodeMessage(payload)
			if decodeErr != nil {
				c.setReadError(decodeErr)
				c.failPendingRequests(decodeErr)
				return
			}

			if message.SessionID != "" {
				c.lifecycleState().setSessionID(message.SessionID)
			}
			c.trackLastErrorResult(message)
			if message.Type == protocol.MessageTypeResult {
				c.signalFirstResult()
			}
			c.emitMessage(message)
		}
	}
}

func (c *sessionClient) sendInternalRawMessage(message map[string]any, sessionID string) error {
	if !c.isConnected() {
		return ErrNotConnected
	}
	if len(message) == 0 {
		return nil
	}

	payload := cloneMap(message)
	if payload["session_id"] == nil {
		effectiveSessionID := sessionID
		if effectiveSessionID == "" {
			effectiveSessionID = c.currentSessionID()
		}
		payload["session_id"] = effectiveSessionID
	}
	if c.transport == nil {
		return ErrNotConnected
	}
	if err := c.transport.WriteJSON(payload); err != nil {
		if transport.IsTransportWriteFailure(err) {
			c.markTransportFailed(fmt.Errorf("client: send continuation message failed: %w", err))
		}
		return fmt.Errorf("client: send continuation message failed: %w", err)
	}
	return nil
}

func (c *sessionClient) emitMessage(message protocol.ReceivedMessage) {
	streams := c.streamState()
	if message.Type == protocol.MessageTypeResult {
		c.signalFirstResult()
	}
	streams.messages <- message
}

func (c *sessionClient) signalFirstResult() {
	streams := c.streamState()
	c.lifecycleState().firstResultOnceDo(func() {
		close(streams.firstResult)
	})
}

func (c *sessionClient) trackLastErrorResult(message protocol.ReceivedMessage) {
	if message.Type == protocol.MessageTypeResult && message.Result != nil {
		if message.Result.IsError {
			c.lifecycleState().setLastErrorResult(resultErrorText(message.Result))
			return
		}
		c.lifecycleState().setLastErrorResult("")
		return
	}
	if message.Type == protocol.MessageTypeSystem && message.Subtype == "session_state_changed" {
		return
	}
	c.lifecycleState().setLastErrorResult("")
}

func resultErrorText(result *protocol.ResultMessage) string {
	if result == nil {
		return "unknown error"
	}
	if len(result.Errors) > 0 {
		parts := make([]string, 0, len(result.Errors))
		for _, item := range result.Errors {
			if text := strings.TrimSpace(item); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	if text := strings.TrimSpace(result.Subtype); text != "" {
		return text
	}
	return "unknown error"
}
