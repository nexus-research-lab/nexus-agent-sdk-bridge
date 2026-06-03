package client

import (
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/runtimeinfo"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

// ContextUsageCategory 表示上下文使用分类。
type ContextUsageCategory struct {
	Name       string `json:"name,omitempty"`
	Tokens     int    `json:"tokens,omitempty"`
	Color      string `json:"color,omitempty"`
	IsDeferred bool   `json:"is_deferred,omitempty"`
}

// ContextUsageEntry 表示上下文使用明细中的通用条目。
type ContextUsageEntry struct {
	Name        string         `json:"name,omitempty"`
	Path        string         `json:"path,omitempty"`
	Description string         `json:"description,omitempty"`
	Type        string         `json:"type,omitempty"`
	Scope       string         `json:"scope,omitempty"`
	Tokens      int            `json:"tokens,omitempty"`
	Count       int            `json:"count,omitempty"`
	Percentage  float64        `json:"percentage,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// ContextUsageGridCell 表示网格中的单个单元格。
type ContextUsageGridCell struct {
	Name       string         `json:"name,omitempty"`
	Value      string         `json:"value,omitempty"`
	Tokens     int            `json:"tokens,omitempty"`
	Count      int            `json:"count,omitempty"`
	Color      string         `json:"color,omitempty"`
	Percentage float64        `json:"percentage,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

// ContextUsageSlashCommand 表示 slash command 使用信息。
type ContextUsageSlashCommand struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Tokens      int            `json:"tokens,omitempty"`
	Count       int            `json:"count,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
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
	Categories           []ContextUsageCategory     `json:"categories,omitempty"`
	TotalTokens          int                        `json:"total_tokens,omitempty"`
	MaxTokens            int                        `json:"max_tokens,omitempty"`
	RawMaxTokens         int                        `json:"raw_max_tokens,omitempty"`
	Percentage           float64                    `json:"percentage,omitempty"`
	Model                string                     `json:"model,omitempty"`
	IsAutoCompactEnabled bool                       `json:"is_auto_compact_enabled,omitempty"`
	MemoryFiles          []ContextUsageEntry        `json:"memory_files,omitempty"`
	MCPTools             []ContextUsageEntry        `json:"mcp_tools,omitempty"`
	Agents               []ContextUsageEntry        `json:"agents,omitempty"`
	GridRows             [][]ContextUsageGridCell   `json:"grid_rows,omitempty"`
	AutoCompactThreshold int                        `json:"auto_compact_threshold,omitempty"`
	DeferredBuiltinTools []ContextUsageEntry        `json:"deferred_builtin_tools,omitempty"`
	SystemTools          []ContextUsageEntry        `json:"system_tools,omitempty"`
	SystemPromptSections []ContextUsageEntry        `json:"system_prompt_sections,omitempty"`
	SlashCommands        []ContextUsageSlashCommand `json:"slash_commands,omitempty"`
	APIUsage             ContextUsageAPIUsage       `json:"api_usage,omitempty"`
	Raw                  map[string]any             `json:"raw,omitempty"`
}

// RewindFilesResult 表示 Session.Control().RewindFiles 的结果。
type RewindFilesResult struct {
	CanRewind    bool           `json:"can_rewind,omitempty"`
	Error        string         `json:"error,omitempty"`
	FilesChanged []string       `json:"files_changed,omitempty"`
	Insertions   int            `json:"insertions,omitempty"`
	Deletions    int            `json:"deletions,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

// SlashCommand 表示当前会话可调用的 slash command。
type SlashCommand struct {
	Name         string         `json:"name,omitempty"`
	Description  string         `json:"description,omitempty"`
	ArgumentHint string         `json:"argument_hint,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

// ModelInfo 表示当前会话可选模型。
type ModelInfo struct {
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
	Vendor      string         `json:"vendor,omitempty"`
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
	APIProvider      string         `json:"api_provider,omitempty"`
	APIKeySource     string         `json:"api_key_source,omitempty"`
	Email            string         `json:"email,omitempty"`
	Organization     string         `json:"organization,omitempty"`
	SubscriptionType string         `json:"subscription_type,omitempty"`
	TokenSource      string         `json:"token_source,omitempty"`
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

// PluginStatus 表示 reload_plugins 后的插件状态。
type PluginStatus struct {
	Name   string `json:"name,omitempty"`
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
}

// ReloadPluginsResponse 表示 Session.Control().ReloadPlugins 的结果。
type ReloadPluginsResponse struct {
	Commands      []SlashCommand     `json:"commands,omitempty"`
	Agents        []AgentInfo        `json:"agents,omitempty"`
	Plugins       []PluginStatus     `json:"plugins,omitempty"`
	MCPServers    []mcp.ServerStatus `json:"mcp_servers,omitempty"`
	EnabledCount  int                `json:"enabled_count,omitempty"`
	DisabledCount int                `json:"disabled_count,omitempty"`
	CommandCount  int                `json:"command_count,omitempty"`
	AgentCount    int                `json:"agent_count,omitempty"`
	HookCount     int                `json:"hook_count,omitempty"`
	MCPCount      int                `json:"mcp_count,omitempty"`
	LSPCount      int                `json:"lsp_count,omitempty"`
	ErrorCount    int                `json:"error_count,omitempty"`
	Raw           map[string]any     `json:"raw,omitempty"`
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
			name = jsonvalue.StringValue(command.Raw["command"])
		}
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

func reloadPluginsResponseFromRuntime(info runtimeinfo.ReloadPluginsResponse) ReloadPluginsResponse {
	raw := jsonvalue.CloneMapPreserveTypedSlices(info.Raw)
	if raw == nil {
		raw = map[string]any{}
	}
	return ReloadPluginsResponse{
		Commands:      slashCommandsFromRuntime(info.Commands),
		Agents:        agentInfosFromRuntime(info.Agents),
		Plugins:       pluginStatusesFromRuntime(info.Plugins),
		MCPServers:    append([]mcp.ServerStatus(nil), info.MCPServers...),
		EnabledCount:  info.EnabledCount,
		DisabledCount: info.DisabledCount,
		CommandCount:  info.CommandCount,
		AgentCount:    info.AgentCount,
		HookCount:     info.HookCount,
		MCPCount:      info.MCPCount,
		LSPCount:      info.LSPCount,
		ErrorCount:    info.ErrorCount,
		Raw:           raw,
	}
}

func pluginStatusesFromRuntime(plugins []runtimeinfo.PluginStatus) []PluginStatus {
	result := make([]PluginStatus, 0, len(plugins))
	for _, plugin := range plugins {
		result = append(result, PluginStatus{
			Name:   plugin.Name,
			Path:   plugin.Path,
			Source: plugin.Source,
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
		APIProvider:      jsonvalue.FirstNonEmptyString(raw["apiProvider"], raw["api_provider"]),
		APIKeySource:     jsonvalue.FirstNonEmptyString(raw["apiKeySource"], raw["api_key_source"]),
		Email:            jsonvalue.FirstNonEmptyString(raw["email"], raw["email_address"], account.EmailAddress),
		Organization:     jsonvalue.FirstNonEmptyString(raw["organization"], raw["organization_name"], account.OrganizationName),
		SubscriptionType: jsonvalue.FirstNonEmptyString(raw["subscriptionType"], raw["subscription_type"], raw["subscription"], raw["plan"], account.Plan),
		TokenSource:      jsonvalue.FirstNonEmptyString(raw["tokenSource"], raw["token_source"]),
		Raw:              raw,
	}
}

func decodeReloadPluginsResponse(payload map[string]any) ReloadPluginsResponse {
	return reloadPluginsResponseFromRuntime(runtimeinfo.DecodeReloadPluginsResponse(payload))
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
			IsDeferred: jsonvalue.BoolValue(category["is_deferred"]),
		})
	}

	return ContextUsageResponse{
		Categories:           categories,
		TotalTokens:          jsonvalue.IntValue(payload["total_tokens"]),
		MaxTokens:            jsonvalue.IntValue(payload["max_tokens"]),
		RawMaxTokens:         jsonvalue.IntValue(payload["raw_max_tokens"]),
		Percentage:           jsonvalue.FloatValue(payload["percentage"]),
		Model:                jsonvalue.StringValue(payload["model"]),
		IsAutoCompactEnabled: jsonvalue.BoolValue(payload["is_auto_compact_enabled"]),
		MemoryFiles:          decodeContextUsageEntries(payload["memory_files"]),
		MCPTools:             decodeContextUsageEntries(payload["mcp_tools"]),
		Agents:               decodeContextUsageEntries(payload["agents"]),
		GridRows:             decodeContextUsageGridRows(payload["grid_rows"]),
		AutoCompactThreshold: jsonvalue.IntValue(payload["auto_compact_threshold"]),
		DeferredBuiltinTools: decodeContextUsageEntries(payload["deferred_builtin_tools"]),
		SystemTools:          decodeContextUsageEntries(payload["system_tools"]),
		SystemPromptSections: decodeContextUsageEntries(payload["system_prompt_sections"]),
		SlashCommands:        decodeContextUsageSlashCommands(payload["slash_commands"]),
		APIUsage:             decodeContextUsageAPIUsage(payload["api_usage"]),
		Raw:                  payload,
	}
}

func decodeRewindFilesResult(payload map[string]any) RewindFilesResult {
	return RewindFilesResult{
		CanRewind:    jsonvalue.BoolValue(payload["can_rewind"]),
		Error:        jsonvalue.StringValue(payload["error"]),
		FilesChanged: jsonvalue.StringSliceValue(payload["files_changed"]),
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
			Name:        jsonvalue.FirstNonEmptyString(item["name"], item["label"]),
			Path:        jsonvalue.FirstNonEmptyString(item["path"], item["file"]),
			Description: jsonvalue.StringValue(item["description"]),
			Type:        jsonvalue.StringValue(item["type"]),
			Scope:       jsonvalue.StringValue(item["scope"]),
			Tokens:      jsonvalue.IntValue(item["tokens"]),
			Count:       jsonvalue.IntValue(item["count"]),
			Percentage:  jsonvalue.FloatValue(item["percentage"]),
			Raw:         item,
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
				Name:       jsonvalue.FirstNonEmptyString(cell["name"], cell["label"]),
				Value:      jsonvalue.StringValue(cell["value"]),
				Tokens:     jsonvalue.IntValue(cell["tokens"]),
				Count:      jsonvalue.IntValue(cell["count"]),
				Color:      jsonvalue.StringValue(cell["color"]),
				Percentage: jsonvalue.FloatValue(cell["percentage"]),
				Raw:        cell,
			})
		}
		result = append(result, row)
	}
	return result
}

func decodeContextUsageSlashCommands(raw any) []ContextUsageSlashCommand {
	objectValue := jsonvalue.MapValue(raw)
	if len(objectValue) > 0 {
		result := make([]ContextUsageSlashCommand, 0, len(objectValue))
		for key, rawValue := range objectValue {
			payload := jsonvalue.MapValue(rawValue)
			if len(payload) == 0 {
				payload = map[string]any{"name": key, "value": rawValue}
			}
			if payload["name"] == nil {
				payload["name"] = key
			}
			result = append(result, ContextUsageSlashCommand{
				Name:        jsonvalue.StringValue(payload["name"]),
				Description: jsonvalue.StringValue(payload["description"]),
				Tokens:      jsonvalue.IntValue(payload["tokens"]),
				Count:       jsonvalue.IntValue(payload["count"]),
				Raw:         payload,
			})
		}
		return result
	}

	items := jsonvalue.MapSliceValue(raw)
	result := make([]ContextUsageSlashCommand, 0, len(items))
	for _, item := range items {
		result = append(result, ContextUsageSlashCommand{
			Name:        jsonvalue.FirstNonEmptyString(item["name"], item["command"]),
			Description: jsonvalue.StringValue(item["description"]),
			Tokens:      jsonvalue.IntValue(item["tokens"]),
			Count:       jsonvalue.IntValue(item["count"]),
			Raw:         item,
		})
	}
	return result
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
