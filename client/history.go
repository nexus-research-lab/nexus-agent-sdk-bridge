package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

// ListSessionsOptions 表示会话列表查询选项。
type ListSessionsOptions struct {
	Directory        string
	Limit            int
	IncludeWorktrees *bool
}

// GetSessionMessagesOptions 表示会话消息查询选项。
type GetSessionMessagesOptions struct {
	Directory string
	Limit     int
	Offset    int
}

// SessionLookupOptions 表示单个会话查找选项。
type SessionLookupOptions struct {
	Directory string
}

// SessionMutationOptions 表示按 session id 修改本地会话元数据的参数。
type SessionMutationOptions struct {
	Directory string
}

// SessionInfo 表示本地持久化会话的摘要信息。
type SessionInfo struct {
	SessionID    string
	Summary      string
	LastModified int64
	FileSize     int64
	CustomTitle  *string
	FirstPrompt  *string
	GitBranch    *string
	CWD          *string
	Tag          *string
	CreatedAt    *int64
}

// SessionMessage 表示持久化 transcript 中的一条用户或助手消息。
type SessionMessage struct {
	Type            string
	UUID            string
	SessionID       string
	Message         any
	ParentToolUseID *string
	Raw             map[string]any
}

type sessionRecord struct {
	info SessionInfo
	path string
}

// ListSessions 列出本地持久化的会话摘要。
func ListSessions(options ListSessionsOptions) ([]SessionInfo, error) {
	records, err := listSessionRecords(options)
	if err != nil {
		return nil, err
	}
	sessions := make([]SessionInfo, 0, len(records))
	for _, record := range records {
		sessions = append(sessions, record.info)
	}
	return sessions, nil
}

// GetSessionInfo 读取单个会话的摘要信息；找不到时返回 nil。
func GetSessionInfo(sessionID string, options SessionLookupOptions) (*SessionInfo, error) {
	record, err := findSessionRecord(sessionID, options.Directory)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	info := record.info
	return &info, nil
}

// GetSessionMessages 读取单个会话中的用户和助手消息。
func GetSessionMessages(sessionID string, options GetSessionMessagesOptions) ([]SessionMessage, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("client: session id is required")
	}
	record, err := findSessionRecord(sessionID, options.Directory)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, os.ErrNotExist
	}

	messages := []SessionMessage{}
	if err := readSessionJSONL(record.path, func(payload map[string]any) error {
		messageType := jsonvalue.TrimmedStringValue(payload["type"])
		if messageType != "user" && messageType != "assistant" {
			return nil
		}
		messages = append(messages, SessionMessage{
			Type:            messageType,
			UUID:            jsonvalue.TrimmedStringValue(payload["uuid"]),
			SessionID:       firstString(payload["session_id"], payload["sessionId"], record.info.SessionID),
			Message:         payload["message"],
			ParentToolUseID: stringPointer(payload["parent_tool_use_id"]),
			Raw:             cloneAnyMap(payload),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	offset := options.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(messages) {
		return []SessionMessage{}, nil
	}
	messages = messages[offset:]
	if options.Limit > 0 && options.Limit < len(messages) {
		messages = messages[:options.Limit]
	}
	return messages, nil
}

// RenameSession 写入自定义会话标题；重复调用时最后一次生效。
func RenameSession(sessionID string, title string, options SessionMutationOptions) error {
	title = strings.TrimSpace(title)
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("client: session id is required")
	}
	if title == "" {
		return errors.New("client: session title is required")
	}
	record, err := findSessionRecord(sessionID, options.Directory)
	if err != nil {
		return err
	}
	if record == nil {
		return os.ErrNotExist
	}
	return appendSessionEntry(record.path, sessionTitleEntry{
		Type:        "custom-title",
		CustomTitle: title,
		SessionID:   record.info.SessionID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// TagSession 写入或清除会话标签；tag 为 nil 时清除标签。
func TagSession(sessionID string, tag *string, options SessionMutationOptions) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("client: session id is required")
	}
	if tag != nil {
		trimmed := strings.TrimSpace(*tag)
		if trimmed == "" {
			return errors.New("client: session tag is required")
		}
		tag = &trimmed
	}
	record, err := findSessionRecord(sessionID, options.Directory)
	if err != nil {
		return err
	}
	if record == nil {
		return os.ErrNotExist
	}
	return appendSessionEntry(record.path, sessionTagEntry{
		Type:      "session-tag",
		Tag:       tag,
		SessionID: record.info.SessionID,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

type sessionTitleEntry struct {
	Type        string `json:"type"`
	CustomTitle string `json:"customTitle"`
	SessionID   string `json:"sessionId"`
	Timestamp   string `json:"timestamp"`
}

type sessionTagEntry struct {
	Type      string  `json:"type"`
	Tag       *string `json:"tag"`
	SessionID string  `json:"sessionId"`
	Timestamp string  `json:"timestamp"`
}

func listSessionRecords(options ListSessionsOptions) ([]sessionRecord, error) {
	files, err := sessionFiles(options.Directory, includeWorktrees(options.IncludeWorktrees))
	if err != nil {
		return nil, err
	}
	records := make([]sessionRecord, 0, len(files))
	for _, file := range files {
		record, err := parseSessionFile(file)
		if err != nil {
			return nil, err
		}
		if options.Directory != "" && !sessionMatchesDirectory(record, options.Directory, includeWorktrees(options.IncludeWorktrees)) {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i int, j int) bool {
		return records[i].info.LastModified > records[j].info.LastModified
	})
	if options.Limit > 0 && options.Limit < len(records) {
		records = records[:options.Limit]
	}
	return records, nil
}

func findSessionRecord(sessionID string, directory string) (*sessionRecord, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("client: session id is required")
	}
	files, err := sessionFiles(directory, true)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		record, err := parseSessionFile(file)
		if err != nil {
			return nil, err
		}
		if record.info.SessionID == sessionID {
			return &record, nil
		}
	}
	return nil, nil
}

func sessionFiles(directory string, includeWorktrees bool) ([]string, error) {
	projectsRoot := filepath.Join(resolveConfigDir(nil), "projects")
	if strings.TrimSpace(directory) != "" && !includeWorktrees {
		projectDir, err := exactProjectDir(projectsRoot, directory)
		if err != nil {
			return nil, err
		}
		return jsonlFiles(projectDir)
	}

	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirFiles, err := jsonlFiles(filepath.Join(projectsRoot, entry.Name()))
		if err != nil {
			return nil, err
		}
		files = append(files, dirFiles...)
	}
	return files, nil
}

func jsonlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	return files, nil
}

func parseSessionFile(path string) (sessionRecord, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return sessionRecord{}, err
	}
	record := sessionRecord{
		path: path,
		info: SessionInfo{
			SessionID:    strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			LastModified: stat.ModTime().UnixMilli(),
			FileSize:     stat.Size(),
		},
	}
	var aiTitle string
	messageCount := 0

	if err := readSessionJSONL(path, func(payload map[string]any) error {
		sessionID := firstString(payload["session_id"], payload["sessionId"])
		if sessionID != "" {
			record.info.SessionID = sessionID
		}
		if cwd := jsonvalue.TrimmedStringValue(payload["cwd"]); cwd != "" {
			record.info.CWD = &cwd
		}
		if branch := firstString(payload["git_branch"], payload["gitBranch"]); branch != "" {
			record.info.GitBranch = &branch
		}
		if createdAt := parseTimestampMillis(payload["timestamp"]); createdAt != nil && record.info.CreatedAt == nil {
			record.info.CreatedAt = createdAt
		}

		switch jsonvalue.TrimmedStringValue(payload["type"]) {
		case "ai-title":
			aiTitle = firstString(payload["aiTitle"], payload["ai_title"], payload["title"])
		case "custom-title":
			if title := firstString(payload["customTitle"], payload["custom_title"], payload["title"]); title != "" {
				record.info.CustomTitle = &title
			}
		case "session-tag":
			if payload["tag"] == nil {
				record.info.Tag = nil
			} else if tag := jsonvalue.TrimmedStringValue(payload["tag"]); tag != "" {
				record.info.Tag = &tag
			}
		case "user", "assistant":
			messageCount++
			if record.info.FirstPrompt == nil && jsonvalue.TrimmedStringValue(payload["type"]) == "user" {
				if prompt := firstPromptText(payload["message"]); prompt != "" {
					record.info.FirstPrompt = &prompt
				}
			}
		}
		return nil
	}); err != nil {
		return sessionRecord{}, err
	}

	switch {
	case record.info.CustomTitle != nil && *record.info.CustomTitle != "":
		record.info.Summary = *record.info.CustomTitle
	case aiTitle != "":
		record.info.Summary = aiTitle
	case record.info.FirstPrompt != nil && *record.info.FirstPrompt != "":
		record.info.Summary = *record.info.FirstPrompt
	default:
		record.info.Summary = record.info.SessionID
	}
	_ = messageCount
	return record, nil
}

func readSessionJSONL(path string, fn func(map[string]any) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			payload := map[string]any{}
			if decodeErr := json.Unmarshal([]byte(line), &payload); decodeErr != nil {
				return fmt.Errorf("client: decode session transcript %s failed: %w", path, decodeErr)
			}
			if fnErr := fn(payload); fnErr != nil {
				return fnErr
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return nil
}

func appendSessionEntry(path string, entry any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return err
	}
	return nil
}

func exactProjectDir(projectsRoot string, directory string) (string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	return filepath.Join(projectsRoot, encodeProjectDirectory(absolute)), nil
}

func encodeProjectDirectory(directory string) string {
	var builder strings.Builder
	for _, value := range directory {
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') {
			builder.WriteRune(value)
			continue
		}
		builder.WriteByte('-')
	}
	sanitized := builder.String()
	if len(sanitized) <= maxProjectDirectoryNameLength {
		return sanitized
	}
	return sanitized[:maxProjectDirectoryNameLength] + "-" + projectPathHashSuffix(directory)
}

func sessionMatchesDirectory(record sessionRecord, directory string, includeWorktrees bool) bool {
	if directory == "" {
		return true
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return false
	}
	if record.info.CWD != nil {
		recordCWD, err := filepath.Abs(*record.info.CWD)
		if err == nil && recordCWD == absolute {
			return true
		}
		if includeWorktrees && sameGitRoot(absolute, recordCWD) {
			return true
		}
	}
	projectDir, err := exactProjectDir(filepath.Join(resolveConfigDir(nil), "projects"), absolute)
	if err != nil {
		return false
	}
	return filepath.Dir(record.path) == projectDir
}

func includeWorktrees(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func sameGitRoot(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftRoot := findGitRoot(left)
	rightRoot := findGitRoot(right)
	return leftRoot != "" && rightRoot != "" && leftRoot == rightRoot
}

func findGitRoot(directory string) string {
	current := directory
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func firstPromptText(message any) string {
	payload, ok := message.(map[string]any)
	if !ok {
		return ""
	}
	switch content := payload["content"].(type) {
	case string:
		return strings.TrimSpace(content)
	case []any:
		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok || jsonvalue.TrimmedStringValue(block["type"]) != "text" {
				continue
			}
			if text := jsonvalue.TrimmedStringValue(block["text"]); text != "" {
				return text
			}
		}
	}
	return ""
}

func parseTimestampMillis(value any) *int64 {
	raw := jsonvalue.TrimmedStringValue(value)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	result := parsed.UnixMilli()
	return &result
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := jsonvalue.TrimmedStringValue(value); text != "" {
			return text
		}
	}
	return ""
}

func stringPointer(value any) *string {
	text := jsonvalue.TrimmedStringValue(value)
	if text == "" {
		return nil
	}
	return &text
}
