package protocol

import "strings"

// RuntimeLifecycleNodeKind 表示 Bridge 能从统一消息协议中确认的运行节点类型。
// Host 可以把这些事件绑定到自己的 round / execution identity，但不应反向把
// 产品状态写进 provider 消息。
type RuntimeLifecycleNodeKind string

const (
	RuntimeLifecycleNodeTool     RuntimeLifecycleNodeKind = "tool"
	RuntimeLifecycleNodeSubagent RuntimeLifecycleNodeKind = "subagent"
)

// RuntimeLifecyclePhase 表示一次运行节点的单调阶段。
type RuntimeLifecyclePhase string

const (
	RuntimeLifecycleStarted  RuntimeLifecyclePhase = "started"
	RuntimeLifecycleProgress RuntimeLifecyclePhase = "progress"
	RuntimeLifecycleFinished RuntimeLifecyclePhase = "finished"
)

// RuntimeLifecycleEvent 是从 ReceivedMessage 确定性派生的 provider-neutral 事件。
// EventID 在同一 runtime 消息重复投递时保持稳定；Metadata 只保留低敏的展示
// 线索，不复制工具参数或结果正文。
type RuntimeLifecycleEvent struct {
	EventID         string                   `json:"event_id"`
	NodeKind        RuntimeLifecycleNodeKind `json:"node_kind"`
	Phase           RuntimeLifecyclePhase    `json:"phase"`
	SubjectID       string                   `json:"subject_id"`
	ParentSubjectID string                   `json:"parent_subject_id,omitempty"`
	Name            string                   `json:"name,omitempty"`
	Description     string                   `json:"description,omitempty"`
	AgentID         string                   `json:"agent_id,omitempty"`
	ChildSessionID  string                   `json:"child_session_id,omitempty"`
	Status          string                   `json:"status,omitempty"`
	Failed          bool                     `json:"failed,omitempty"`
	Metadata        map[string]string        `json:"metadata,omitempty"`
}

// DeriveRuntimeLifecycleEvents 把所有 runtime 已统一的工具与子智能体消息投影成
// lifecycle 事件。未知消息返回空切片；该函数不依赖具体 Provider 或 runtime 名称。
func DeriveRuntimeLifecycleEvents(message ReceivedMessage) []RuntimeLifecycleEvent {
	events := make([]RuntimeLifecycleEvent, 0)
	parentToolUseID := stringPointerValue(message.ParentToolUseID)

	if message.Assistant != nil {
		if parentToolUseID == "" {
			parentToolUseID = stringPointerValue(message.Assistant.ParentToolUseID)
		}
		for _, block := range message.Assistant.Message.Content {
			toolUse, ok := AsToolUseBlock(block)
			if !ok || strings.TrimSpace(toolUse.ID) == "" {
				continue
			}
			events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
				NodeKind:        RuntimeLifecycleNodeTool,
				Phase:           RuntimeLifecycleStarted,
				SubjectID:       strings.TrimSpace(toolUse.ID),
				ParentSubjectID: parentToolUseID,
				Name:            strings.TrimSpace(toolUse.Name),
				Status:          "running",
			}))
		}
	}

	if message.User != nil {
		if parentToolUseID == "" {
			parentToolUseID = stringPointerValue(message.User.ParentToolUseID)
		}
		for _, block := range message.User.Message.Content {
			toolResult, ok := AsToolResultBlock(block)
			if !ok || strings.TrimSpace(toolResult.ToolUseID) == "" {
				continue
			}
			status := "succeeded"
			if toolResult.IsError {
				status = "failed"
			}
			events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
				NodeKind:        RuntimeLifecycleNodeTool,
				Phase:           RuntimeLifecycleFinished,
				SubjectID:       strings.TrimSpace(toolResult.ToolUseID),
				ParentSubjectID: parentToolUseID,
				Status:          status,
				Failed:          toolResult.IsError,
			}))
		}
	}

	if progress := message.ToolProgress; progress != nil && strings.TrimSpace(progress.ToolUseID) != "" {
		events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeTool,
			Phase:           RuntimeLifecycleProgress,
			SubjectID:       strings.TrimSpace(progress.ToolUseID),
			ParentSubjectID: stringPointerValue(progress.ParentToolUseID),
			Name:            strings.TrimSpace(progress.ToolName),
			Status:          "running",
		}))
	}

	taskStarted := message.TaskStarted
	taskProgress := message.TaskProgress
	taskNotification := message.TaskNotification
	taskUpdated := message.TaskUpdated
	if message.System != nil {
		if taskStarted == nil {
			taskStarted = message.System.TaskStarted
		}
		if taskProgress == nil {
			taskProgress = message.System.TaskProgress
		}
		if taskNotification == nil {
			taskNotification = message.System.TaskNotification
		}
		if taskUpdated == nil {
			taskUpdated = message.System.TaskUpdated
		}
	}
	if taskStarted != nil && strings.TrimSpace(taskStarted.TaskID) != "" {
		events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeSubagent,
			Phase:           RuntimeLifecycleStarted,
			SubjectID:       strings.TrimSpace(taskStarted.TaskID),
			ParentSubjectID: strings.TrimSpace(taskStarted.ParentTaskID),
			Name:            firstLifecycleValue(taskStarted.AgentType, taskStarted.TaskType, taskStarted.WorkflowName),
			Description:     strings.TrimSpace(taskStarted.Description),
			AgentID:         strings.TrimSpace(taskStarted.AgentID),
			ChildSessionID:  strings.TrimSpace(taskStarted.ChildSessionID),
			Status:          "running",
			Metadata: lifecycleMetadata(
				"tool_use_id", taskStarted.ToolUseID,
				"workflow_name", taskStarted.WorkflowName,
			),
		}))
	}
	if taskProgress != nil && strings.TrimSpace(taskProgress.TaskID) != "" {
		events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeSubagent,
			Phase:           RuntimeLifecycleProgress,
			SubjectID:       strings.TrimSpace(taskProgress.TaskID),
			ParentSubjectID: strings.TrimSpace(taskProgress.ParentTaskID),
			Name:            firstLifecycleValue(taskProgress.AgentType, taskProgress.TaskType),
			Description:     firstLifecycleValue(taskProgress.Summary, taskProgress.Description),
			AgentID:         strings.TrimSpace(taskProgress.AgentID),
			ChildSessionID:  strings.TrimSpace(taskProgress.ChildSessionID),
			Status:          "running",
			Metadata: lifecycleMetadata(
				"tool_use_id", taskProgress.ToolUseID,
				"last_tool_name", taskProgress.LastToolName,
			),
		}))
	}
	if taskNotification != nil && strings.TrimSpace(taskNotification.TaskID) != "" {
		status := normalizeLifecycleTerminalStatus(taskNotification.Status)
		events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeSubagent,
			Phase:           RuntimeLifecycleFinished,
			SubjectID:       strings.TrimSpace(taskNotification.TaskID),
			ParentSubjectID: strings.TrimSpace(taskNotification.ParentTaskID),
			Description:     strings.TrimSpace(taskNotification.Summary),
			AgentID:         strings.TrimSpace(taskNotification.AgentID),
			ChildSessionID:  strings.TrimSpace(taskNotification.ChildSessionID),
			Status:          status,
			Failed:          lifecycleStatusFailed(status),
			Metadata:        lifecycleMetadata("tool_use_id", taskNotification.ToolUseID),
		}))
	}
	if taskUpdated != nil && strings.TrimSpace(taskUpdated.TaskID) != "" {
		status := firstLifecycleValue(taskUpdated.Status, taskUpdated.Patch.Status)
		phase := RuntimeLifecycleProgress
		if lifecycleStatusTerminal(status) {
			phase = RuntimeLifecycleFinished
		}
		status = normalizeLifecycleTerminalStatus(status)
		events = append(events, lifecycleEvent(message, RuntimeLifecycleEvent{
			NodeKind:    RuntimeLifecycleNodeSubagent,
			Phase:       phase,
			SubjectID:   strings.TrimSpace(taskUpdated.TaskID),
			Description: strings.TrimSpace(taskUpdated.Patch.Description),
			Status:      status,
			Failed:      lifecycleStatusFailed(status),
		}))
	}

	return events
}

func lifecycleEvent(message ReceivedMessage, event RuntimeLifecycleEvent) RuntimeLifecycleEvent {
	event.EventID = strings.Join([]string{
		firstLifecycleValue(message.UUID, message.SessionID, "runtime"),
		string(event.NodeKind),
		string(event.Phase),
		event.SubjectID,
		firstLifecycleValue(event.Status, "unknown"),
	}, ":")
	return event
}

func lifecycleMetadata(values ...string) map[string]string {
	result := make(map[string]string)
	for index := 0; index+1 < len(values); index += 2 {
		if value := strings.TrimSpace(values[index+1]); value != "" {
			result[values[index]] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func firstLifecycleValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func normalizeLifecycleTerminalStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "", "completed", "complete", "success", "succeeded", "done":
		return "succeeded"
	case "error", "failed", "failure":
		return "failed"
	case "cancelled", "canceled":
		return "cancelled"
	case "interrupted", "stopped", "aborted":
		return "interrupted"
	default:
		return status
	}
}

func lifecycleStatusTerminal(status string) bool {
	switch normalizeLifecycleTerminalStatus(status) {
	case "succeeded", "failed", "cancelled", "interrupted":
		return true
	default:
		return false
	}
}

func lifecycleStatusFailed(status string) bool {
	switch normalizeLifecycleTerminalStatus(status) {
	case "failed", "cancelled", "interrupted":
		return true
	default:
		return false
	}
}
