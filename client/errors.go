package client

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var (
	// ErrNotConnected 表示会话尚未连接或已经断开。
	ErrNotConnected = errors.New("client: not connected")
	// ErrNoResult 表示消息流结束前没有收到 result 消息。
	ErrNoResult = errors.New("client: stream closed before result message")
	// ErrAborted 表示 SDK 操作被调用方或会话中断取消。
	ErrAborted = errors.New("client: operation aborted")
	// ErrBypassPermissionsNotAllowed 表示会话启动时没有允许运行期切换到 bypassPermissions。
	ErrBypassPermissionsNotAllowed = errors.New("client: bypassPermissions requires allowDangerouslySkipPermissions at session launch")
	// ErrRestartRequired 表示 Reconfigure 遇到必须重启 runtime 进程才能生效的配置变化。
	ErrRestartRequired = errors.New("client: runtime restart required")
	// ErrUnsupportedCapability 表示当前后端不支持请求的运行时能力。
	ErrUnsupportedCapability = errors.New("client: unsupported runtime capability")
)

// RestartReason 表示运行时必须重启的配置差异原因。
type RestartReason string

const (
	// RestartReasonProcessEnvChanged 表示进程环境变量变化。
	RestartReasonProcessEnvChanged RestartReason = "process_env_changed"
	// RestartReasonToolPolicyChanged 表示启动期工具策略变化。
	RestartReasonToolPolicyChanged RestartReason = "tool_policy_changed"
	// RestartReasonMCPControlUnsupported 表示当前 runtime 不支持 MCP 热更新控制面。
	RestartReasonMCPControlUnsupported RestartReason = "mcp_control_unsupported"
)

// RestartRequiredError 携带 runtime 重启原因。
type RestartRequiredError struct {
	Reason RestartReason
	Cause  error
}

func (e *RestartRequiredError) Error() string {
	message := ErrRestartRequired.Error()
	if e == nil {
		return message
	}
	if e.Reason != "" {
		message += ": " + string(e.Reason)
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *RestartRequiredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *RestartRequiredError) Is(target error) bool {
	return target == ErrRestartRequired
}

// CLINotFoundError 表示本地 CLI 可执行文件未找到。
type CLINotFoundError struct {
	Command string
	Cause   error
}

func (e *CLINotFoundError) Error() string {
	command := strings.TrimSpace(e.Command)
	if command == "" {
		command = "cli"
	}
	message := fmt.Sprintf("client: cli executable %q not found", command)
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *CLINotFoundError) Unwrap() error {
	return e.Cause
}

func (e *CLINotFoundError) Is(target error) bool {
	_, ok := target.(*CLINotFoundError)
	return ok
}

// CLIConnectionError 表示 SDK 与底层 CLI transport 连接失败。
type CLIConnectionError struct {
	Message string
	Cause   error
}

func (e *CLIConnectionError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "client: cli connection failed"
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *CLIConnectionError) Unwrap() error {
	return e.Cause
}

func (e *CLIConnectionError) Is(target error) bool {
	_, ok := target.(*CLIConnectionError)
	return ok
}

// ProcessError 表示底层 CLI 进程异常退出。
type ProcessError struct {
	Message  string
	Stderr   string
	ExitCode int
	Cause    error
}

func (e *ProcessError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "client: cli process exited with error"
	}
	if e.ExitCode != 0 {
		message = fmt.Sprintf("%s (exit code %d)", message, e.ExitCode)
	}
	if e.Stderr != "" {
		message = fmt.Sprintf("%s: %s", message, strings.TrimSpace(e.Stderr))
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *ProcessError) Unwrap() error {
	return e.Cause
}

func (e *ProcessError) Is(target error) bool {
	_, ok := target.(*ProcessError)
	return ok
}

// CLIJSONDecodeError 表示底层 CLI 输出了无法解析的 JSON 消息。
type CLIJSONDecodeError = protocol.JSONDecodeError

// MessageParseError 表示底层消息结构无法映射到 SDK 协议模型。
type MessageParseError = protocol.MessageParseError

// UnsupportedCapabilityError 表示某个运行时能力在当前后端不可用。
type UnsupportedCapabilityError struct {
	Capability Capability
}

func (e *UnsupportedCapabilityError) Error() string {
	if e == nil || e.Capability == "" {
		return ErrUnsupportedCapability.Error()
	}
	return fmt.Sprintf("%s: %s", ErrUnsupportedCapability.Error(), e.Capability)
}

func (e *UnsupportedCapabilityError) Is(target error) bool {
	return target == ErrUnsupportedCapability
}

// StreamClosedBeforeTerminalError 表示消息流在收到 terminal result 前关闭。
type StreamClosedBeforeTerminalError struct {
	LastMessageID   string
	LastMessageType string
	LastStreamStop  StreamStopDiagnostics
	SessionID       string
	Cause           error
}

func (e *StreamClosedBeforeTerminalError) Error() string {
	message := ErrNoResult.Error()
	if e == nil {
		return message
	}
	if e.LastMessageType != "" {
		message += "; last_message_type=" + e.LastMessageType
	}
	if e.LastMessageID != "" {
		message += "; last_message_id=" + e.LastMessageID
	}
	if e.SessionID != "" {
		message += "; session_id=" + e.SessionID
	}
	message = appendStreamStopErrorDetail(message, e.LastStreamStop)
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *StreamClosedBeforeTerminalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *StreamClosedBeforeTerminalError) Is(target error) bool {
	return target == ErrNoResult
}

// NewCLIJSONDecodeError 创建 CLI JSON 解析错误。
func NewCLIJSONDecodeError(message string, raw string, cause error) *CLIJSONDecodeError {
	return protocol.NewJSONDecodeErrorWithCause(message, raw, cause)
}

func abortError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAborted) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %w", ErrAborted, err)
	}
	return err
}

func classifyTransportStartError(options Options, err error) error {
	if err == nil {
		return nil
	}
	var notFound *CLINotFoundError
	if errors.As(err, &notFound) {
		return err
	}
	if isExecNotFound(err) {
		return &CLINotFoundError{
			Command: cliCommandName(options, err),
			Cause:   err,
		}
	}
	var connection *CLIConnectionError
	if errors.As(err, &connection) {
		return err
	}
	return &CLIConnectionError{
		Message: "client: start sdk transport failed",
		Cause:   err,
	}
}

func classifyProcessExitError(err error) error {
	if err == nil {
		return nil
	}
	var processErr *ProcessError
	if errors.As(err, &processErr) {
		return err
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	return &ProcessError{
		ExitCode: exitErr.ExitCode(),
		Stderr:   string(exitErr.Stderr),
		Cause:    err,
	}
}

func withLastErrorResult(err error, text string) error {
	if err == nil {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return err
	}

	classified := classifyProcessExitError(err)
	var processErr *ProcessError
	if !errors.As(classified, &processErr) {
		return classified
	}
	return &ProcessError{
		Message:  "cli process returned an error result: " + text,
		ExitCode: processErr.ExitCode,
		Stderr:   processErr.Stderr,
		Cause:    processErr.Cause,
	}
}

func isExecNotFound(err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	var execErr *exec.Error
	return errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound)
}

func cliCommandName(options Options, cause error) string {
	command := strings.TrimSpace(options.CLIPath)
	if command == "" {
		var execErr *exec.Error
		if errors.As(cause, &execErr) && strings.TrimSpace(execErr.Name) != "" {
			return strings.TrimSpace(execErr.Name)
		}
		return "cli"
	}
	return command
}
