package protocol

import "fmt"

// JSONDecodeError 表示 JSON 反序列化失败。
type JSONDecodeError struct {
	Message string
	Raw     string
	Cause   error
}

// Error 返回错误文本。
func (e *JSONDecodeError) Error() string {
	message := e.Message
	if e.Raw != "" {
		rawSnippet := e.Raw
		if len(rawSnippet) > 160 {
			rawSnippet = rawSnippet[:160] + "..."
		}
		message = fmt.Sprintf("%s (raw: %s)", message, rawSnippet)
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

// Unwrap 返回底层错误。
func (e *JSONDecodeError) Unwrap() error {
	return e.Cause
}

// Is 用于 errors.Is 比较。
func (e *JSONDecodeError) Is(target error) bool {
	_, ok := target.(*JSONDecodeError)
	return ok
}

// NewJSONDecodeErrorWithCause 创建 JSON 反序列化错误。
func NewJSONDecodeErrorWithCause(message string, raw string, cause error) *JSONDecodeError {
	return &JSONDecodeError{
		Message: message,
		Raw:     raw,
		Cause:   cause,
	}
}

// MessageParseError 表示消息结构解析失败。
type MessageParseError struct {
	Message     string
	MessageType string
	Cause       error
}

// Error 返回错误文本。
func (e *MessageParseError) Error() string {
	message := e.Message
	if e.MessageType != "" {
		message = fmt.Sprintf("%s (type: %s)", message, e.MessageType)
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

// Unwrap 返回底层错误。
func (e *MessageParseError) Unwrap() error {
	return e.Cause
}

// Is 用于 errors.Is 比较。
func (e *MessageParseError) Is(target error) bool {
	_, ok := target.(*MessageParseError)
	return ok
}

// NewMessageParseError 创建消息解析错误。
func NewMessageParseError(message string) *MessageParseError {
	return &MessageParseError{Message: message}
}

// NewMessageParseErrorWithType 创建带消息类型的解析错误。
func NewMessageParseErrorWithType(message string, messageType string) *MessageParseError {
	return &MessageParseError{
		Message:     message,
		MessageType: messageType,
	}
}

// NewMessageParseErrorWithCause 创建带底层错误的消息解析错误。
func NewMessageParseErrorWithCause(message string, messageType string, cause error) *MessageParseError {
	return &MessageParseError{
		Message:     message,
		MessageType: messageType,
		Cause:       cause,
	}
}
