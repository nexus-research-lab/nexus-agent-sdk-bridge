package client

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const defaultSessionID = "default"

// sessionCore 是 Query 和 Session 共用的未导出执行核心。
type sessionCore struct {
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
	hookAppliedAcks  *registry[func(hook.AppliedAck)]
	sdkMCPServers    *registry[mcp.SDKMCPServer]
	nextTurnContext  *nextTurnContextBuffer
}

func newSessionCore(options Options) *sessionCore {
	return newSessionCoreWithTransport(options, nil)
}

func newSessionCoreWithTransport(options Options, customTransport Transport) *sessionCore {
	return &sessionCore{runner: newRunner(options, customTransport, nil, customTransport != nil)}
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
		hookAppliedAcks:  newRegistry[func(hook.AppliedAck)](),
		sdkMCPServers:    newRegistry[mcp.SDKMCPServer](),
		nextTurnContext:  newNextTurnContextBuffer(),
	}
}

func (c *sessionCore) lifecycleState() *sessionLifecycle {
	if c.lifecycle == nil {
		c.lifecycle = newSessionLifecycle()
	}
	return c.lifecycle
}

func (c *sessionCore) streamState() *sessionStreams {
	if c.streams == nil {
		c.streams = newSessionStreams(64)
	}
	return c.streams
}

func (c *sessionCore) hookCallbackRegistry() *hookCallbackRegistry {
	if c.hookCallbacks == nil {
		c.hookCallbacks = newHookCallbackRegistry()
	}
	return c.hookCallbacks
}

func (c *sessionCore) hookAppliedAckRegistry() *registry[func(hook.AppliedAck)] {
	if c.hookAppliedAcks == nil {
		c.hookAppliedAcks = newRegistry[func(hook.AppliedAck)]()
	}
	return c.hookAppliedAcks
}

func (c *sessionCore) sdkMCPServerRegistry() *registry[mcp.SDKMCPServer] {
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
	core *sessionCore
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
	instance := newSessionCoreWithTransport(options, options.Transport)
	if err := instance.Connect(ctx); err != nil {
		return nil, err
	}
	return &Session{core: instance}, nil
}

// Send 发送一条文本用户消息并返回本轮响应流。
func (s *Session) Send(ctx context.Context, prompt string) (*Stream, error) {
	return s.SendWithOptions(ctx, prompt, protocol.OutboundMessageOptions{})
}

// SendWithOptions 发送一条带附加语义的文本用户消息并返回本轮响应流。
func (s *Session) SendWithOptions(ctx context.Context, prompt string, options protocol.OutboundMessageOptions) (*Stream, error) {
	if s == nil || s.core == nil {
		return nil, ErrNotConnected
	}
	if err := s.core.SendWithOptions(ctx, prompt, nil, "", options); err != nil {
		return nil, err
	}
	return &Stream{core: s.core}, nil
}

// SendMessage 发送一条强类型用户消息并返回本轮响应流。
func (s *Session) SendMessage(ctx context.Context, message protocol.OutboundMessage) (*Stream, error) {
	return s.SendMessageWithOptions(ctx, message, protocol.OutboundMessageOptions{})
}

// SendMessageWithOptions 发送一条带附加语义的强类型用户消息并返回本轮响应流。
func (s *Session) SendMessageWithOptions(ctx context.Context, message protocol.OutboundMessage, options protocol.OutboundMessageOptions) (*Stream, error) {
	if s == nil || s.core == nil {
		return nil, ErrNotConnected
	}
	if err := s.core.SendMessageWithOptions(ctx, message, "", options); err != nil {
		return nil, err
	}
	return &Stream{core: s.core}, nil
}

// StreamInput 绑定一个持续输入通道，后续用户消息会按通道顺序进入会话。
func (s *Session) StreamInput(ctx context.Context, messages <-chan protocol.OutboundMessage) (*Stream, error) {
	if s == nil || s.core == nil {
		return nil, ErrNotConnected
	}
	if messages == nil {
		return nil, errors.New("client: message stream is required")
	}
	select {
	case <-ctx.Done():
		return nil, abortError(ctx.Err())
	default:
	}
	if err := s.core.ConnectWithMessages(ctx, messages); err != nil {
		return nil, abortError(err)
	}
	return &Stream{core: s.core}, nil
}

// ID 返回当前会话 ID；新会话在收到真实 session_id 前返回空字符串。
func (s *Session) ID() string {
	if s == nil || s.core == nil {
		return ""
	}
	sessionID := s.core.SessionID()
	if sessionID == defaultSessionID {
		return ""
	}
	return sessionID
}

// Close 关闭会话。
func (s *Session) Close(ctx context.Context) error {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.Disconnect(ctx)
}

// Recv 读取会话底层下一条消息。
func (s *Session) Recv(ctx context.Context) (protocol.ReceivedMessage, error) {
	if s == nil || s.core == nil {
		return protocol.ReceivedMessage{}, ErrNotConnected
	}
	return (&Stream{core: s.core}).Recv(ctx)
}

// Wait 等待会话底层传输结束。
func (s *Session) Wait() error {
	if s == nil || s.core == nil {
		return nil
	}
	return abortError(s.core.Wait())
}

// Interrupt 中断当前执行。
func (s *Session) Interrupt(ctx context.Context) error {
	if s == nil || s.core == nil {
		return ErrNotConnected
	}
	return s.core.interrupt(ctx)
}

// InterruptWithReason 按指定原因中断当前执行。
func (s *Session) InterruptWithReason(ctx context.Context, reason string) error {
	if s == nil || s.core == nil {
		return ErrNotConnected
	}
	return s.core.interruptWithReason(ctx, reason)
}

// Control 返回当前会话的运行期控制能力。
func (s *Session) Control() *SessionControl {
	return &SessionControl{session: s}
}

// MCP 返回当前会话的 MCP 控制能力。
func (s *Session) MCP() *SessionMCP {
	return &SessionMCP{session: s}
}

func (s *Session) activeCore() (*sessionCore, error) {
	if s == nil || s.core == nil {
		return nil, ErrNotConnected
	}
	return s.core, nil
}

// SessionControl 承载会话运行期控制能力。
type SessionControl struct {
	session *Session
}

func (c *SessionControl) activeCore() (*sessionCore, error) {
	if c == nil || c.session == nil {
		return nil, ErrNotConnected
	}
	return c.session.activeCore()
}

// SetPermissionMode 修改权限模式。
func (c *SessionControl) SetPermissionMode(ctx context.Context, mode permission.Mode) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	return core.setPermissionMode(ctx, mode)
}

// SetModel 修改当前模型。
func (c *SessionControl) SetModel(ctx context.Context, model string) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	return core.setModel(ctx, model)
}

// SetMaxThinkingTokens 修改最大思考 token。
func (c *SessionControl) SetMaxThinkingTokens(ctx context.Context, maxThinkingTokens int) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	return core.setMaxThinkingTokens(ctx, maxThinkingTokens)
}

// ContextUsage 获取当前上下文使用量。
func (c *SessionControl) ContextUsage(ctx context.Context) (ContextUsageResponse, error) {
	core, err := c.activeCore()
	if err != nil {
		return ContextUsageResponse{}, err
	}
	return core.getContextUsage(ctx)
}

// RewindFiles 回滚指定用户消息造成的文件变更。
func (c *SessionControl) RewindFiles(
	ctx context.Context,
	userMessageID string,
	dryRun bool,
) (RewindFilesResult, error) {
	core, err := c.activeCore()
	if err != nil {
		return RewindFilesResult{}, err
	}
	return core.rewindFiles(ctx, userMessageID, dryRun)
}

// RemoveMessages 删除当前会话中的指定消息 UUID，并同步 runtime transcript。
func (c *SessionControl) RemoveMessages(ctx context.Context, uuids []string) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	return core.removeMessages(ctx, uuids)
}

// StopTask 停止指定后台任务。
func (c *SessionControl) StopTask(ctx context.Context, taskID string) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	if !core.supports(CapabilityStopTask) {
		return &UnsupportedCapabilityError{Capability: CapabilityStopTask}
	}
	return core.stopTask(ctx, taskID)
}

// SendTaskMessage 向后台 subagent task 排队一条后续消息。
func (c *SessionControl) SendTaskMessage(ctx context.Context, taskID string, message string, summary string) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	if !core.supports(CapabilitySendTaskMessage) {
		return &UnsupportedCapabilityError{Capability: CapabilitySendTaskMessage}
	}
	return core.sendTaskMessage(ctx, taskID, message, summary)
}

// TryAutoDream 唤醒 nxs 的 AutoDream 检查，并等待跳过或完成结果。
func (c *SessionControl) TryAutoDream(ctx context.Context) (AutoDreamResult, error) {
	core, err := c.activeCore()
	if err != nil {
		return AutoDreamResult{}, err
	}
	if !core.supports(CapabilityAutoDream) {
		return AutoDreamResult{}, &UnsupportedCapabilityError{Capability: CapabilityAutoDream}
	}
	return core.tryAutoDream(ctx)
}

// ApplyFlagSettings 合并当前会话的运行时 flag settings。
func (c *SessionControl) ApplyFlagSettings(ctx context.Context, settings map[string]any) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	return core.applyFlagSettings(ctx, settings)
}

// GetSettings 返回当前会话合并后的 runtime settings。
func (c *SessionControl) GetSettings(ctx context.Context) (SettingsResponse, error) {
	core, err := c.activeCore()
	if err != nil {
		return SettingsResponse{}, err
	}
	return core.getSettings(ctx)
}

// SupportedCommands 返回当前会话可调用的 slash command。
func (c *SessionControl) SupportedCommands(ctx context.Context) ([]SlashCommand, error) {
	core, err := c.activeCore()
	if err != nil {
		return nil, err
	}
	return core.supportedCommands(ctx)
}

// SupportedModels 返回当前会话可选模型。
func (c *SessionControl) SupportedModels(ctx context.Context) ([]ModelInfo, error) {
	core, err := c.activeCore()
	if err != nil {
		return nil, err
	}
	return core.supportedModels(ctx)
}

// SupportedAgents 返回当前会话可用 agent。
func (c *SessionControl) SupportedAgents(ctx context.Context) ([]AgentInfo, error) {
	core, err := c.activeCore()
	if err != nil {
		return nil, err
	}
	return core.supportedAgents(ctx)
}

// AccountInfo 返回当前会话账号快照。
func (c *SessionControl) AccountInfo(ctx context.Context) (AccountInfo, error) {
	core, err := c.activeCore()
	if err != nil {
		return AccountInfo{}, err
	}
	return core.accountInfo(ctx)
}

// InitializationResult 返回当前会话初始化快照。
func (c *SessionControl) InitializationResult(ctx context.Context) (InitializationResult, error) {
	core, err := c.activeCore()
	if err != nil {
		return InitializationResult{}, err
	}
	return core.initializationResult(ctx)
}

// ServerInfo 返回当前会话的 runtime 初始化快照。
func (c *SessionControl) ServerInfo(ctx context.Context) (map[string]any, error) {
	core, err := c.activeCore()
	if err != nil {
		return nil, err
	}
	return core.serverInfo(ctx)
}

// SetNextTurnContext 尝试为下一轮注入内部上下文。
func (c *SessionControl) SetNextTurnContext(ctx context.Context, blocks []InternalContextBlock) error {
	core, err := c.activeCore()
	if err != nil {
		return err
	}
	return core.setNextTurnContext(ctx, blocks)
}

// SessionMCP 承载会话 MCP 控制能力。
type SessionMCP struct {
	session *Session
}

func (m *SessionMCP) activeCore() (*sessionCore, error) {
	if m == nil || m.session == nil {
		return nil, ErrNotConnected
	}
	return m.session.activeCore()
}

// Status 获取 MCP 服务器状态。
func (m *SessionMCP) Status(ctx context.Context) (mcp.StatusResponse, error) {
	core, err := m.activeCore()
	if err != nil {
		return mcp.StatusResponse{}, err
	}
	return core.getMCPStatus(ctx)
}

// SetServers 动态替换当前 SDK 管理的 MCP 服务集合。
func (m *SessionMCP) SetServers(ctx context.Context, servers map[string]mcp.ServerConfig) (mcp.SetServersResult, error) {
	core, err := m.activeCore()
	if err != nil {
		return mcp.SetServersResult{}, err
	}
	return core.setMCPServers(ctx, servers)
}

// Authenticate 为指定 MCP 服务启动认证流程。
func (m *SessionMCP) Authenticate(ctx context.Context, serverName string) (map[string]any, error) {
	core, err := m.activeCore()
	if err != nil {
		return nil, err
	}
	return core.mcpAuthenticate(ctx, serverName)
}

// ClearAuth 清除指定 MCP 服务的认证状态。
func (m *SessionMCP) ClearAuth(ctx context.Context, serverName string) (map[string]any, error) {
	core, err := m.activeCore()
	if err != nil {
		return nil, err
	}
	return core.mcpClearAuth(ctx, serverName)
}

// SubmitOAuthCallbackURL 提交 MCP OAuth 回调 URL。
func (m *SessionMCP) SubmitOAuthCallbackURL(ctx context.Context, serverName string, callbackURL string) (map[string]any, error) {
	core, err := m.activeCore()
	if err != nil {
		return nil, err
	}
	return core.submitMCPOAuthCallbackURL(ctx, serverName, callbackURL)
}

// Reconnect 重新连接指定 MCP 服务。
func (m *SessionMCP) Reconnect(ctx context.Context, serverName string) error {
	core, err := m.activeCore()
	if err != nil {
		return err
	}
	return core.reconnectMCPServer(ctx, serverName)
}

// SetEnabled 启用或禁用指定 MCP 服务。
func (m *SessionMCP) SetEnabled(ctx context.Context, serverName string, enabled bool) error {
	core, err := m.activeCore()
	if err != nil {
		return err
	}
	return core.setMCPServerEnabled(ctx, serverName, enabled)
}

// Toggle 是 SetEnabled 的语义别名。
func (m *SessionMCP) Toggle(ctx context.Context, serverName string, enabled bool) error {
	return m.SetEnabled(ctx, serverName, enabled)
}
