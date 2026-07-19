package runtimeinfo

import (
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
)

// InitializeResponse 表示 runtime 初始化响应快照。
type InitializeResponse struct {
	SessionID             string             `json:"session_id,omitempty"`
	ProtocolCapabilities  []string           `json:"protocol_capabilities,omitempty"`
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
	ArgumentHint  string         `json:"argumentHint,omitempty"`
	AllowedTools  []string       `json:"allowedTools,omitempty"`
	Category      string         `json:"category,omitempty"`
	Source        string         `json:"source,omitempty"`
	LoadedFrom    string         `json:"loadedFrom,omitempty"`
	Kind          string         `json:"kind,omitempty"`
	UserInvocable *bool          `json:"userInvocable,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

// ModelInfo 表示初始化响应中的模型信息。
type ModelInfo struct {
	ID          string         `json:"value,omitempty"`
	Name        string         `json:"name,omitempty"`
	DisplayName string         `json:"displayName,omitempty"`
	Vendor      string         `json:"description,omitempty"`
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
	Email            string         `json:"email,omitempty"`
	Organization     string         `json:"organization,omitempty"`
	SubscriptionType string         `json:"subscriptionType,omitempty"`
	TokenSource      string         `json:"tokenSource,omitempty"`
	APIKeySource     string         `json:"apiKeySource,omitempty"`
	APIProvider      string         `json:"apiProvider,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

// DecodeInitializeResponse 解析初始化响应。
func DecodeInitializeResponse(payload map[string]any) InitializeResponse {
	return InitializeResponse{
		SessionID:             jsonvalue.StringValue(payload["session_id"]),
		ProtocolCapabilities:  jsonvalue.StringSliceValue(payload["protocol_capabilities"]),
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
			Name:          jsonvalue.StringValue(item["name"]),
			Type:          jsonvalue.StringValue(item["type"]),
			Description:   jsonvalue.StringValue(item["description"]),
			ArgumentHint:  jsonvalue.StringValue(item["argumentHint"]),
			AllowedTools:  parseSlashCommandTools(item["allowedTools"]),
			Category:      jsonvalue.StringValue(item["category"]),
			Source:        jsonvalue.StringValue(item["source"]),
			LoadedFrom:    jsonvalue.StringValue(item["loadedFrom"]),
			Kind:          jsonvalue.StringValue(item["kind"]),
			UserInvocable: parseOptionalBool(item["userInvocable"]),
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
		value := jsonvalue.StringValue(item["value"])
		result = append(result, ModelInfo{
			ID:          value,
			Name:        value,
			DisplayName: jsonvalue.StringValue(item["displayName"]),
			Vendor:      jsonvalue.StringValue(item["description"]),
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
		Email:            jsonvalue.StringValue(item["email"]),
		Organization:     jsonvalue.StringValue(item["organization"]),
		SubscriptionType: jsonvalue.StringValue(item["subscriptionType"]),
		TokenSource:      jsonvalue.StringValue(item["tokenSource"]),
		APIKeySource:     jsonvalue.StringValue(item["apiKeySource"]),
		APIProvider:      jsonvalue.StringValue(item["apiProvider"]),
		Raw:              item,
	}
}
