package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const defaultDirectConnectDialTimeout = 15 * time.Second

// DirectConnectEndpoint 表示解析后的 direct-connect 终点。
type DirectConnectEndpoint struct {
	ServerURL string
	AuthToken string
}

// DirectConnectConfig 表示 direct-connect 传输配置。
type DirectConnectConfig struct {
	ServerURL                       string
	AuthToken                       string
	SessionKey                      string
	CWD                             string
	PermissionMode                  string
	AllowDangerouslySkipPermissions bool
	DeleteSessionOnClose            bool
	HTTPClient                      *http.Client
	DialTimeout                     time.Duration
}

// DirectConnectError 表示 direct-connect 传输错误。
type DirectConnectError struct {
	message string
}

func (e *DirectConnectError) Error() string {
	if e == nil {
		return "direct-connect: error"
	}
	message := strings.TrimSpace(e.message)
	if message == "" {
		return "direct-connect: error"
	}
	if strings.HasPrefix(message, "direct-connect:") {
		return message
	}
	return "direct-connect: " + message
}

type directConnectSessionResponse struct {
	SessionID string `json:"session_id"`
	WSURL     string `json:"ws_url"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// DirectConnectManager 管理 direct-connect 传输。
type DirectConnectManager struct {
	config DirectConnectConfig

	client *http.Client
	conn   *websocket.Conn

	stateMu      sync.RWMutex
	sessionID    string
	workDir      string
	waitErr      error
	closing      bool
	started      bool
	deleteCalled bool

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc

	readQueue      chan map[string]any
	closeOnce      sync.Once
	closeQueueOnce sync.Once
	doneOnce       sync.Once
	done           chan struct{}
}

// NewDirectConnectManager 创建 direct-connect 管理器。
func NewDirectConnectManager(config DirectConnectConfig) *DirectConnectManager {
	return &DirectConnectManager{
		config:    config,
		readQueue: make(chan map[string]any, 64),
		done:      make(chan struct{}),
	}
}

// ParseDirectConnectURL 解析官方 TS SDK 使用的 direct-connect 地址格式。
func ParseDirectConnectURL(raw string) (DirectConnectEndpoint, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DirectConnectEndpoint{}, errors.New("direct-connect: url is empty")
	}
	if strings.HasPrefix(value, "cc+unix://") {
		return DirectConnectEndpoint{}, &DirectConnectError{
			message: "unix socket connect (cc+unix://) is not supported by the SDK transport",
		}
	}
	if strings.HasPrefix(value, "cc://") {
		parsed, err := url.Parse("http://" + strings.TrimPrefix(value, "cc://"))
		if err != nil {
			return DirectConnectEndpoint{}, fmt.Errorf("direct-connect: parse url failed: %w", err)
		}
		if parsed.Host == "" {
			return DirectConnectEndpoint{}, errors.New("direct-connect: host is empty")
		}
		host := normalizeDirectConnectHost(parsed.Host)
		if host == "" {
			return DirectConnectEndpoint{}, errors.New("direct-connect: host is empty")
		}
		return DirectConnectEndpoint{
			ServerURL: (&url.URL{Scheme: "http", Host: host}).String(),
			AuthToken: strings.TrimPrefix(parsed.Path, "/"),
		}, nil
	}

	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = "http://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return DirectConnectEndpoint{}, fmt.Errorf("direct-connect: parse url failed: %w", err)
	}
	if parsed.Host == "" {
		return DirectConnectEndpoint{}, errors.New("direct-connect: host is empty")
	}
	host := normalizeDirectConnectHost(parsed.Host)
	if host == "" {
		return DirectConnectEndpoint{}, errors.New("direct-connect: host is empty")
	}

	return DirectConnectEndpoint{
		ServerURL: (&url.URL{Scheme: parsed.Scheme, Host: host}).String(),
	}, nil
}

func normalizeDirectConnectHost(host string) string {
	// TypeScript 的 WHATWG URL 解析器会接受 "http://ws://localhost:3000"
	// 这类输入，并把 host 解析成 "ws"。
	return strings.TrimSuffix(strings.TrimSpace(host), ":")
}

// Start 建立 HTTP 会话并升级为 WebSocket。
func (m *DirectConnectManager) Start(ctx context.Context) error {
	var startErr error

	m.stateMu.Lock()
	if m.started {
		m.stateMu.Unlock()
		return nil
	}
	m.started = true
	m.stateMu.Unlock()
	defer func() {
		if startErr == nil {
			return
		}
		m.stateMu.Lock()
		m.started = false
		m.stateMu.Unlock()
	}()

	if strings.TrimSpace(m.config.ServerURL) == "" {
		startErr = errors.New("direct-connect: server url is empty")
		return startErr
	}

	m.client = m.config.HTTPClient
	if m.client == nil {
		m.client = http.DefaultClient
	}

	session, err := m.createSession(ctx)
	if err != nil {
		startErr = err
		return startErr
	}

	m.stateMu.Lock()
	m.sessionID = session.SessionID
	m.workDir = session.WorkDir
	m.stateMu.Unlock()

	dialTimeout := m.config.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultDirectConnectDialTimeout
	}

	dialContext, dialCancel := context.WithTimeout(ctx, dialTimeout)
	defer dialCancel()

	headers := http.Header{}
	if m.config.AuthToken != "" {
		headers.Set("Authorization", "Bearer "+m.config.AuthToken)
	}

	conn, _, err := websocket.Dial(dialContext, session.WSURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		if m.config.DeleteSessionOnClose {
			_ = m.deleteSession(context.Background())
		}
		startErr = &DirectConnectError{
			message: fmt.Sprintf("failed to connect websocket: %v", err),
		}
		return startErr
	}

	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())

	m.stateMu.Lock()
	m.conn = conn
	m.lifecycleCtx = lifecycleCtx
	m.lifecycleCancel = lifecycleCancel
	m.stateMu.Unlock()

	go m.readLoop()
	return nil
}

// ReadJSON 读取下一条 JSON 消息。
func (m *DirectConnectManager) ReadJSON() (map[string]any, error) {
	message, ok := <-m.readQueue
	if ok {
		return message, nil
	}

	if err := m.getWaitError(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return nil, io.EOF
}

// WriteJSON 写入一条 JSON 消息。
func (m *DirectConnectManager) WriteJSON(payload any) error {
	conn := m.currentConn()
	if conn == nil {
		return errors.New("direct-connect: websocket unavailable")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("direct-connect: marshal payload failed: %w", err)
	}

	writeContext := m.currentContext()
	if writeContext == nil {
		writeContext = context.Background()
	}

	if err := conn.Write(writeContext, websocket.MessageText, append(data, '\n')); err != nil {
		normalized := m.normalizeSocketError(err)
		m.finish(normalized)
		return fmt.Errorf("direct-connect: write payload failed: %w", normalized)
	}
	return nil
}

// EndInput 对 direct-connect 是空操作。
func (m *DirectConnectManager) EndInput() error {
	return nil
}

// Interrupt 对 direct-connect 不能直接操作底层执行，交给 client 回退 control interrupt。
func (m *DirectConnectManager) Interrupt() error {
	return ErrInterruptUnsupported
}

// Wait 等待传输结束。
func (m *DirectConnectManager) Wait() error {
	<-m.done
	return normalizeDirectConnectWaitError(m.getWaitError())
}

// Close 主动关闭 direct-connect 连接。
func (m *DirectConnectManager) Close() error {
	var closeErr error

	m.closeOnce.Do(func() {
		m.stateMu.Lock()
		m.closing = true
		conn := m.conn
		cancel := m.lifecycleCancel
		m.stateMu.Unlock()

		if cancel != nil {
			cancel()
		}
		if conn != nil {
			closeErr = normalizeDirectConnectCloseError(conn.Close(websocket.StatusNormalClosure, "Normal closure"))
		}
		if m.config.DeleteSessionOnClose {
			closeErr = errors.Join(closeErr, m.deleteSession(context.Background()))
		}

		if conn == nil {
			m.finishAndCloseQueue(io.EOF)
		}
	})

	return normalizeDirectConnectWaitError(closeErr)
}

func (m *DirectConnectManager) createSession(ctx context.Context) (directConnectSessionResponse, error) {
	payload := map[string]any{}
	if m.config.CWD != "" {
		payload["cwd"] = m.config.CWD
	}
	if m.config.SessionKey != "" {
		payload["session_key"] = m.config.SessionKey
	}
	if m.config.PermissionMode != "" {
		payload["permission_mode"] = m.config.PermissionMode
	}
	if m.config.AllowDangerouslySkipPermissions {
		payload["allow_dangerously_skip_permissions"] = true
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return directConnectSessionResponse{}, fmt.Errorf("direct-connect: marshal create session payload failed: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(m.config.ServerURL, "/")+"/sessions",
		bytes.NewReader(data),
	)
	if err != nil {
		return directConnectSessionResponse{}, fmt.Errorf("direct-connect: build create session request failed: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if m.config.AuthToken != "" {
		request.Header.Set("Authorization", "Bearer "+m.config.AuthToken)
	}

	response, err := m.client.Do(request)
	if err != nil {
		return directConnectSessionResponse{}, &DirectConnectError{
			message: fmt.Sprintf("failed to connect to server at %s: %v", m.config.ServerURL, err),
		}
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(response.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = response.Status
		}
		return directConnectSessionResponse{}, &DirectConnectError{
			message: fmt.Sprintf("failed to create session: %s", message),
		}
	}

	var result directConnectSessionResponse
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&result); err != nil {
		return directConnectSessionResponse{}, &DirectConnectError{
			message: fmt.Sprintf("invalid session response: %v", err),
		}
	}
	if result.SessionID == "" || result.WSURL == "" {
		return directConnectSessionResponse{}, &DirectConnectError{
			message: "invalid session response: missing session_id or ws_url",
		}
	}
	return result, nil
}

func (m *DirectConnectManager) deleteSession(ctx context.Context) error {
	m.stateMu.Lock()
	if m.deleteCalled || m.sessionID == "" {
		m.stateMu.Unlock()
		return nil
	}
	m.deleteCalled = true
	sessionID := m.sessionID
	m.stateMu.Unlock()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		strings.TrimRight(m.config.ServerURL, "/")+"/sessions/"+sessionID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("direct-connect: build delete session request failed: %w", err)
	}
	if m.config.AuthToken != "" {
		request.Header.Set("Authorization", "Bearer "+m.config.AuthToken)
	}

	response, err := m.client.Do(request)
	if err != nil {
		return fmt.Errorf("direct-connect: delete session failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("direct-connect: delete session failed with status %s", response.Status)
	}
	return nil
}

func (m *DirectConnectManager) readLoop() {
	contextValue := m.currentContext()
	if contextValue == nil {
		contextValue = context.Background()
	}

	var partial []byte
	for {
		conn := m.currentConn()
		if conn == nil {
			m.finishAndCloseQueue(io.EOF)
			return
		}

		_, data, err := conn.Read(contextValue)
		if err != nil {
			m.finishAndCloseQueue(m.normalizeSocketError(err))
			return
		}

		combined := make([]byte, 0, len(partial)+len(data))
		combined = append(combined, partial...)
		combined = append(combined, data...)

		lines := bytes.Split(combined, []byte{'\n'})
		partial = partial[:0]
		if len(combined) > 0 && combined[len(combined)-1] != '\n' {
			partial = append(partial, lines[len(lines)-1]...)
			lines = lines[:len(lines)-1]
		}

		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			payload, decodeErr := decodeJSONLine(line)
			if decodeErr != nil {
				continue
			}

			select {
			case m.readQueue <- payload:
			case <-contextValue.Done():
				m.finishAndCloseQueue(io.EOF)
				return
			}
		}
	}
}

func (m *DirectConnectManager) currentConn() *websocket.Conn {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.conn
}

func (m *DirectConnectManager) currentContext() context.Context {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.lifecycleCtx
}

func (m *DirectConnectManager) getWaitError() error {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.waitErr
}

func (m *DirectConnectManager) finish(err error) {
	m.stateMu.Lock()
	if m.waitErr == nil {
		m.waitErr = err
	}
	m.stateMu.Unlock()

	m.doneOnce.Do(func() {
		close(m.done)
	})
}

func (m *DirectConnectManager) finishAndCloseQueue(err error) {
	m.finish(err)
	m.closeQueueOnce.Do(func() {
		close(m.readQueue)
	})
}

func (m *DirectConnectManager) normalizeSocketError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return io.EOF
	}

	m.stateMu.RLock()
	closing := m.closing
	m.stateMu.RUnlock()
	if closing {
		return io.EOF
	}

	return &DirectConnectError{
		message: err.Error(),
	}
}

func normalizeDirectConnectWaitError(err error) error {
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func normalizeDirectConnectCloseError(err error) error {
	if err == nil || errors.Is(err, net.ErrClosed) {
		return nil
	}
	if strings.Contains(err.Error(), "use of closed network connection") {
		return nil
	}
	return err
}

func decodeJSONLine(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}
