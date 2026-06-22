package client

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

var (
	apiRetryStderrPattern = regexp.MustCompile(`API error \(attempt ([0-9]+)/([0-9]+)\):\s*(.*)`)
	apiRetryStatusPattern = regexp.MustCompile(`\b(429|529)\b`)
)

func apiRetryMessageFromStderr(line string, sessionID string) (protocol.ReceivedMessage, bool) {
	matches := apiRetryStderrPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 4 {
		return protocol.ReceivedMessage{}, false
	}
	attempt, err := strconv.Atoi(matches[1])
	if err != nil {
		return protocol.ReceivedMessage{}, false
	}
	maxRetries, err := strconv.Atoi(matches[2])
	if err != nil {
		return protocol.ReceivedMessage{}, false
	}
	detail := strings.TrimSpace(matches[3])
	data := map[string]any{
		"message":     apiRetryMessageText(detail),
		"attempt":     attempt,
		"max_retries": maxRetries,
		"error":       classifyAPIRetryError(detail),
	}
	if status := apiRetryStatus(detail); status != "" {
		data["error_status"] = status
	}
	return protocol.ReceivedMessage{
		Type:      protocol.MessageTypeSystem,
		Subtype:   "api_retry",
		SessionID: sessionID,
		UUID:      fmt.Sprintf("api_retry_%d_%d", attempt, maxRetries),
		System: &protocol.SystemMessage{
			Subtype: "api_retry",
			Data:    data,
		},
	}, true
}

func normalizeAPIRetrySystemMessage(message protocol.ReceivedMessage) protocol.ReceivedMessage {
	if message.Type != protocol.MessageTypeSystem || message.System == nil {
		return message
	}
	if message.Subtype != "api_error" && message.System.Subtype != "api_error" {
		return message
	}
	data := cloneAPIRetryData(message.System.Data)
	data["subtype"] = "api_retry"
	if _, ok := data["attempt"]; !ok {
		if attempt := firstAPIRetryInt(data, "retryAttempt", "retry_attempt"); attempt > 0 {
			data["attempt"] = attempt
		}
	}
	if _, ok := data["max_retries"]; !ok {
		if maxRetries := firstAPIRetryInt(data, "maxRetries", "max_retries"); maxRetries > 0 {
			data["max_retries"] = maxRetries
		}
	}
	if _, ok := data["retry_delay_ms"]; !ok {
		if retryDelayMS := firstAPIRetryInt(data, "retryInMs", "retry_delay_ms"); retryDelayMS > 0 {
			data["retry_delay_ms"] = retryDelayMS
		}
	}
	if _, ok := data["error_status"]; !ok {
		if status := firstAPIRetryInt(data, "error_status", "status"); status > 0 {
			data["error_status"] = status
		} else if errorStatus := jsonvalue.IntValue(jsonvalue.MapValue(data["error"])["status"]); errorStatus > 0 {
			data["error_status"] = errorStatus
		}
	}
	if rawError, ok := data["error"]; ok && rawError != nil {
		data["raw_error"] = rawError
		data["error"] = classifyAPIRetryError(fmt.Sprint(rawError))
	} else {
		data["error"] = classifyAPIRetryError(fmt.Sprint(message.System.Data["error"]))
	}
	if strings.TrimSpace(jsonvalue.StringValue(data["message"])) == "" {
		data["message"] = apiRetryMessageText(fmt.Sprint(data["error"]))
	}
	message.Subtype = "api_retry"
	message.System.Subtype = "api_retry"
	message.System.Data = data
	return message
}

func cloneAPIRetryData(source map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range source {
		result[key] = value
	}
	return result
}

func firstAPIRetryInt(data map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := jsonvalue.IntValue(data[key]); value > 0 {
			return value
		}
	}
	return 0
}

func apiRetryMessageText(detail string) string {
	if classifyAPIRetryError(detail) == "rate_limit" {
		return "模型请求暂时受限，正在自动重试。"
	}
	return "API 请求失败，正在自动重试。"
}

func classifyAPIRetryError(detail string) string {
	normalized := strings.ToLower(detail)
	switch {
	case strings.Contains(normalized, "overloaded_error"),
		strings.Contains(normalized, "rate_limit"),
		strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "529"),
		strings.Contains(normalized, "429"):
		return "rate_limit"
	case strings.Contains(normalized, "timeout") || strings.Contains(normalized, "timed out"):
		return "timeout"
	case strings.Contains(normalized, "connection") || strings.Contains(normalized, "connect"):
		return "connection"
	default:
		return "api_error"
	}
}

func apiRetryStatus(detail string) string {
	matches := apiRetryStatusPattern.FindStringSubmatch(detail)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}
