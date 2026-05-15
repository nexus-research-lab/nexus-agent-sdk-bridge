package tools

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

// FileReadEvent 描述一次成功的文本文件读取。
type FileReadEvent struct {
	Content    string
	FilePath   string
	Metadata   map[string]any
	NumLines   int
	StartLine  int
	ToolName   string
	ToolUseID  string
	TotalLines int
}

// FileReadHandler 接收文本文件读取事件。
type FileReadHandler func(context.Context, FileReadEvent)

// GitCommitOperation 描述一次本地 git commit。
type GitCommitOperation struct {
	Kind string
	SHA  string
}

// GitPushOperation 描述一次本地 git push。
type GitPushOperation struct {
	Branch string
}

// GitBranchOperation 描述一次本地分支操作。
type GitBranchOperation struct {
	Action string
	Ref    string
}

// GitPullRequestOperation 描述一次 pull request 操作。
type GitPullRequestOperation struct {
	Action     string
	Number     int
	Repository string
	URL        string
}

// GitOperationEvent 描述一次 shell 中识别到的 git 操作。
type GitOperationEvent struct {
	Branch           *GitBranchOperation
	Command          string
	Commit           *GitCommitOperation
	Operations       []string
	PR               *GitPullRequestOperation
	Push             *GitPushOperation
	Shell            string
	ToolName         string
	ToolUseID        string
	WorkingDirectory string
}

// GitOperationHandler 接收 git 操作事件。
type GitOperationHandler func(context.Context, GitOperationEvent)

// HookEvaluationRequest 描述交给宿主模型执行的 prompt / agent hook 评估请求。
type HookEvaluationRequest struct {
	AgentName string
	Event     hook.Event
	HookName  string
	Input     map[string]any
	InputJSON string
	Kind      string
	Model     string
	PluginID  string
	Prompt    string
	RawPrompt string
	Timeout   time.Duration
	ToolName  string
	ToolUseID string
}

// HookEvaluationResult 表示 prompt / agent hook 的模型可见输出。
type HookEvaluationResult struct {
	Cancelled bool
	Output    string
}

// HookEvaluationRunner 执行宿主模型 hook 评估。
type HookEvaluationRunner func(context.Context, HookEvaluationRequest) (HookEvaluationResult, error)

// AsyncCommandHookEvent 描述 async / asyncRewake command hook 完成事件。
type AsyncCommandHookEvent struct {
	AsyncRewake   bool
	BlockingError string
	Command       string
	DurationMS    int64
	Event         hook.Event
	ExitCode      int
	HookName      string
	PluginID      string
	Stderr        string
	Stdout        string
	Succeeded     bool
	ToolName      string
	ToolUseID     string
}

// AsyncCommandHookHandler 接收异步命令 hook 完成事件。
type AsyncCommandHookHandler func(context.Context, AsyncCommandHookEvent)

// CronFireEvent 描述一次 cron 到期触发。
type CronFireEvent struct {
	AgentID     string
	CreatedAt   int64
	Cron        string
	Durable     bool
	FiredAt     int64
	ID          string
	LastFiredAt int64
	Permanent   bool
	Prompt      string
	Recurring   bool
	ScheduledAt int64
}

// CronFireHandler 接收 cron 到期事件。
type CronFireHandler func(context.Context, CronFireEvent)

// TeammateSpawnRequest 描述 AgentTool 队友启动请求。
type TeammateSpawnRequest struct {
	AgentType        string
	CWD              string
	Description      string
	Model            string
	Name             string
	PlanModeRequired bool
	Prompt           string
	Team             map[string]any
	TeamFilePath     string
	TeamName         string
	ToolUseID        string
	WorkingDirectory string
}

// TeammateSpawnResult 表示队友启动结果。
type TeammateSpawnResult struct {
	AgentID          string
	AgentType        string
	Color            string
	IsSplitPane      bool
	Message          string
	Metadata         map[string]any
	Model            string
	Name             string
	PlanModeRequired bool
	Task             map[string]any
	Team             map[string]any
	TeamName         string
	TeammateID       string
	TmuxPaneID       string
	TmuxSessionName  string
	TmuxWindowName   string
}

// TeammateSpawnRunner 让宿主启动本地队友。
type TeammateSpawnRunner func(context.Context, TeammateSpawnRequest) (TeammateSpawnResult, error)

// PeerInfo 描述一个可消息投递的本地 peer。
type PeerInfo struct {
	Address string
	CWD     string
	Name    string
	PID     int
}

// ListPeersRequest 描述 peer 发现请求。
type ListPeersRequest struct {
	AgentName           string
	AssistantUUID       string
	IncludeSelf         bool
	MessagingSocketPath string
	ProcessID           int
	ToolUseID           string
	WorkingDirectory    string
}

// ListPeersResult 表示 peer 发现结果。
type ListPeersResult struct {
	Metadata map[string]any
	Peers    []PeerInfo
}

// ListPeersDiscoverer 发现本地或 bridge peer。
type ListPeersDiscoverer func(context.Context, ListPeersRequest) (ListPeersResult, error)

// PushNotificationRequest 描述宿主通知请求。
type PushNotificationRequest struct {
	AgentName        string
	AssistantUUID    string
	Body             string
	Priority         string
	TeamName         string
	Title            string
	ToolUseID        string
	WorkingDirectory string
}

// PushNotificationResult 表示宿主通知结果。
type PushNotificationResult struct {
	Message  string
	Metadata map[string]any
	Sent     bool
}

// PushNotificationHandler 接收宿主通知请求。
type PushNotificationHandler func(context.Context, PushNotificationRequest) (PushNotificationResult, error)

// RemoteTriggerRequest 描述远端 trigger 请求。
type RemoteTriggerRequest struct {
	Action           string
	AgentName        string
	AssistantUUID    string
	Body             map[string]any
	TeamName         string
	ToolUseID        string
	TriggerID        string
	WorkingDirectory string
}

// RemoteTriggerResult 表示远端 trigger 结果。
type RemoteTriggerResult struct {
	AuditID  string
	JSON     string
	Metadata map[string]any
	Status   int
}

// RemoteTriggerHandler 执行远端 trigger 请求。
type RemoteTriggerHandler func(context.Context, RemoteTriggerRequest) (RemoteTriggerResult, error)

// REPLRequest 描述一次宿主 REPL 执行请求。
type REPLRequest struct {
	AgentName        string
	AssistantUUID    string
	Code             string
	TeamName         string
	ToolUseID        string
	WorkingDirectory string
}

// REPLResult 表示宿主 REPL 执行结果。
type REPLResult struct {
	Metadata  map[string]any
	Result    string
	ToolCalls int
}

// REPLRunner 执行宿主 REPL 请求。
type REPLRunner func(context.Context, REPLRequest) (REPLResult, error)

// SubscribePRRequest 描述 pull request 订阅请求。
type SubscribePRRequest struct {
	AgentName        string
	AssistantUUID    string
	Events           []string
	PRNumber         int
	Repo             string
	TeamName         string
	ToolUseID        string
	WorkingDirectory string
}

// SubscribePRResult 表示 pull request 订阅结果。
type SubscribePRResult struct {
	Metadata       map[string]any
	Subscribed     bool
	SubscriptionID string
}

// SubscribePRHandler 执行 pull request 订阅请求。
type SubscribePRHandler func(context.Context, SubscribePRRequest) (SubscribePRResult, error)

// SuggestBackgroundPRRequest 描述后台 PR 建议请求。
type SuggestBackgroundPRRequest struct {
	AgentName        string
	AssistantUUID    string
	Branch           string
	Description      string
	TeamName         string
	Title            string
	ToolUseID        string
	WorkingDirectory string
}

// SuggestBackgroundPRResult 表示后台 PR 建议结果。
type SuggestBackgroundPRResult struct {
	Metadata     map[string]any
	Suggested    bool
	SuggestionID string
}

// SuggestBackgroundPRHandler 执行后台 PR 建议请求。
type SuggestBackgroundPRHandler func(context.Context, SuggestBackgroundPRRequest) (SuggestBackgroundPRResult, error)

// TerminalCaptureRequest 描述终端截取请求。
type TerminalCaptureRequest struct {
	AgentName        string
	AssistantUUID    string
	Lines            int
	PanelID          string
	TeamName         string
	ToolUseID        string
	WorkingDirectory string
}

// TerminalCaptureResult 表示终端截取结果。
type TerminalCaptureResult struct {
	Content   string
	LineCount int
	Metadata  map[string]any
}

// TerminalCaptureHandler 执行终端截取请求。
type TerminalCaptureHandler func(context.Context, TerminalCaptureRequest) (TerminalCaptureResult, error)
