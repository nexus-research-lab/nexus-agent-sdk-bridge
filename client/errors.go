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

// BackendNotFoundError 表示后端可执行文件未找到。
type BackendNotFoundError struct {
	Command string
	Cause   error
}

func (e *BackendNotFoundError) Error() string {
	command := strings.TrimSpace(e.Command)
	if command == "" {
		command = "backend"
	}
	message := fmt.Sprintf("client: backend executable %q not found", command)
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *BackendNotFoundError) Unwrap() error {
	return e.Cause
}

func (e *BackendNotFoundError) Is(target error) bool {
	_, ok := target.(*BackendNotFoundError)
	return ok
}

// BackendConnectionError 表示 SDK 与底层后端传输连接失败。
type BackendConnectionError struct {
	Message string
	Cause   error
}

func (e *BackendConnectionError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "client: backend connection failed"
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *BackendConnectionError) Unwrap() error {
	return e.Cause
}

func (e *BackendConnectionError) Is(target error) bool {
	_, ok := target.(*BackendConnectionError)
	return ok
}

// BackendProcessError 表示底层进程后端异常退出。
type BackendProcessError struct {
	Message  string
	Stderr   string
	ExitCode int
	Cause    error
}

func (e *BackendProcessError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "client: backend process exited with error"
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

func (e *BackendProcessError) Unwrap() error {
	return e.Cause
}

func (e *BackendProcessError) Is(target error) bool {
	_, ok := target.(*BackendProcessError)
	return ok
}

// BackendJSONDecodeError 表示底层后端输出了无法解析的 JSON 消息。
type BackendJSONDecodeError = protocol.JSONDecodeError

// MessageParseError 表示底层消息结构无法映射到 SDK 协议模型。
type MessageParseError = protocol.MessageParseError

// NewBackendJSONDecodeError 创建后端 JSON 解析错误。
func NewBackendJSONDecodeError(message string, raw string, cause error) *BackendJSONDecodeError {
	return protocol.NewJSONDecodeErrorWithCause(message, raw, cause)
}

func classifyTransportStartError(options Options, err error) error {
	if err == nil {
		return nil
	}
	var notFound *BackendNotFoundError
	if errors.As(err, &notFound) {
		return err
	}
	if isExecNotFound(err) {
		return &BackendNotFoundError{
			Command: backendCommandName(options, err),
			Cause:   err,
		}
	}
	var connection *BackendConnectionError
	if errors.As(err, &connection) {
		return err
	}
	return &BackendConnectionError{
		Message: "client: start sdk transport failed",
		Cause:   err,
	}
}

func classifyProcessExitError(err error) error {
	if err == nil {
		return nil
	}
	var processErr *BackendProcessError
	if errors.As(err, &processErr) {
		return err
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	return &BackendProcessError{
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
	var processErr *BackendProcessError
	if !errors.As(classified, &processErr) {
		return classified
	}
	return &BackendProcessError{
		Message:  "process backend returned an error result: " + text,
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

func backendCommandName(options Options, cause error) string {
	command := strings.TrimSpace(options.commandPath)
	if command == "" {
		var execErr *exec.Error
		if errors.As(cause, &execErr) && strings.TrimSpace(execErr.Name) != "" {
			return strings.TrimSpace(execErr.Name)
		}
		return "process backend"
	}
	return command
}
