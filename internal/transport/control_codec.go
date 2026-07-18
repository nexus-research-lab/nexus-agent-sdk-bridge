package transport

import (
	"context"
	"sync"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type controlCodecTransport struct {
	inner Transport

	mu              sync.Mutex
	requestSubtypes map[string]string
}

func newControlCodecTransport(inner Transport) Transport {
	return &controlCodecTransport{
		inner:           inner,
		requestSubtypes: map[string]string{},
	}
}

func (t *controlCodecTransport) Start(ctx context.Context) error {
	return t.inner.Start(ctx)
}

func (t *controlCodecTransport) ReadJSON() (map[string]any, error) {
	payload, err := t.inner.ReadJSON()
	if err != nil {
		return nil, err
	}
	if jsonvalue.StringValue(payload["type"]) == "control_response" {
		return t.normalizeInboundControlResponse(payload), nil
	}
	return t.normalizeInboundControlRequest(payload), nil
}

func (t *controlCodecTransport) WriteJSON(payload any) error {
	if formatted, ok := t.formatOutboundControlRequest(payload); ok {
		return t.inner.WriteJSON(formatted)
	}
	return t.inner.WriteJSON(t.formatOutboundControlResponse(payload))
}

func (t *controlCodecTransport) EndInput() error {
	return t.inner.EndInput()
}

func (t *controlCodecTransport) Interrupt() error {
	return t.inner.Interrupt()
}

func (t *controlCodecTransport) Wait() error {
	return t.inner.Wait()
}

func (t *controlCodecTransport) Close() error {
	return t.inner.Close()
}

func (t *controlCodecTransport) StderrTail() string {
	provider, ok := t.inner.(interface{ StderrTail() string })
	if !ok {
		return ""
	}
	return provider.StderrTail()
}

func (t *controlCodecTransport) normalizeInboundControlRequest(payload map[string]any) map[string]any {
	if jsonvalue.StringValue(payload["type"]) != "control_request" {
		return payload
	}
	requestID := jsonvalue.StringValue(payload["request_id"])
	request := jsonvalue.CloneMapValue(payload["request"])
	if request == nil {
		return payload
	}

	subtype := jsonvalue.StringValue(request["subtype"])
	if subtype == "can_use_tool" {
		if suggestions, ok := request["permission_suggestions"]; ok {
			request["permission_suggestions"] = normalizePermissionUpdatesFromClaude(suggestions)
		}
	}
	if requestID != "" && subtype != "" {
		t.mu.Lock()
		t.requestSubtypes[requestID] = subtype
		t.mu.Unlock()
	}

	normalized := jsonvalue.CloneMapPreserveTypedSlices(payload)
	normalized["request"] = request
	return normalized
}

func (t *controlCodecTransport) normalizeInboundControlResponse(payload map[string]any) map[string]any {
	response := jsonvalue.CloneMapValue(payload["response"])
	if response == nil {
		return payload
	}
	requestID := jsonvalue.StringValue(response["request_id"])
	subtype := ""
	if requestID != "" {
		t.mu.Lock()
		subtype = t.requestSubtypes[requestID]
		delete(t.requestSubtypes, requestID)
		t.mu.Unlock()
	}
	if subtype == "" || jsonvalue.StringValue(response["subtype"]) != "success" {
		normalized := jsonvalue.CloneMapPreserveTypedSlices(payload)
		normalized["response"] = response
		return normalized
	}

	body := jsonvalue.CloneMapValue(response["response"])
	if body != nil {
		response["response"] = normalizeControlResponseFromClaude(subtype, body)
	}
	normalized := jsonvalue.CloneMapPreserveTypedSlices(payload)
	normalized["response"] = response
	return normalized
}

func (t *controlCodecTransport) formatOutboundControlRequest(payload any) (any, bool) {
	envelope, ok := payload.(protocol.ControlRequestEnvelope)
	if !ok {
		if pointer, pointerOK := payload.(*protocol.ControlRequestEnvelope); pointerOK && pointer != nil {
			envelope = *pointer
			ok = true
		}
	}
	if !ok {
		return nil, false
	}

	request := controlRequestMap(envelope.Request)
	subtype := jsonvalue.StringValue(request["subtype"])
	if envelope.RequestID != "" && subtype != "" {
		t.mu.Lock()
		t.requestSubtypes[envelope.RequestID] = subtype
		t.mu.Unlock()
	}
	return map[string]any{
		"type":       "control_request",
		"request_id": envelope.RequestID,
		"request":    formatControlRequestForClaude(request),
	}, true
}

func (t *controlCodecTransport) formatOutboundControlResponse(payload any) any {
	response, ok := payload.(protocol.ControlResponseEnvelope)
	if !ok {
		if pointer, pointerOK := payload.(*protocol.ControlResponseEnvelope); pointerOK && pointer != nil {
			response = *pointer
			ok = true
		}
	}
	if !ok {
		return payload
	}

	requestID := response.Response.RequestID
	subtype := ""
	if requestID != "" {
		t.mu.Lock()
		subtype = t.requestSubtypes[requestID]
		delete(t.requestSubtypes, requestID)
		t.mu.Unlock()
	}
	if response.Response.Subtype != "success" || subtype == "" || len(response.Response.Response) == 0 {
		return response
	}

	response.Response.Response = formatControlResponseForClaude(subtype, response.Response.Response)
	return response
}

func controlRequestMap(request protocol.ControlRequest) map[string]any {
	payload, ok := jsonvalue.AnyMap(request)
	if !ok {
		return map[string]any{}
	}
	return jsonvalue.CloneMapPreserveTypedSlices(payload)
}

func formatControlRequestForClaude(request map[string]any) map[string]any {
	switch jsonvalue.StringValue(request["subtype"]) {
	case "initialize":
		return formatInitializeRequestForClaude(request)
	case "mcp_reconnect", "mcp_toggle", "mcp_authenticate", "mcp_clear_auth":
		return renameKeys(request, map[string]string{
			"server_name": "serverName",
		})
	case "mcp_oauth_callback_url":
		return renameKeys(request, map[string]string{
			"server_name":  "serverName",
			"callback_url": "callbackUrl",
		})
	default:
		return request
	}
}

func formatInitializeRequestForClaude(request map[string]any) map[string]any {
	output := renameKeys(request, map[string]string{
		"sdk_mcp_servers":          "sdkMcpServers",
		"json_schema":              "jsonSchema",
		"system_prompt":            "systemPrompt",
		"append_system_prompt":     "appendSystemPrompt",
		"exclude_dynamic_sections": "excludeDynamicSections",
		"agent_progress_summaries": "agentProgressSummaries",
	})
	if hooks := jsonvalue.MapValue(request["hooks"]); len(hooks) > 0 {
		output["hooks"] = formatHookInitializationForClaude(hooks)
	}
	if agents := jsonvalue.MapValue(request["agents"]); len(agents) > 0 {
		output["agents"] = formatAgentDefinitionsForClaude(agents)
	}
	return output
}

func formatHookInitializationForClaude(hooks map[string]any) map[string]any {
	output := make(map[string]any, len(hooks))
	for event, rawMatchers := range hooks {
		matchers := jsonvalue.SliceValue(rawMatchers)
		encoded := make([]map[string]any, 0, len(matchers))
		for _, rawMatcher := range matchers {
			matcher := jsonvalue.MapValue(rawMatcher)
			if len(matcher) == 0 {
				continue
			}
			encoded = append(encoded, renameKeys(matcher, map[string]string{
				"hook_callback_ids": "hookCallbackIds",
			}))
		}
		if len(encoded) > 0 {
			output[event] = encoded
		}
	}
	return output
}

func formatAgentDefinitionsForClaude(agents map[string]any) map[string]any {
	output := make(map[string]any, len(agents))
	for name, rawDefinition := range agents {
		definition := jsonvalue.MapValue(rawDefinition)
		if len(definition) == 0 {
			output[name] = jsonvalue.CloneValuePreserveTypedSlices(rawDefinition)
			continue
		}
		mapped := renameKeys(definition, map[string]string{
			"disallowed_tools":                      "disallowedTools",
			"mcp_servers":                           "mcpServers",
			"required_mcp_servers":                  "requiredMcpServers",
			"critical_system_reminder_experimental": "criticalSystemReminder_EXPERIMENTAL",
			"initial_prompt":                        "initialPrompt",
			"max_turns":                             "maxTurns",
			"permission_mode":                       "permissionMode",
		})
		output[name] = mapped
	}
	return output
}

func formatControlResponseForClaude(subtype string, response map[string]any) map[string]any {
	switch subtype {
	case "can_use_tool":
		return formatPermissionDecisionForClaude(response)
	case "hook_callback":
		return formatHookOutputForClaude(response)
	default:
		return response
	}
}

func normalizeControlResponseFromClaude(subtype string, response map[string]any) map[string]any {
	switch subtype {
	case "initialize":
		return normalizeInitializeResponseFromClaude(response)
	case "get_settings":
		return normalizeSettingsResponseFromClaude(response)
	case "mcp_status":
		return normalizeMCPStatusResponseFromClaude(response)
	case "get_context_usage":
		return normalizeContextUsageResponseFromClaude(response)
	case "rewind_files":
		return renameKeys(response, map[string]string{
			"canRewind":    "can_rewind",
			"filesChanged": "files_changed",
		})
	default:
		return response
	}
}

func normalizeInitializeResponseFromClaude(response map[string]any) map[string]any {
	output := jsonvalue.CloneMapPreserveTypedSlices(response)
	if commands := normalizeMapSlice(response["commands"], normalizeSlashCommandFromClaude); len(commands) > 0 {
		output["commands"] = commands
	}
	if models := normalizeMapSlice(response["models"], normalizeModelInfoFromClaude); len(models) > 0 {
		output["models"] = models
	}
	if account := jsonvalue.MapValue(response["account"]); len(account) > 0 {
		output["account"] = normalizeAccountInfoFromClaude(account)
	}
	return output
}

func normalizeSlashCommandFromClaude(command map[string]any) map[string]any {
	return renameKeys(command, map[string]string{
		"argumentHint":   "argument_hint",
		"allowedTools":   "allowed_tools",
		"loadedFrom":     "loaded_from",
		"userInvocable":  "user_invocable",
		"commandType":    "command_type",
		"sourceFilename": "source_filename",
	})
}

func normalizeModelInfoFromClaude(model map[string]any) map[string]any {
	output := renameKeys(model, map[string]string{
		"displayName":              "display_name",
		"supportsEffort":           "supports_effort",
		"supportedEffortLevels":    "supported_effort_levels",
		"supportsAdaptiveThinking": "supports_adaptive_thinking",
		"supportsFastMode":         "supports_fast_mode",
		"supportsAutoMode":         "supports_auto_mode",
	})
	if value := jsonvalue.StringValue(model["value"]); value != "" && output["id"] == nil {
		output["id"] = value
	}
	delete(output, "value")
	return output
}

func normalizeAccountInfoFromClaude(account map[string]any) map[string]any {
	output := renameKeys(account, map[string]string{
		"email":            "email_address",
		"organization":     "organization_name",
		"subscriptionType": "subscription_type",
		"tokenSource":      "token_source",
		"apiKeySource":     "api_key_source",
		"apiProvider":      "api_provider",
	})
	return output
}

func normalizeSettingsResponseFromClaude(response map[string]any) map[string]any {
	output := jsonvalue.CloneMapPreserveTypedSlices(response)
	if effective := jsonvalue.MapValue(response["effective"]); len(effective) > 0 {
		output["effective"] = renameKeys(effective, map[string]string{
			"permissionMode":    "permission_mode",
			"maxThinkingTokens": "max_thinking_tokens",
			"allowedTools":      "allowed_tools",
			"disallowedTools":   "disallowed_tools",
		})
	}
	return output
}

func normalizeMCPStatusResponseFromClaude(response map[string]any) map[string]any {
	output := renameKeys(response, map[string]string{
		"mcpServers": "mcp_servers",
	})
	if servers, ok := response["mcpServers"]; ok {
		output["mcp_servers"] = normalizeMCPServersFromClaude(servers)
	}
	return output
}

func normalizeMCPServersFromClaude(raw any) any {
	return normalizeMapSlice(raw, func(server map[string]any) map[string]any {
		output := renameKeys(server, map[string]string{
			"serverInfo": "server_info",
		})
		if tools := normalizeMapSlice(server["tools"], normalizeMCPToolFromClaude); len(tools) > 0 {
			output["tools"] = tools
		}
		return output
	})
}

func normalizeMCPToolFromClaude(tool map[string]any) map[string]any {
	output := renameKeys(tool, map[string]string{
		"inputSchema": "input_schema",
	})
	if annotations := jsonvalue.MapValue(tool["annotations"]); len(annotations) > 0 {
		output["annotations"] = renameKeys(annotations, map[string]string{
			"readOnlyHint":    "read_only_hint",
			"destructiveHint": "destructive_hint",
			"idempotentHint":  "idempotent_hint",
			"openWorldHint":   "open_world_hint",
			"readOnly":        "read_only",
			"destructive":     "destructive",
			"openWorld":       "open_world",
		})
	}
	return output
}

func normalizeContextUsageResponseFromClaude(response map[string]any) map[string]any {
	output := renameKeys(response, map[string]string{
		"totalTokens":            "total_tokens",
		"maxTokens":              "max_tokens",
		"rawMaxTokens":           "raw_max_tokens",
		"gridRows":               "grid_rows",
		"memoryFiles":            "memory_files",
		"mcpTools":               "mcp_tools",
		"deferredBuiltinTools":   "deferred_builtin_tools",
		"systemTools":            "system_tools",
		"systemPromptSections":   "system_prompt_sections",
		"slashCommands":          "slash_commands",
		"apiUsage":               "api_usage",
		"autoCompactThreshold":   "auto_compact_threshold",
		"isAutoCompactEnabled":   "is_auto_compact_enabled",
		"messageBreakdown":       "message_breakdown",
		"toolCallsByType":        "tool_calls_by_type",
		"attachmentTokens":       "attachment_tokens",
		"assistantMessageTokens": "assistant_message_tokens",
		"userMessageTokens":      "user_message_tokens",
	})
	if categories := normalizeMapSlice(response["categories"], func(category map[string]any) map[string]any {
		return renameKeys(category, map[string]string{"isDeferred": "is_deferred"})
	}); len(categories) > 0 {
		output["categories"] = categories
	}
	for _, key := range []string{"memoryFiles", "mcpTools", "agents", "deferredBuiltinTools", "systemTools", "systemPromptSections"} {
		if entries := normalizeMapSlice(response[key], normalizeContextUsageEntryFromClaude); len(entries) > 0 {
			if mapped, ok := map[string]string{
				"memoryFiles":          "memory_files",
				"mcpTools":             "mcp_tools",
				"deferredBuiltinTools": "deferred_builtin_tools",
				"systemTools":          "system_tools",
				"systemPromptSections": "system_prompt_sections",
			}[key]; ok {
				output[mapped] = entries
			} else {
				output[key] = entries
			}
		}
	}
	if rows := normalizeContextUsageGridRowsFromClaude(response["gridRows"]); len(rows) > 0 {
		output["grid_rows"] = rows
	}
	if slashCommands := jsonvalue.MapValue(response["slashCommands"]); len(slashCommands) > 0 {
		output["slash_commands"] = normalizeContextUsageSlashCommandsFromClaude(slashCommands)
	}
	return output
}

func normalizeContextUsageEntryFromClaude(entry map[string]any) map[string]any {
	return renameKeys(entry, map[string]string{
		"serverName": "server_name",
		"isLoaded":   "is_loaded",
		"agentType":  "agent_type",
	})
}

func normalizeContextUsageGridRowsFromClaude(raw any) [][]map[string]any {
	rows := jsonvalue.SliceValue(raw)
	result := make([][]map[string]any, 0, len(rows))
	for _, rawRow := range rows {
		cells := jsonvalue.SliceValue(rawRow)
		row := make([]map[string]any, 0, len(cells))
		for _, rawCell := range cells {
			cell := jsonvalue.MapValue(rawCell)
			if len(cell) == 0 {
				continue
			}
			row = append(row, renameKeys(cell, map[string]string{
				"isFilled":     "is_filled",
				"categoryName": "category_name",
			}))
		}
		result = append(result, row)
	}
	return result
}

func normalizeContextUsageSlashCommandsFromClaude(commands map[string]any) map[string]any {
	output := make(map[string]any, len(commands))
	for name, rawCommand := range commands {
		command := jsonvalue.MapValue(rawCommand)
		if len(command) == 0 {
			output[name] = jsonvalue.CloneValuePreserveTypedSlices(rawCommand)
			continue
		}
		output[name] = renameKeys(command, map[string]string{
			"totalCommands":    "total_commands",
			"includedCommands": "included_commands",
		})
	}
	return output
}

func normalizeMapSlice(raw any, normalize func(map[string]any) map[string]any) []map[string]any {
	items := jsonvalue.SliceValue(raw)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}
		result = append(result, normalize(payload))
	}
	return result
}

func formatPermissionDecisionForClaude(response map[string]any) map[string]any {
	output := renameKeys(response, map[string]string{
		"updated_input":           "updatedInput",
		"updated_permissions":     "updatedPermissions",
		"tool_use_id":             "toolUseID",
		"decision_classification": "decisionClassification",
	})
	if updates, ok := response["updated_permissions"]; ok {
		output["updatedPermissions"] = formatPermissionUpdatesForClaude(updates)
	}
	return output
}

func formatHookOutputForClaude(response map[string]any) map[string]any {
	output := renameKeys(response, map[string]string{
		"async_timeout":        "asyncTimeout",
		"suppress_output":      "suppressOutput",
		"stop_reason":          "stopReason",
		"system_message":       "systemMessage",
		"hook_specific_output": "hookSpecificOutput",
	})
	if specific, ok := response["hook_specific_output"]; ok {
		output["hookSpecificOutput"] = formatHookSpecificOutputForClaude(specific)
	}
	return output
}

func formatHookSpecificOutputForClaude(raw any) any {
	specific := jsonvalue.MapValue(raw)
	if len(specific) == 0 {
		return jsonvalue.CloneValuePreserveTypedSlices(raw)
	}
	output := renameKeys(specific, map[string]string{
		"hook_event_name":            "hookEventName",
		"permission_decision":        "permissionDecision",
		"permission_decision_reason": "permissionDecisionReason",
		"updated_input":              "updatedInput",
		"additional_context":         "additionalContext",
		"additional_contexts":        "additionalContexts",
		"initial_user_message":       "initialUserMessage",
		"watch_paths":                "watchPaths",
		"updated_mcp_tool_output":    "updatedMCPToolOutput",
		"worktree_path":              "worktreePath",
	})
	if decision := jsonvalue.MapValue(specific["decision"]); len(decision) > 0 {
		output["decision"] = formatPermissionDecisionForClaude(decision)
	}
	return output
}

func renameKeys(input map[string]any, mapping map[string]string) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		if mapped, ok := mapping[key]; ok {
			output[mapped] = jsonvalue.CloneValuePreserveTypedSlices(value)
			continue
		}
		output[key] = jsonvalue.CloneValuePreserveTypedSlices(value)
	}
	return output
}

func formatPermissionUpdatesForClaude(raw any) any {
	switch updates := raw.(type) {
	case []permission.Update:
		return encodePermissionUpdatesForClaude(updates)
	case []map[string]any:
		result := make([]map[string]any, 0, len(updates))
		for _, update := range updates {
			result = append(result, formatPermissionUpdateForClaude(update))
		}
		return result
	case []any:
		result := make([]any, 0, len(updates))
		for _, item := range updates {
			update := jsonvalue.MapValue(item)
			if len(update) == 0 {
				result = append(result, jsonvalue.CloneValuePreserveTypedSlices(item))
				continue
			}
			result = append(result, formatPermissionUpdateForClaude(update))
		}
		return result
	default:
		return jsonvalue.CloneValuePreserveTypedSlices(raw)
	}
}

func formatPermissionUpdateForClaude(update map[string]any) map[string]any {
	output := jsonvalue.CloneMapPreserveTypedSlices(update)
	if rules, ok := update["rules"]; ok {
		output["rules"] = formatPermissionRuleValuesForClaude(rules)
	}
	return output
}

func formatPermissionRuleValuesForClaude(raw any) any {
	switch rules := raw.(type) {
	case []permission.RuleValue:
		return encodePermissionRuleValuesForClaude(rules)
	case []map[string]any:
		result := make([]map[string]any, 0, len(rules))
		for _, rule := range rules {
			result = append(result, renameKeys(rule, map[string]string{
				"tool_name":    "toolName",
				"rule_content": "ruleContent",
			}))
		}
		return result
	case []any:
		result := make([]any, 0, len(rules))
		for _, item := range rules {
			rule := jsonvalue.MapValue(item)
			if len(rule) == 0 {
				result = append(result, jsonvalue.CloneValuePreserveTypedSlices(item))
				continue
			}
			result = append(result, renameKeys(rule, map[string]string{
				"tool_name":    "toolName",
				"rule_content": "ruleContent",
			}))
		}
		return result
	default:
		return jsonvalue.CloneValuePreserveTypedSlices(raw)
	}
}

func encodePermissionUpdatesForClaude(updates []permission.Update) []map[string]any {
	result := make([]map[string]any, 0, len(updates))
	for _, update := range updates {
		payload := map[string]any{
			"type": update.Type,
		}
		if len(update.Rules) > 0 {
			payload["rules"] = encodePermissionRuleValuesForClaude(update.Rules)
		}
		if update.Behavior != "" {
			payload["behavior"] = string(update.Behavior)
		}
		if update.Mode != "" {
			payload["mode"] = string(update.Mode)
		}
		if len(update.Directories) > 0 {
			payload["directories"] = append([]string(nil), update.Directories...)
		}
		if update.Destination != "" {
			payload["destination"] = string(update.Destination)
		}
		result = append(result, payload)
	}
	return result
}

func encodePermissionRuleValuesForClaude(rules []permission.RuleValue) []map[string]any {
	result := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		payload := map[string]any{
			"toolName": rule.ToolName,
		}
		if rule.RuleContent != "" {
			payload["ruleContent"] = rule.RuleContent
		}
		result = append(result, payload)
	}
	return result
}

func normalizePermissionUpdatesFromClaude(raw any) any {
	switch updates := raw.(type) {
	case []map[string]any:
		result := make([]map[string]any, 0, len(updates))
		for _, update := range updates {
			result = append(result, normalizePermissionUpdateFromClaude(update))
		}
		return result
	case []any:
		result := make([]any, 0, len(updates))
		for _, item := range updates {
			update := jsonvalue.MapValue(item)
			if len(update) == 0 {
				result = append(result, jsonvalue.CloneValuePreserveTypedSlices(item))
				continue
			}
			result = append(result, normalizePermissionUpdateFromClaude(update))
		}
		return result
	default:
		return jsonvalue.CloneValuePreserveTypedSlices(raw)
	}
}

func normalizePermissionUpdateFromClaude(update map[string]any) map[string]any {
	output := jsonvalue.CloneMapPreserveTypedSlices(update)
	if rules, ok := update["rules"]; ok {
		output["rules"] = normalizePermissionRuleValuesFromClaude(rules)
	}
	return output
}

func normalizePermissionRuleValuesFromClaude(raw any) any {
	switch rules := raw.(type) {
	case []map[string]any:
		result := make([]map[string]any, 0, len(rules))
		for _, rule := range rules {
			result = append(result, renameKeys(rule, map[string]string{
				"toolName":    "tool_name",
				"ruleContent": "rule_content",
			}))
		}
		return result
	case []any:
		result := make([]any, 0, len(rules))
		for _, item := range rules {
			rule := jsonvalue.MapValue(item)
			if len(rule) == 0 {
				result = append(result, jsonvalue.CloneValuePreserveTypedSlices(item))
				continue
			}
			result = append(result, renameKeys(rule, map[string]string{
				"toolName":    "tool_name",
				"ruleContent": "rule_content",
			}))
		}
		return result
	default:
		return jsonvalue.CloneValuePreserveTypedSlices(raw)
	}
}
