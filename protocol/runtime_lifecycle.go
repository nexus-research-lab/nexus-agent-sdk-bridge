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
			if !ok {
				continue
			}
			events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
				NodeKind:        RuntimeLifecycleNodeTool,
				Phase:           RuntimeLifecycleStarted,
				SubjectID:       toolUse.ID,
				ParentSubjectID: parentToolUseID,
				Name:            toolUse.Name,
				Status:          "running",
			})
		}
	}

	if message.User != nil {
		if parentToolUseID == "" {
			parentToolUseID = stringPointerValue(message.User.ParentToolUseID)
		}
		for _, block := range message.User.Message.Content {
			toolResult, ok := AsToolResultBlock(block)
			if !ok {
				continue
			}
			status := "succeeded"
			if toolResult.IsError {
				status = "failed"
			}
			events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
				NodeKind:        RuntimeLifecycleNodeTool,
				Phase:           RuntimeLifecycleFinished,
				SubjectID:       toolResult.ToolUseID,
				ParentSubjectID: parentToolUseID,
				Status:          status,
				Failed:          toolResult.IsError,
			})
		}
	}

	if progress := message.ToolProgress; progress != nil {
		subjectID := strings.TrimSpace(progress.ToolUseID)
		if subjectID != "" {
			events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
				NodeKind:        RuntimeLifecycleNodeTool,
				Phase:           RuntimeLifecycleProgress,
				SubjectID:       subjectID,
				ParentSubjectID: stringPointerValue(progress.ParentToolUseID),
				Name:            progress.ToolName,
				Status:          "running",
			})
			if subagent, ok := agentProgressLifecycle(progress); ok {
				events = appendLifecycleEvent(events, message, subagent)
			}
		}
	}

	if subagent, ok := structuredOutputSubagentLifecycle(message.Attachment); ok {
		events = appendLifecycleEvent(events, message, subagent)
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
	if taskStarted != nil {
		events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeSubagent,
			Phase:           RuntimeLifecycleStarted,
			SubjectID:       taskStarted.TaskID,
			ParentSubjectID: taskStarted.ParentTaskID,
			Name:            firstLifecycleValue(taskStarted.AgentType, taskStarted.TaskType, taskStarted.WorkflowName),
			Description:     taskStarted.Description,
			AgentID:         taskStarted.AgentID,
			ChildSessionID:  taskStarted.ChildSessionID,
			Status:          "running",
			Metadata: lifecycleMetadata(
				"tool_use_id", taskStarted.ToolUseID,
				"workflow_name", taskStarted.WorkflowName,
			),
		})
	}
	if taskProgress != nil {
		events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeSubagent,
			Phase:           RuntimeLifecycleProgress,
			SubjectID:       taskProgress.TaskID,
			ParentSubjectID: taskProgress.ParentTaskID,
			Name:            firstLifecycleValue(taskProgress.AgentType, taskProgress.TaskType),
			Description:     firstLifecycleValue(taskProgress.Summary, taskProgress.Description),
			AgentID:         taskProgress.AgentID,
			ChildSessionID:  taskProgress.ChildSessionID,
			Status:          "running",
			Metadata: lifecycleMetadata(
				"tool_use_id", taskProgress.ToolUseID,
				"last_tool_name", taskProgress.LastToolName,
			),
		})
	}
	if taskNotification != nil {
		status := normalizeLifecycleTerminalStatus(taskNotification.Status)
		events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
			NodeKind:        RuntimeLifecycleNodeSubagent,
			Phase:           RuntimeLifecycleFinished,
			SubjectID:       taskNotification.TaskID,
			ParentSubjectID: taskNotification.ParentTaskID,
			Description:     taskNotification.Summary,
			AgentID:         taskNotification.AgentID,
			ChildSessionID:  taskNotification.ChildSessionID,
			Status:          status,
			Failed:          lifecycleStatusFailed(status),
			Metadata:        lifecycleMetadata("tool_use_id", taskNotification.ToolUseID),
		})
	}
	if taskUpdated != nil {
		status := normalizeLifecycleTerminalStatus(firstLifecycleValue(taskUpdated.Status, taskUpdated.Patch.Status))
		phase := RuntimeLifecycleProgress
		if lifecycleStatusTerminal(status) {
			phase = RuntimeLifecycleFinished
		}
		events = appendLifecycleEvent(events, message, RuntimeLifecycleEvent{
			NodeKind:    RuntimeLifecycleNodeSubagent,
			Phase:       phase,
			SubjectID:   taskUpdated.TaskID,
			Description: taskUpdated.Patch.Description,
			Status:      status,
			Failed:      lifecycleStatusFailed(status),
		})
	}

	return events
}

func agentProgressLifecycle(progress *ToolProgressMessage) (RuntimeLifecycleEvent, bool) {
	data := lifecycleMap(progress.Additional["data"])
	if !strings.EqualFold(lifecycleMapString(data, "type"), "agent_progress") {
		return RuntimeLifecycleEvent{}, false
	}
	taskID := firstLifecycleValue(
		progress.TaskID,
		lifecycleMapString(data, "task_id", "taskId"),
		lifecycleMapString(data, "agent_id", "agentId"),
		progress.ToolUseID,
	)
	toolUseID := firstLifecycleValue(
		stringPointerValue(progress.ParentToolUseID),
		lifecycleMapString(data, "tool_use_id", "toolUseId"),
	)
	if taskID == "" || toolUseID == "" {
		return RuntimeLifecycleEvent{}, false
	}
	agentID := firstLifecycleValue(
		lifecycleMapString(data, "agent_id", "agentId"),
		taskID,
	)
	return RuntimeLifecycleEvent{
		NodeKind:       RuntimeLifecycleNodeSubagent,
		Phase:          RuntimeLifecycleStarted,
		SubjectID:      taskID,
		Name:           firstLifecycleValue(lifecycleMapString(data, "agent_type", "agentType"), "subagent"),
		Description:    lifecycleMapString(data, "description", "summary"),
		AgentID:        agentID,
		ChildSessionID: firstLifecycleValue(lifecycleMapString(data, "child_session_id", "childSessionId"), agentID),
		Status:         "running",
		Metadata: lifecycleMetadata(
			"tool_use_id", toolUseID,
			"last_tool_name", lifecycleMapString(data, "last_tool_name", "lastToolName"),
		),
	}, true
}

func structuredOutputSubagentLifecycle(
	attachment *AttachmentMessage,
) (RuntimeLifecycleEvent, bool) {
	if attachment == nil || !strings.EqualFold(strings.TrimSpace(attachment.Type), "structured_output") {
		return RuntimeLifecycleEvent{}, false
	}
	data := lifecycleMap(attachment.Data)
	agentID := lifecycleMapString(data, "agent_id", "agentId")
	taskID := firstLifecycleValue(
		lifecycleMapString(data, "task_id", "taskId"),
		agentID,
	)
	toolUseID := firstLifecycleValue(
		attachment.ToolUseID,
		lifecycleMapString(data, "tool_use_id", "toolUseId"),
	)
	if taskID == "" || toolUseID == "" {
		return RuntimeLifecycleEvent{}, false
	}
	status := normalizeLifecycleTerminalStatus(firstLifecycleValue(
		lifecycleMapString(data, "task_status", "taskStatus"),
		lifecycleMapString(data, "status"),
	))
	phase := RuntimeLifecycleProgress
	if lifecycleStatusTerminal(status) {
		phase = RuntimeLifecycleFinished
	}
	return RuntimeLifecycleEvent{
		NodeKind:       RuntimeLifecycleNodeSubagent,
		Phase:          phase,
		SubjectID:      taskID,
		Name:           lifecycleMapString(data, "agent_type", "agentType"),
		Description:    firstLifecycleValue(lifecycleMapString(data, "description"), lifecycleMapString(data, "summary")),
		AgentID:        firstLifecycleValue(agentID, taskID),
		ChildSessionID: firstLifecycleValue(lifecycleMapString(data, "child_session_id", "childSessionId"), agentID),
		Status:         status,
		Failed:         lifecycleStatusFailed(status),
		Metadata:       lifecycleMetadata("tool_use_id", toolUseID),
	}, true
}

func lifecycleMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func lifecycleMapString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}

func appendLifecycleEvent(events []RuntimeLifecycleEvent, message ReceivedMessage, event RuntimeLifecycleEvent) []RuntimeLifecycleEvent {
	event.SubjectID = strings.TrimSpace(event.SubjectID)
	if event.SubjectID == "" {
		return events
	}
	event.ParentSubjectID = strings.TrimSpace(event.ParentSubjectID)
	event.Name = strings.TrimSpace(event.Name)
	event.Description = strings.TrimSpace(event.Description)
	event.AgentID = strings.TrimSpace(event.AgentID)
	event.ChildSessionID = strings.TrimSpace(event.ChildSessionID)
	event.Status = strings.TrimSpace(event.Status)
	event.EventID = strings.Join([]string{
		firstLifecycleValue(message.UUID, message.SessionID, "runtime"),
		string(event.NodeKind),
		string(event.Phase),
		event.SubjectID,
		firstLifecycleValue(event.Status, "unknown"),
	}, ":")
	return append(events, event)
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
	switch status {
	case "succeeded", "failed", "cancelled", "interrupted":
		return true
	default:
		return false
	}
}

func lifecycleStatusFailed(status string) bool {
	switch status {
	case "failed", "cancelled", "interrupted":
		return true
	default:
		return false
	}
}
