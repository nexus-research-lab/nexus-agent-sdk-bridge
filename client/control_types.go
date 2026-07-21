package client

import (
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
)

// ContextUsageCategory 表示上下文使用分类。
type ContextUsageCategory struct {
	Name       string `json:"name,omitempty"`
	Tokens     int    `json:"tokens,omitempty"`
	Color      string `json:"color,omitempty"`
	IsDeferred bool   `json:"isDeferred,omitempty"`
}

// ContextUsageEntry 表示上下文使用明细中的通用条目。
type ContextUsageEntry struct {
	Name       string         `json:"name,omitempty"`
	Path       string         `json:"path,omitempty"`
	Type       string         `json:"type,omitempty"`
	ServerName string         `json:"serverName,omitempty"`
	IsLoaded   bool           `json:"isLoaded,omitempty"`
	AgentType  string         `json:"agentType,omitempty"`
	Source     string         `json:"source,omitempty"`
	Tokens     int            `json:"tokens,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// ContextUsageGridCell 表示网格中的单个单元格。
type ContextUsageGridCell struct {
	Color          string         `json:"color,omitempty"`
	IsFilled       bool           `json:"isFilled,omitempty"`
	CategoryName   string         `json:"categoryName,omitempty"`
	Tokens         int            `json:"tokens,omitempty"`
	Percentage     float64        `json:"percentage,omitempty"`
	SquareFullness float64        `json:"squareFullness,omitempty"`
	Raw            map[string]any `json:"raw,omitempty"`
}

// ContextUsageSlashCommands 表示 slash command 的上下文占用汇总。
type ContextUsageSlashCommands struct {
	TotalCommands    int            `json:"totalCommands,omitempty"`
	IncludedCommands int            `json:"includedCommands,omitempty"`
	Tokens           int            `json:"tokens,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

// ContextUsageAPIUsage 表示 API 账单相关使用情况。
type ContextUsageAPIUsage struct {
	InputTokens              int            `json:"input_tokens,omitempty"`
	OutputTokens             int            `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int            `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int            `json:"cache_read_input_tokens,omitempty"`
	Raw                      map[string]any `json:"raw,omitempty"`
}

// ContextUsageResponse 表示 Session.Control().ContextUsage 的结果。
type ContextUsageResponse struct {
	Categories           []ContextUsageCategory    `json:"categories,omitempty"`
	TotalTokens          int                       `json:"totalTokens,omitempty"`
	MaxTokens            int                       `json:"maxTokens,omitempty"`
	RawMaxTokens         int                       `json:"rawMaxTokens,omitempty"`
	Percentage           float64                   `json:"percentage,omitempty"`
	Model                string                    `json:"model,omitempty"`
	IsAutoCompactEnabled bool                      `json:"isAutoCompactEnabled,omitempty"`
	MemoryFiles          []ContextUsageEntry       `json:"memoryFiles,omitempty"`
	MCPTools             []ContextUsageEntry       `json:"mcpTools,omitempty"`
	Agents               []ContextUsageEntry       `json:"agents,omitempty"`
	GridRows             [][]ContextUsageGridCell  `json:"gridRows,omitempty"`
	AutoCompactThreshold int                       `json:"autoCompactThreshold,omitempty"`
	DeferredBuiltinTools []ContextUsageEntry       `json:"deferredBuiltinTools,omitempty"`
	SystemTools          []ContextUsageEntry       `json:"systemTools,omitempty"`
	SystemPromptSections []ContextUsageEntry       `json:"systemPromptSections,omitempty"`
	SlashCommands        ContextUsageSlashCommands `json:"slashCommands,omitempty"`
	APIUsage             ContextUsageAPIUsage      `json:"apiUsage,omitempty"`
	Raw                  map[string]any            `json:"raw,omitempty"`
}

// RewindFilesResult 表示 Session.Control().RewindFiles 的结果。
type RewindFilesResult struct {
	CanRewind    bool           `json:"canRewind,omitempty"`
	Error        string         `json:"error,omitempty"`
	FilesChanged []string       `json:"filesChanged,omitempty"`
	Insertions   int            `json:"insertions,omitempty"`
	Deletions    int            `json:"deletions,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

// SlashCommand 表示当前会话可调用的 slash command。
type SlashCommand struct {
	Name         string         `json:"name,omitempty"`
	Description  string         `json:"description,omitempty"`
	ArgumentHint string         `json:"argumentHint,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

// ModelInfo 表示当前会话可选模型。
type ModelInfo struct {
	ID          string         `json:"value,omitempty"`
	Name        string         `json:"name,omitempty"`
	DisplayName string         `json:"displayName,omitempty"`
	Vendor      string         `json:"description,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// AgentInfo 表示当前会话可用 agent。
type AgentInfo struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	Model       string         `json:"model,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// AccountInfo 表示当前会话的账号快照。
type AccountInfo struct {
	APIProvider      string         `json:"apiProvider,omitempty"`
	APIKeySource     string         `json:"apiKeySource,omitempty"`
	Email            string         `json:"email,omitempty"`
	Organization     string         `json:"organization,omitempty"`
	SubscriptionType string         `json:"subscriptionType,omitempty"`
	TokenSource      string         `json:"tokenSource,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

// InitializationResult 表示 runtime 初始化后的公开只读快照。
type InitializationResult struct {
	Account               AccountInfo    `json:"account,omitempty"`
	Agents                []AgentInfo    `json:"agents,omitempty"`
	AvailableOutputStyles []string       `json:"available_output_styles,omitempty"`
	Commands              []SlashCommand `json:"commands,omitempty"`
	Models                []ModelInfo    `json:"models,omitempty"`
	OutputStyle           string         `json:"output_style,omitempty"`
	FastModeState         string         `json:"fast_mode_state,omitempty"`
	Raw                   map[string]any `json:"raw,omitempty"`
}

// SettingsSource 表示 settings 的单个来源。
type SettingsSource struct {
	Source   string         `json:"source,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

// SettingsApplied 表示已经投影到 runtime 状态的 settings。
type SettingsApplied struct {
	Model  string         `json:"model,omitempty"`
	Effort *string        `json:"effort,omitempty"`
	Raw    map[string]any `json:"raw,omitempty"`
}

// SettingsResponse 表示 Session.Control().GetSettings 的结果。
type SettingsResponse struct {
	Effective map[string]any   `json:"effective,omitempty"`
	Sources   []SettingsSource `json:"sources,omitempty"`
	Applied   SettingsApplied  `json:"applied,omitempty"`
	Raw       map[string]any   `json:"raw,omitempty"`
}

// AutoDreamStatus 表示一次 AutoDream 检查的终态。
type AutoDreamStatus string

const (
	// AutoDreamStatusSkipped 表示当前未满足执行条件。
	AutoDreamStatusSkipped AutoDreamStatus = "skipped"
	// AutoDreamStatusCompleted 表示记忆巩固已经完成。
	AutoDreamStatusCompleted AutoDreamStatus = "completed"
)

// AutoDreamResult 表示 nxs 对 AutoDream 唤醒请求的处理结果。
type AutoDreamResult struct {
	Status           AutoDreamStatus `json:"status,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	SessionsReviewed int             `json:"sessions_reviewed,omitempty"`
	NextCheckAtMS    int64           `json:"next_check_at_ms,omitempty"`
	Summary          string          `json:"summary,omitempty"`
	WrittenPaths     []string        `json:"written_paths,omitempty"`
	Raw              map[string]any  `json:"raw,omitempty"`
}

func initializationResultFromRuntime(info runtimeinfo.InitializeResponse) InitializationResult {
	raw := jsonvalue.CloneMap(info.Raw)
	if raw == nil {
		raw = map[string]any{}
	}
	return InitializationResult{
		Account:               accountInfoFromRuntime(info.Account),
		Agents:                agentInfosFromRuntime(info.Agents),
		AvailableOutputStyles: jsonvalue.CloneStringSlice(info.AvailableOutputStyles),
		Commands:              slashCommandsFromRuntime(info.Commands),
		Models:                modelInfosFromRuntime(info.Models),
		OutputStyle:           info.OutputStyle,
		FastModeState:         info.FastModeState,
		Raw:                   raw,
	}
}

func slashCommandsFromRuntime(commands []runtimeinfo.SlashCommandInfo) []SlashCommand {
	result := make([]SlashCommand, 0, len(commands))
	for _, command := range commands {
		if command.UserInvocable != nil && !*command.UserInvocable {
			continue
		}
		name := strings.TrimSpace(command.Name)
		if name == "" {
			continue
		}
		result = append(result, SlashCommand{
			Name:         name,
			Description:  command.Description,
			ArgumentHint: command.ArgumentHint,
			Raw:          jsonvalue.CloneMap(command.Raw),
		})
	}
	return result
}

func modelInfosFromRuntime(models []runtimeinfo.ModelInfo) []ModelInfo {
	result := make([]ModelInfo, 0, len(models))
	for _, model := range models {
		result = append(result, ModelInfo{
			ID:          model.ID,
			Name:        model.Name,
			DisplayName: model.DisplayName,
			Vendor:      model.Vendor,
			Raw:         jsonvalue.CloneMap(model.Raw),
		})
	}
	return result
}

func agentInfosFromRuntime(agents []runtimeinfo.AgentInfo) []AgentInfo {
	result := make([]AgentInfo, 0, len(agents))
	for _, agent := range agents {
		result = append(result, AgentInfo{
			Name:        agent.Name,
			Description: agent.Description,
			Prompt:      agent.Prompt,
			Model:       agent.Model,
			Raw:         jsonvalue.CloneMap(agent.Raw),
		})
	}
	return result
}

func accountInfoFromRuntime(account runtimeinfo.AccountInfo) AccountInfo {
	raw := jsonvalue.CloneMap(account.Raw)
	if raw == nil {
		raw = map[string]any{}
	}
	return AccountInfo{
		APIProvider:      account.APIProvider,
		APIKeySource:     account.APIKeySource,
		Email:            account.Email,
		Organization:     account.Organization,
		SubscriptionType: account.SubscriptionType,
		TokenSource:      account.TokenSource,
		Raw:              raw,
	}
}

func decodeSettingsResponse(payload map[string]any) SettingsResponse {
	sources := []SettingsSource{}
	for _, item := range jsonvalue.SliceValue(payload["sources"]) {
		source := jsonvalue.MapValue(item)
		if len(source) == 0 {
			continue
		}
		sources = append(sources, SettingsSource{
			Source:   jsonvalue.StringValue(source["source"]),
			Settings: jsonvalue.CloneMapPreserveTypedSlices(jsonvalue.MapValue(source["settings"])),
			Raw:      jsonvalue.CloneMapPreserveTypedSlices(source),
		})
	}

	raw := jsonvalue.CloneMapPreserveTypedSlices(payload)
	if raw == nil {
		raw = map[string]any{}
	}
	return SettingsResponse{
		Effective: jsonvalue.CloneMapPreserveTypedSlices(jsonvalue.MapValue(payload["effective"])),
		Sources:   sources,
		Applied:   decodeSettingsApplied(payload["applied"]),
		Raw:       raw,
	}
}

func decodeAutoDreamResult(payload map[string]any) AutoDreamResult {
	nextCheckAtMS, _ := jsonvalue.Int64Value(payload["next_check_at_ms"])
	raw := jsonvalue.CloneMapPreserveTypedSlices(payload)
	if raw == nil {
		raw = map[string]any{}
	}
	return AutoDreamResult{
		Status:           AutoDreamStatus(jsonvalue.StringValue(payload["status"])),
		Reason:           jsonvalue.StringValue(payload["reason"]),
		SessionsReviewed: jsonvalue.IntValue(payload["sessions_reviewed"]),
		NextCheckAtMS:    nextCheckAtMS,
		Summary:          jsonvalue.StringValue(payload["summary"]),
		WrittenPaths:     jsonvalue.StringSliceValue(payload["written_paths"]),
		Raw:              raw,
	}
}

func decodeSettingsApplied(raw any) SettingsApplied {
	payload := jsonvalue.MapValue(raw)
	if len(payload) == 0 {
		return SettingsApplied{}
	}
	return SettingsApplied{
		Model:  jsonvalue.StringValue(payload["model"]),
		Effort: optionalStringValue(payload["effort"]),
		Raw:    jsonvalue.CloneMapPreserveTypedSlices(payload),
	}
}

func optionalStringValue(value any) *string {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		return &typed
	default:
		text := jsonvalue.StringValue(value)
		if text == "" {
			return nil
		}
		return &text
	}
}

func decodeContextUsageResponse(payload map[string]any) ContextUsageResponse {
	categories := make([]ContextUsageCategory, 0, len(jsonvalue.SliceValue(payload["categories"])))
	for _, item := range jsonvalue.SliceValue(payload["categories"]) {
		category := jsonvalue.MapValue(item)
		if len(category) == 0 {
			continue
		}
		categories = append(categories, ContextUsageCategory{
			Name:       jsonvalue.StringValue(category["name"]),
			Tokens:     jsonvalue.IntValue(category["tokens"]),
			Color:      jsonvalue.StringValue(category["color"]),
			IsDeferred: jsonvalue.BoolValue(category["isDeferred"]),
		})
	}

	return ContextUsageResponse{
		Categories:           categories,
		TotalTokens:          jsonvalue.IntValue(payload["totalTokens"]),
		MaxTokens:            jsonvalue.IntValue(payload["maxTokens"]),
		RawMaxTokens:         jsonvalue.IntValue(payload["rawMaxTokens"]),
		Percentage:           jsonvalue.FloatValue(payload["percentage"]),
		Model:                jsonvalue.StringValue(payload["model"]),
		IsAutoCompactEnabled: jsonvalue.BoolValue(payload["isAutoCompactEnabled"]),
		MemoryFiles:          decodeContextUsageEntries(payload["memoryFiles"]),
		MCPTools:             decodeContextUsageEntries(payload["mcpTools"]),
		Agents:               decodeContextUsageEntries(payload["agents"]),
		GridRows:             decodeContextUsageGridRows(payload["gridRows"]),
		AutoCompactThreshold: jsonvalue.IntValue(payload["autoCompactThreshold"]),
		DeferredBuiltinTools: decodeContextUsageEntries(payload["deferredBuiltinTools"]),
		SystemTools:          decodeContextUsageEntries(payload["systemTools"]),
		SystemPromptSections: decodeContextUsageEntries(payload["systemPromptSections"]),
		SlashCommands:        decodeContextUsageSlashCommands(payload["slashCommands"]),
		APIUsage:             decodeContextUsageAPIUsage(payload["apiUsage"]),
		Raw:                  payload,
	}
}

func decodeRewindFilesResult(payload map[string]any) RewindFilesResult {
	return RewindFilesResult{
		CanRewind:    jsonvalue.BoolValue(payload["canRewind"]),
		Error:        jsonvalue.StringValue(payload["error"]),
		FilesChanged: jsonvalue.StringSliceValue(payload["filesChanged"]),
		Insertions:   jsonvalue.IntValue(payload["insertions"]),
		Deletions:    jsonvalue.IntValue(payload["deletions"]),
		Raw:          payload,
	}
}

func decodeContextUsageEntries(raw any) []ContextUsageEntry {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]ContextUsageEntry, 0, len(items))
	for _, item := range items {
		result = append(result, ContextUsageEntry{
			Name:       jsonvalue.StringValue(item["name"]),
			Path:       jsonvalue.StringValue(item["path"]),
			Type:       jsonvalue.StringValue(item["type"]),
			ServerName: jsonvalue.StringValue(item["serverName"]),
			IsLoaded:   jsonvalue.BoolValue(item["isLoaded"]),
			AgentType:  jsonvalue.StringValue(item["agentType"]),
			Source:     jsonvalue.StringValue(item["source"]),
			Tokens:     jsonvalue.IntValue(item["tokens"]),
			Raw:        item,
		})
	}
	return result
}

func decodeContextUsageGridRows(raw any) [][]ContextUsageGridCell {
	rows := jsonvalue.SliceValue(raw)
	result := make([][]ContextUsageGridCell, 0, len(rows))
	for _, rowRaw := range rows {
		cells := jsonvalue.SliceValue(rowRaw)
		row := make([]ContextUsageGridCell, 0, len(cells))
		for _, cellRaw := range cells {
			cell := jsonvalue.MapValue(cellRaw)
			if len(cell) == 0 {
				continue
			}
			row = append(row, ContextUsageGridCell{
				Color:          jsonvalue.StringValue(cell["color"]),
				IsFilled:       jsonvalue.BoolValue(cell["isFilled"]),
				CategoryName:   jsonvalue.StringValue(cell["categoryName"]),
				Tokens:         jsonvalue.IntValue(cell["tokens"]),
				Percentage:     jsonvalue.FloatValue(cell["percentage"]),
				SquareFullness: jsonvalue.FloatValue(cell["squareFullness"]),
				Raw:            cell,
			})
		}
		result = append(result, row)
	}
	return result
}

func decodeContextUsageSlashCommands(raw any) ContextUsageSlashCommands {
	payload := jsonvalue.MapValue(raw)
	return ContextUsageSlashCommands{
		TotalCommands:    jsonvalue.IntValue(payload["totalCommands"]),
		IncludedCommands: jsonvalue.IntValue(payload["includedCommands"]),
		Tokens:           jsonvalue.IntValue(payload["tokens"]),
		Raw:              payload,
	}
}

func decodeContextUsageAPIUsage(raw any) ContextUsageAPIUsage {
	payload := jsonvalue.MapValue(raw)
	return ContextUsageAPIUsage{
		InputTokens:              jsonvalue.IntValue(payload["input_tokens"]),
		OutputTokens:             jsonvalue.IntValue(payload["output_tokens"]),
		CacheCreationInputTokens: jsonvalue.IntValue(payload["cache_creation_input_tokens"]),
		CacheReadInputTokens:     jsonvalue.IntValue(payload["cache_read_input_tokens"]),
		Raw:                      payload,
	}
}
