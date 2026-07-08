package runtimeinfo

import (
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

// InitializeResponse 表示 runtime 初始化响应快照。
type InitializeResponse struct {
	SessionID             string             `json:"session_id,omitempty"`
	Commands              []SlashCommandInfo `json:"commands,omitempty"`
	Agents                []AgentInfo        `json:"agents,omitempty"`
	OutputStyle           string             `json:"output_style,omitempty"`
	AvailableOutputStyles []string           `json:"available_output_styles,omitempty"`
	Models                []ModelInfo        `json:"models,omitempty"`
	Account               AccountInfo        `json:"account,omitempty"`
	FastModeState         string             `json:"fast_mode_state,omitempty"`
	Raw                   map[string]any     `json:"raw,omitempty"`
}

// SlashCommandInfo 表示初始化响应中的 slash command。
type SlashCommandInfo struct {
	Name          string         `json:"name,omitempty"`
	Type          string         `json:"type,omitempty"`
	Description   string         `json:"description,omitempty"`
	ArgumentHint  string         `json:"argument_hint,omitempty"`
	AllowedTools  []string       `json:"allowed_tools,omitempty"`
	Category      string         `json:"category,omitempty"`
	Source        string         `json:"source,omitempty"`
	LoadedFrom    string         `json:"loaded_from,omitempty"`
	Kind          string         `json:"kind,omitempty"`
	UserInvocable *bool          `json:"user_invocable,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

// ModelInfo 表示初始化响应中的模型信息。
type ModelInfo struct {
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
	Vendor      string         `json:"vendor,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// AgentInfo 表示初始化响应中的 agent 信息。
type AgentInfo struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	Model       string         `json:"model,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// AccountInfo 表示初始化响应中的账号信息。
type AccountInfo struct {
	EmailAddress       string         `json:"email_address,omitempty"`
	OrganizationName   string         `json:"organization_name,omitempty"`
	Plan               string         `json:"plan,omitempty"`
	SubscriptionStatus string         `json:"subscription_status,omitempty"`
	Raw                map[string]any `json:"raw,omitempty"`
}

// DecodeInitializeResponse 解析初始化响应。
func DecodeInitializeResponse(payload map[string]any) InitializeResponse {
	return InitializeResponse{
		SessionID:             jsonvalue.FirstNonEmptyString(payload["session_id"], payload["sessionId"]),
		Commands:              decodeSlashCommands(payload["commands"]),
		Agents:                decodeAgents(payload["agents"]),
		OutputStyle:           jsonvalue.StringValue(payload["output_style"]),
		AvailableOutputStyles: jsonvalue.StringSliceValue(payload["available_output_styles"]),
		Models:                decodeModels(payload["models"]),
		Account:               decodeAccountInfo(payload["account"]),
		FastModeState:         jsonvalue.StringValue(payload["fast_mode_state"]),
		Raw:                   payload,
	}
}

func decodeSlashCommands(raw any) []SlashCommandInfo {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]SlashCommandInfo, 0, len(items))
	for _, item := range items {
		result = append(result, SlashCommandInfo{
			Name:          jsonvalue.FirstNonEmptyString(item["name"], item["command"]),
			Type:          jsonvalue.FirstNonEmptyString(item["type"], item["command_type"]),
			Description:   jsonvalue.StringValue(item["description"]),
			ArgumentHint:  jsonvalue.StringValue(item["argument_hint"]),
			AllowedTools:  parseSlashCommandTools(item["allowed_tools"]),
			Category:      jsonvalue.StringValue(item["category"]),
			Source:        jsonvalue.StringValue(item["source"]),
			LoadedFrom:    jsonvalue.StringValue(item["loaded_from"]),
			Kind:          jsonvalue.StringValue(item["kind"]),
			UserInvocable: parseOptionalBool(item["user_invocable"]),
			Raw:           item,
		})
	}
	return result
}

func parseOptionalBool(values ...any) *bool {
	for _, value := range values {
		switch typed := value.(type) {
		case bool:
			result := typed
			return &result
		case string:
			normalized := strings.TrimSpace(strings.ToLower(typed))
			switch normalized {
			case "true":
				result := true
				return &result
			case "false":
				result := false
				return &result
			}
		}
	}
	return nil
}

func parseSlashCommandTools(values ...any) []string {
	for _, value := range values {
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			tools := make([]string, 0, len(typed))
			for _, item := range typed {
				if text := jsonvalue.StringValue(item); text != "" {
					tools = append(tools, text)
				}
			}
			if len(tools) > 0 {
				return tools
			}
		case string:
			return splitToolList(typed)
		}
	}
	return nil
}

func splitToolList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		raw = strings.TrimSpace(raw[1 : len(raw)-1])
	}
	if raw == "" {
		return nil
	}
	values := []string{}
	current := strings.Builder{}
	depth := 0
	var quote rune
	flush := func() {
		part := strings.TrimSpace(current.String())
		current.Reset()
		part = strings.Trim(part, `"'`)
		if part != "" {
			values = append(values, part)
		}
	}
	for _, r := range raw {
		switch {
		case quote != 0:
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			current.WriteRune(r)
		case r == '(' || r == '[' || r == '{':
			depth++
			current.WriteRune(r)
		case r == ')' || r == ']' || r == '}':
			if depth > 0 {
				depth--
			}
			current.WriteRune(r)
		case r == ',' && depth == 0:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return values
}

func decodeModels(raw any) []ModelInfo {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]ModelInfo, 0, len(items))
	for _, item := range items {
		result = append(result, ModelInfo{
			ID:          jsonvalue.FirstNonEmptyString(item["id"], item["name"]),
			Name:        jsonvalue.StringValue(item["name"]),
			DisplayName: jsonvalue.StringValue(item["display_name"]),
			Vendor:      jsonvalue.StringValue(item["vendor"]),
			Raw:         item,
		})
	}
	return result
}

func decodeAgents(raw any) []AgentInfo {
	items := jsonvalue.MapSliceValue(raw)
	result := make([]AgentInfo, 0, len(items))
	for _, item := range items {
		result = append(result, AgentInfo{
			Name:        jsonvalue.StringValue(item["name"]),
			Description: jsonvalue.StringValue(item["description"]),
			Prompt:      jsonvalue.StringValue(item["prompt"]),
			Model:       jsonvalue.StringValue(item["model"]),
			Raw:         item,
		})
	}
	return result
}

func decodeAccountInfo(raw any) AccountInfo {
	item := jsonvalue.MapValue(raw)
	return AccountInfo{
		EmailAddress:       jsonvalue.FirstNonEmptyString(item["email"], item["email_address"]),
		OrganizationName:   jsonvalue.StringValue(item["organization_name"]),
		Plan:               jsonvalue.FirstNonEmptyString(item["plan"], item["subscription_plan"]),
		SubscriptionStatus: jsonvalue.StringValue(item["subscription_status"]),
		Raw:                item,
	}
}
