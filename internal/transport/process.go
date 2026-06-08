package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultCloseTimeout = 5 * time.Second
const defaultStderrDrainTimeout = 100 * time.Millisecond
const defaultMaxBufferSize = 1024 * 1024
const diagnosticTextLimit = 2048
const diagnosticTailLimit = 4096
const minimumCommandVersion = "2.0.0"
const claudeCommandPathEnvName = "NEXUS_CLAUDE_COMMAND_PATH"
const skipVersionCheckEnv = "CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK"
const versionCheckTimeout = 2 * time.Second

var commandVersionPattern = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+)`)

type processCommandResolver struct {
	goos       string
	getenv     func(string) string
	lookPath   func(string) (string, error)
	fileExists func(string) bool
}

// ProcessDiagnosticEvent 表示 process bridge 产生的一条诊断事件。
type ProcessDiagnosticEvent struct {
	Component  string
	Event      string
	Attributes map[string]any
}

// StdoutDecodeError 携带 stdout JSON 解码失败时的现场信息。
type StdoutDecodeError struct {
	Err           error
	StdoutBytes   int
	StdoutPrefix  string
	StdoutSuffix  string
	ProcessExited bool
	ProcessError  string
	StderrTail    string
}

func (e *StdoutDecodeError) Error() string {
	if e == nil {
		return "process: decode stdout JSON message failed"
	}
	detail := fmt.Sprintf(
		"process: decode stdout JSON message failed: %v stdout_bytes=%d stdout_prefix=%q stdout_suffix=%q",
		e.Err,
		e.StdoutBytes,
		e.StdoutPrefix,
		e.StdoutSuffix,
	)
	if e.ProcessExited {
		detail += " process_exited=true"
	}
	if strings.TrimSpace(e.ProcessError) != "" {
		detail += " process_error=" + strings.TrimSpace(e.ProcessError)
	}
	if strings.TrimSpace(e.StderrTail) != "" {
		detail += " stderr_tail=" + strings.TrimSpace(e.StderrTail)
	}
	return detail
}

func (e *StdoutDecodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ProcessConfig 表示子进程传输配置。
type ProcessConfig struct {
	CommandPath        string
	CWD                string
	User               string
	MaxBufferSize      int
	Args               []string
	Env                map[string]string
	Stderr             func(string)
	Diagnostics        func(ProcessDiagnosticEvent)
	ControlWireDialect ControlWireDialect
}

// ProcessManager 管理 Claude CLI 子进程。
type ProcessManager struct {
	config        ProcessConfig
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stdoutWriter  *os.File
	stderr        io.ReadCloser
	stderrWriter  *os.File
	reader        *bufio.Reader
	writeMu       sync.Mutex
	closeOnce     sync.Once
	done          chan struct{}
	waitErr       error
	waitMu        sync.Mutex
	stderrWG      sync.WaitGroup
	maxBufferSize int
	stderrTail    diagnosticTail
}

// NewProcessManager 创建进程管理器。
func NewProcessManager(config ProcessConfig) *ProcessManager {
	return &ProcessManager{
		config:     config,
		done:       make(chan struct{}),
		stderrTail: diagnosticTail{limit: diagnosticTailLimit},
	}
}

// Start 启动子进程。
func (m *ProcessManager) Start(ctx context.Context) error {
	if m.cmd != nil {
		return nil
	}

	commandPath, err := resolveCommandPath(m.config.CommandPath)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	m.checkCommandVersion(ctx, commandPath)

	cmd := exec.Command(commandPath, m.config.Args...)
	cmd.Dir = m.config.CWD
	cmd.Env = buildEnvironment(m.config.Env, m.config.CWD)
	if err := applyCommandUser(cmd, m.config.User); err != nil {
		return err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("process: create stdin pipe failed: %w", err)
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("process: create stdout pipe failed: %w", err)
	}
	cmd.Stdout = stdoutWriter

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return fmt.Errorf("process: create stderr pipe failed: %w", err)
	}
	cmd.Stderr = stderrWriter

	if err := cmd.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		return fmt.Errorf("process: start command failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	m.emitDiagnostic("process_start", map[string]any{
		"command_path": commandPath,
		"cwd":          m.config.CWD,
		"pid":          cmd.Process.Pid,
		"args":         append([]string(nil), m.config.Args...),
	})

	if err := stdoutWriter.Close(); err != nil {
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("process: close parent stdout writer failed: %w", err)
	}
	if err := stderrWriter.Close(); err != nil {
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("process: close parent stderr writer failed: %w", err)
	}

	m.cmd = cmd
	m.stdin = stdin
	m.stdout = stdoutReader
	m.stdoutWriter = stdoutWriter
	m.reader = bufio.NewReader(stdoutReader)
	m.maxBufferSize = m.config.MaxBufferSize
	if m.maxBufferSize <= 0 {
		m.maxBufferSize = defaultMaxBufferSize
	}
	m.stderr = stderrReader
	m.stderrWriter = stderrWriter

	m.stderrWG.Add(1)
	go m.readStderr(stderrReader)

	go func() {
		defer close(m.done)
		err := cmd.Wait()
		m.setWaitError(err)
		attributes := map[string]any{"pid": cmd.Process.Pid}
		if normalizedErr := normalizeExitError(err); normalizedErr != nil {
			attributes["error"] = normalizedErr.Error()
		}
		m.emitDiagnostic("process_exit", attributes)
	}()

	return nil
}

// ReadJSON 读取下一条 JSON 消息。
func (m *ProcessManager) ReadJSON() (map[string]any, error) {
	if m.reader == nil {
		return nil, errors.New("process: manager not started")
	}

	for {
		line, err := readJSONLine(m.reader, m.maxBufferSize)
		if err != nil {
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// 中文注释：CLI 在某些平台会把调试输出写到 stdout，这些非 JSON 行需要跳过，
		// 否则会污染协议流并导致后续真实消息无法解析。
		if line[0] != '{' {
			continue
		}

		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.UseNumber()

		var payload map[string]any
		if err := decoder.Decode(&payload); err != nil {
			return nil, m.newStdoutDecodeError(err, line)
		}
		return payload, nil
	}
}

// WriteJSON 写入一条 JSON 消息。
func (m *ProcessManager) WriteJSON(payload any) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	if m.stdin == nil {
		return errors.New("process: stdin unavailable")
	}
	if processExited, processError := m.processExitSnapshot(); processExited {
		if strings.TrimSpace(processError) != "" {
			return fmt.Errorf("process: cannot write to exited process: %s", strings.TrimSpace(processError))
		}
		return errors.New("process: cannot write to exited process")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("process: marshal payload failed: %w", err)
	}

	if _, err := m.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("process: write payload failed: %w", err)
	}
	return nil
}

// EndInput 主动关闭 stdin。
func (m *ProcessManager) EndInput() error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	if m.stdin == nil {
		return nil
	}

	if err := m.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return fmt.Errorf("process: close stdin failed: %w", err)
	}
	m.stdin = nil
	return nil
}

// Interrupt 发送中断信号。
func (m *ProcessManager) Interrupt() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	return m.cmd.Process.Signal(os.Interrupt)
}

func terminateProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}
	return process.Signal(syscall.SIGTERM)
}

func applyCommandUser(cmd *exec.Cmd, userName string) error {
	if strings.TrimSpace(userName) == "" {
		return nil
	}
	if !commandCredentialSupported() {
		return fmt.Errorf("process: setting subprocess user is not supported on this platform")
	}

	resolvedUser, err := user.Lookup(userName)
	if err != nil {
		return fmt.Errorf("process: lookup user %q failed: %w", userName, err)
	}

	uid, err := strconv.ParseUint(resolvedUser.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("process: parse user %q uid failed: %w", userName, err)
	}
	gid, err := strconv.ParseUint(resolvedUser.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("process: parse user %q gid failed: %w", userName, err)
	}

	return setCommandCredential(cmd, uid, gid)
}

func commandCredentialSupported() bool {
	credentialField, ok := commandCredentialField(&syscall.SysProcAttr{})
	return ok && credentialField.Kind() == reflect.Pointer && credentialField.Type().Elem().Kind() == reflect.Struct
}

func setCommandCredential(cmd *exec.Cmd, uid uint64, gid uint64) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	credentialField, ok := commandCredentialField(cmd.SysProcAttr)
	if !ok {
		return fmt.Errorf("process: setting subprocess user is not supported on this platform")
	}

	credential := reflect.New(credentialField.Type().Elem())
	if !setUnsignedStructField(credential.Elem(), "Uid", uid) {
		return fmt.Errorf("process: setting subprocess uid is not supported on this platform")
	}
	if !setUnsignedStructField(credential.Elem(), "Gid", gid) {
		return fmt.Errorf("process: setting subprocess gid is not supported on this platform")
	}
	credentialField.Set(credential)
	return nil
}

func commandCredentialField(attributes *syscall.SysProcAttr) (reflect.Value, bool) {
	if attributes == nil {
		return reflect.Value{}, false
	}
	value := reflect.ValueOf(attributes).Elem()
	field := value.FieldByName("Credential")
	if !field.IsValid() || !field.CanSet() {
		return reflect.Value{}, false
	}
	return field, true
}

func setUnsignedStructField(value reflect.Value, name string, number uint64) bool {
	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanSet() {
		return false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if field.OverflowUint(number) {
			return false
		}
		field.SetUint(number)
		return true
	default:
		return false
	}
}

func resolveCommandPath(commandPath string) (string, error) {
	return resolveCommandPathWith(commandPath, defaultProcessCommandResolver())
}

func defaultProcessCommandResolver() processCommandResolver {
	return processCommandResolver{
		goos:     runtime.GOOS,
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		fileExists: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
	}
}

func resolveCommandPathWith(commandPath string, resolver processCommandResolver) (string, error) {
	commandPath = strings.TrimSpace(commandPath)
	if commandPath != "" {
		return commandPath, nil
	}

	resolver = normalizeProcessCommandResolver(resolver)
	if override := strings.TrimSpace(resolver.getenv(claudeCommandPathEnvName)); override != "" {
		return override, nil
	}
	names := commandNames(resolver.goos)
	for _, name := range names {
		if path, err := resolver.lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path, nil
		}
	}
	for _, candidate := range knownCommandPaths(resolver.goos, resolver.getenv) {
		if resolver.fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("process: cli executable %q not found: %w", strings.Join(names, "|"), exec.ErrNotFound)
}

func normalizeProcessCommandResolver(resolver processCommandResolver) processCommandResolver {
	if strings.TrimSpace(resolver.goos) == "" {
		resolver.goos = runtime.GOOS
	}
	if resolver.getenv == nil {
		resolver.getenv = os.Getenv
	}
	if resolver.lookPath == nil {
		resolver.lookPath = exec.LookPath
	}
	if resolver.fileExists == nil {
		resolver.fileExists = func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		}
	}
	return resolver
}

func commandNames(goos string) []string {
	if goos == "windows" {
		// Windows 的 npm 全局安装通常只暴露 claude.cmd/claude.ps1。
		return []string{"claude.exe", "claude.cmd", "claude.ps1", "claude"}
	}
	return []string{"claude"}
}

func knownCommandPaths(goos string, getenv func(string) string) []string {
	if goos == "windows" {
		return knownWindowsCommandPaths(getenv)
	}

	home := strings.TrimSpace(getenv("HOME"))
	if home == "" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			home = homeDir
		}
	}
	name := commandNames(goos)[0]
	paths := make([]string, 0, 6)
	if home != "" {
		paths = append(paths, filepath.Join(home, ".npm-global", "bin", name))
	}
	paths = append(paths,
		filepath.Join(string(filepath.Separator), "usr", "local", "bin", name),
	)
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".local", "bin", name),
			filepath.Join(home, "node_modules", ".bin", name),
			filepath.Join(home, ".yarn", "bin", name),
			filepath.Join(home, ".claude", "local", name),
		)
	}
	return compactCommandPaths(paths)
}

func knownWindowsCommandPaths(getenv func(string) string) []string {
	paths := []string{}
	if appData := strings.TrimSpace(getenv("APPDATA")); appData != "" {
		paths = appendCommandNames(paths, filepath.Join(appData, "npm"), "windows")
	}
	if userProfile := strings.TrimSpace(getenv("USERPROFILE")); userProfile != "" {
		paths = appendCommandNames(paths, filepath.Join(userProfile, ".local", "bin"), "windows")
		paths = appendCommandNames(paths, filepath.Join(userProfile, ".claude", "local"), "windows")
		paths = appendCommandNames(paths, filepath.Join(userProfile, "node_modules", ".bin"), "windows")
	}
	return compactCommandPaths(paths)
}

func appendCommandNames(paths []string, dir string, goos string) []string {
	for _, name := range commandNames(goos) {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

func compactCommandPaths(paths []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized := strings.TrimSpace(path)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(normalized))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func (m *ProcessManager) checkCommandVersion(parent context.Context, commandPath string) {
	if versionCheckSkipped() {
		return
	}
	ctx, cancel := context.WithTimeout(parent, versionCheckTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, commandPath, "-v").Output()
	if err != nil {
		return
	}
	m.emitUnsupportedCommandVersionDiagnostic(commandPath, string(output))
}

func (m *ProcessManager) emitUnsupportedCommandVersionDiagnostic(commandPath, output string) {
	version, ok := parseCommandVersion(output)
	if !ok || compareSemanticVersion(version, minimumCommandVersion) >= 0 {
		return
	}
	m.emitDiagnostic("cli_version_unsupported", map[string]any{
		"command_path":     commandPath,
		"version":          version,
		"minimum_version":  minimumCommandVersion,
		"version_check_ms": versionCheckTimeout.Milliseconds(),
	})
}

func versionCheckSkipped() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(skipVersionCheckEnv)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseCommandVersion(output string) (string, bool) {
	output = strings.TrimSpace(output)
	match := commandVersionPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func compareSemanticVersion(left string, right string) int {
	leftParts := semanticVersionParts(left)
	rightParts := semanticVersionParts(right)
	for index := 0; index < 3; index++ {
		if leftParts[index] < rightParts[index] {
			return -1
		}
		if leftParts[index] > rightParts[index] {
			return 1
		}
	}
	return 0
}

func semanticVersionParts(version string) [3]int {
	var parts [3]int
	for index, segment := range strings.Split(version, ".") {
		if index >= len(parts) {
			break
		}
		value, err := strconv.Atoi(segment)
		if err != nil {
			continue
		}
		parts[index] = value
	}
	return parts
}

// Wait 等待进程结束。
func (m *ProcessManager) Wait() error {
	if m.cmd == nil {
		return nil
	}

	<-m.done
	m.closeOutputPipes()
	m.waitForStderrReader()
	return normalizeExitError(m.waitError())
}

// Close 关闭进程。
func (m *ProcessManager) Close() error {
	var closeErr error
	m.closeOnce.Do(func() {
		_ = m.EndInput()

		if m.cmd == nil || m.cmd.Process == nil {
			return
		}

		forcedExit := false
		if !m.waitForDone(defaultCloseTimeout) {
			m.emitDiagnostic("process_close_timeout_terminate", map[string]any{
				"pid":        m.cmd.Process.Pid,
				"timeout_ms": defaultCloseTimeout.Milliseconds(),
			})
			forcedExit = true
			if err := terminateProcess(m.cmd.Process); err != nil {
				m.emitDiagnostic("process_terminate_error", map[string]any{
					"pid":   m.cmd.Process.Pid,
					"error": err.Error(),
				})
			}
			if !m.waitForDone(defaultCloseTimeout) {
				m.emitDiagnostic("process_terminate_timeout_kill", map[string]any{
					"pid":        m.cmd.Process.Pid,
					"timeout_ms": defaultCloseTimeout.Milliseconds(),
				})
				_ = m.cmd.Process.Kill()
				<-m.done
			}
		}

		m.closeOutputPipes()
		m.waitForStderrReader()
		if !forcedExit {
			closeErr = normalizeExitError(m.waitError())
		}
	})
	return closeErr
}

func (m *ProcessManager) closeOutputPipes() {
	if m.stdout != nil {
		_ = m.stdout.Close()
		m.stdout = nil
	}
	if m.stderr != nil {
		m.closeFileWithTimeout("stderr", m.stderr, defaultStderrDrainTimeout)
		m.stderr = nil
	}
}

func (m *ProcessManager) closeFileWithTimeout(name string, file io.Closer, timeout time.Duration) {
	if file == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = file.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		m.emitDiagnostic("pipe_close_timeout", map[string]any{
			"name":       name,
			"timeout_ms": timeout.Milliseconds(),
		})
	}
}

func (m *ProcessManager) waitForStderrReader() {
	done := make(chan struct{})
	go func() {
		m.stderrWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(defaultStderrDrainTimeout):
		m.emitDiagnostic("stderr_drain_timeout", map[string]any{
			"timeout_ms": defaultStderrDrainTimeout.Milliseconds(),
		})
	}
}

func (m *ProcessManager) waitForDone(timeout time.Duration) bool {
	select {
	case <-m.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (m *ProcessManager) readStderr(stderr io.Reader) {
	defer m.stderrWG.Done()
	if stderr == nil {
		return
	}

	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			m.stderrTail.Append(line)
			if m.config.Stderr != nil {
				m.config.Stderr(line)
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) || isClosedReadError(err) {
				return
			}
			m.emitDiagnostic("stderr_read_error", map[string]any{
				"error": err.Error(),
			})
			return
		}
	}
}

func (m *ProcessManager) setWaitError(err error) {
	m.waitMu.Lock()
	defer m.waitMu.Unlock()
	m.waitErr = err
}

func (m *ProcessManager) waitError() error {
	m.waitMu.Lock()
	defer m.waitMu.Unlock()
	return m.waitErr
}

func (m *ProcessManager) newStdoutDecodeError(err error, line []byte) error {
	processExited, processError := m.processExitSnapshot()
	diagnosticErr := &StdoutDecodeError{
		Err:           err,
		StdoutBytes:   len(line),
		StdoutPrefix:  diagnosticPreview(line, false),
		StdoutSuffix:  diagnosticPreview(line, true),
		ProcessExited: processExited,
		ProcessError:  processError,
		StderrTail:    m.stderrTail.String(),
	}
	m.emitDiagnostic("stdout_decode_error", map[string]any{
		"error":          err.Error(),
		"stdout_bytes":   diagnosticErr.StdoutBytes,
		"stdout_prefix":  diagnosticErr.StdoutPrefix,
		"stdout_suffix":  diagnosticErr.StdoutSuffix,
		"process_exited": diagnosticErr.ProcessExited,
		"process_error":  diagnosticErr.ProcessError,
		"stderr_tail":    diagnosticErr.StderrTail,
	})
	return diagnosticErr
}

func (m *ProcessManager) processExitSnapshot() (bool, string) {
	select {
	case <-m.done:
		if err := normalizeExitError(m.waitError()); err != nil {
			return true, err.Error()
		}
		return true, ""
	default:
		return false, ""
	}
}

func (m *ProcessManager) emitDiagnostic(event string, attributes map[string]any) {
	if m.config.Diagnostics == nil {
		return
	}
	copied := map[string]any{}
	for key, value := range attributes {
		if strings.TrimSpace(key) == "" {
			continue
		}
		copied[key] = value
	}
	m.config.Diagnostics(ProcessDiagnosticEvent{
		Component:  "bridge.process",
		Event:      event,
		Attributes: copied,
	})
}

func buildEnvironment(overrides map[string]string, cwd string) []string {
	environment := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == "CLAUDECODE" {
			continue
		}
		environment[parts[0]] = parts[1]
	}

	environment["CLAUDE_CODE_ENTRYPOINT"] = "sdk-go"
	environment["CLAUDE_AGENT_SDK_VERSION"] = "dev"
	if cwd != "" {
		environment["PWD"] = cwd
	}
	for key, value := range overrides {
		environment[key] = value
	}

	results := make([]string, 0, len(environment))
	for key, value := range environment {
		results = append(results, fmt.Sprintf("%s=%s", key, value))
	}
	return results
}

func normalizeExitError(err error) error {
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("process: command exited with error: %w", err)
	}
	return err
}

func readJSONLine(reader *bufio.Reader, maxBufferSize int) ([]byte, error) {
	buffer := make([]byte, 0, 256)
	for {
		if line, ok := consumeBufferedLine(reader, false); ok {
			if len(buffer)+len(line) > maxBufferSize {
				return nil, fmt.Errorf("process: JSON message exceeded maximum buffer size of %d bytes", maxBufferSize)
			}
			buffer = append(buffer, line...)
			return bytes.TrimSpace(buffer), nil
		}

		fragment, err := reader.ReadSlice('\n')
		if len(fragment) > 0 {
			if len(buffer)+len(fragment) > maxBufferSize {
				return nil, fmt.Errorf("process: JSON message exceeded maximum buffer size of %d bytes", maxBufferSize)
			}
			buffer = append(buffer, fragment...)
		}

		switch {
		case err == nil:
			return bytes.TrimSpace(buffer), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case isClosedReadError(err):
			if line, ok := consumeBufferedLine(reader, true); ok {
				if len(buffer)+len(line) > maxBufferSize {
					return nil, fmt.Errorf("process: JSON message exceeded maximum buffer size of %d bytes", maxBufferSize)
				}
				buffer = append(buffer, line...)
			}
			if len(buffer) == 0 {
				return nil, io.EOF
			}
			return bytes.TrimSpace(buffer), nil
		case errors.Is(err, io.EOF):
			if line, ok := consumeBufferedLine(reader, true); ok {
				if len(buffer)+len(line) > maxBufferSize {
					return nil, fmt.Errorf("process: JSON message exceeded maximum buffer size of %d bytes", maxBufferSize)
				}
				buffer = append(buffer, line...)
			}
			if len(buffer) == 0 {
				return nil, io.EOF
			}
			return bytes.TrimSpace(buffer), nil
		default:
			return nil, err
		}
	}
}

func consumeBufferedLine(reader *bufio.Reader, allowPartial bool) ([]byte, bool) {
	buffered := reader.Buffered()
	if buffered <= 0 {
		return nil, false
	}
	peeked, err := reader.Peek(buffered)
	if err != nil || len(peeked) == 0 {
		return nil, false
	}
	if index := bytes.IndexByte(peeked, '\n'); index >= 0 {
		line := append([]byte(nil), peeked[:index+1]...)
		_, _ = reader.Discard(index + 1)
		return line, true
	}
	if !allowPartial {
		return nil, false
	}
	line := append([]byte(nil), peeked...)
	_, _ = reader.Discard(len(peeked))
	return line, true
}

func isClosedReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(err.Error(), "file already closed")
}

type diagnosticTail struct {
	mu    sync.Mutex
	limit int
	text  string
}

func (t *diagnosticTail) Append(line string) {
	value := strings.TrimSpace(line)
	if value == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.limit <= 0 {
		t.limit = diagnosticTailLimit
	}
	if t.text == "" {
		t.text = value
	} else {
		t.text += "\n" + value
	}
	if len(t.text) > t.limit {
		t.text = t.text[len(t.text)-t.limit:]
	}
}

func (t *diagnosticTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.text
}

func diagnosticPreview(value []byte, suffix bool) string {
	text := string(bytes.TrimSpace(value))
	if len(text) <= diagnosticTextLimit {
		return text
	}
	if suffix {
		return text[len(text)-diagnosticTextLimit:]
	}
	return text[:diagnosticTextLimit]
}
