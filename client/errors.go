package client

import (
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
	// ErrBypassPermissionsNotAllowed 表示会话启动时没有允许运行期切换到 bypassPermissions。
	ErrBypassPermissionsNotAllowed = errors.New("client: bypassPermissions requires allowDangerouslySkipPermissions at session launch")
)

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

// NewCLIJSONDecodeError 创建 CLI JSON 解析错误。
func NewCLIJSONDecodeError(message string, raw string, cause error) *CLIJSONDecodeError {
	return protocol.NewJSONDecodeErrorWithCause(message, raw, cause)
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
