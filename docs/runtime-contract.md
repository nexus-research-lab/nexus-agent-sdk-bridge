# Runtime Contract

## Boundary

`nexus-agent-sdk-bridge` is the open-source process and protocol boundary
between a Go host and an Agent runtime.

```text
Go host
  -> bridge client
       -> runtime process or host-managed transport
            -> stream-json messages and controls
```

The bridge owns process startup, transport lifecycle, typed messages, control
requests, hooks, permission callbacks, and in-process MCP integration. It does
not implement the agent loop, call model providers, or contain the source code
for the native `nxs` runtime.

## Runtime selection

| Runtime | Selection | Command resolution |
| --- | --- | --- |
| `nxs` | Default, or `WithRuntime(client.RuntimeNXS)` | Explicit `WithCLIPath` or `NEXUS_NXS_COMMAND_PATH` |
| Claude Code | `WithRuntime(client.RuntimeClaude)` | Explicit path, `NEXUS_CLAUDE_COMMAND_PATH`, or safe platform discovery |
| Direct connect | `WithDirectConnect(...)` | Host owns the remote runtime process |
| Custom transport | `WithTransport(...)` | Host owns the full connection |

The bridge does not download `nxs`, search application bundles, inspect caches,
or fall back to `PATH` for the native runtime. Official Nexus distributions
provide the closed-source `nxs` executable separately and pass its path to the
bridge.

## Wire format

Process runtimes exchange line-delimited JSON over stdio using the public
`stream-json` contract in `protocol/`. Native `nxs` and Claude Code share the
same mixed-casing control wire.

Compatibility aliases are declared field by field where runtime payloads
differ. The bridge never applies a global snake_case or camelCase conversion to
control messages, tool arguments, hook inputs, or provider payloads.

## Capability negotiation

Hosts must call `Session.Supports` before exposing optional controls. Common
capabilities include typed usage, terminal categories, task stopping,
in-process MCP, exact-boundary session forking, and provider-neutral runtime
lifecycle events. Native-only controls currently include task follow-up,
environment hot updates, and AutoDream. Hook response acknowledgement is
negotiated during initialization. Native runtimes may also negotiate
`CapabilityMessageExecutionPolicy`; this allows a host to apply
`tool_access=none`, `max_output_tokens`, and `skip_auto_memory` to one message
without changing the Agent's persistent configuration.

`SetNextTurnContext` orders internal context blocks by descending priority and
then by name, content, and metadata. NXS extracts the resulting hidden reminder
before persisting the user message, keeps it in the live model history, and does
not write it to the transcript. Claude Code's public stdin accepts only user and
control messages, so the bridge returns the context through its native
`UserPromptSubmit` hook and lets Claude Code create the attachment.

Capability values are defined in [`client/capability.go`](../client/capability.go).
Runtime names are not a substitute for capability checks.

## Session lifecycle

1. `client.NewSession` resolves a transport and initializes the runtime.
2. `Session.Send` or `SendWithOptions` starts a turn.
3. The host consumes typed messages with `Recv` or waits for `Result`.
4. Controls use the active session and preserve the runtime request identity.
5. `Session.Close` releases the transport and owned process resources.

`client.ForkSession` creates an independent target from a source session through
the exact supplied message ID. Claude Code may not persist the target transcript
until its first user turn, but the bridge assigns the target session ID before
returning the session.

Context cancellation and user aborts remain distinguishable through Go error
matching. Long-running controls propagate cancellation with the same
`request_id`, allowing the runtime to cancel the original operation.

## Security responsibilities

- The host decides which runtime executable and environment to trust.
- The bridge passes sandbox policy but does not claim to enforce a sandbox.
- Direct process signals assume the host and runtime share an OS identity. A
  host that crosses this boundary must provide `WithProcessSignalHandler` and
  validate PID ownership before forwarding lifecycle signals.
- Generated or inline MCP configuration is materialized in restricted argument
  files rather than exposed directly in process arguments.
- Provider credentials remain runtime process environment values; the bridge
  does not interpret provider request payloads.

Runtime enforcement and product authorization remain outside this library.

## Compatibility policy

Public Go packages follow repository releases. `internal/` packages are not
supported imports. New runtime-specific behavior must first receive a public
capability and a typed protocol shape; hosts should not branch on undocumented
payload fields.

## nxs 自动权限审核

产品预设 `permission_mode=auto` 组合基础权限检查与独立模型审核，不等同于 `acceptEdits` 或完全访问。静态 deny 优先；显式 ask、用户提问、配置授权与尚未支持审核的工具保留人工处理。当前审核覆盖 Bash、PowerShell、Read、Write、Edit、NotebookEdit、Glob、Grep、WebFetch、WebSearch 的未决请求；MCP、领域命令和子 Agent 授权仍走宿主。

审核由 SDK runtime 使用当前 Provider/模型发起独立无工具请求。输入只保留真人文字、历史工具调用参数、已加载的项目规范与准确操作参数。工具返回、助手自述、思考和 synthetic/meta 消息不进入审核；项目规范只提供约束，不替代真人授权。规则禁止高风险自动允许。只有低风险且授权中/高，或中风险且授权高的结构化结果可以一次性放行；不会改写参数或保存 allow 规则。显式破坏性警告直接转人工。

审核超时为 45 秒。模型失败、无效输出、缺少上下文或证据超过 128 KiB 时转人工；取消或审核期间权限变化不能自动放行。审核请求和耗时复用 nxs 现有 classifier 诊断状态。审核模型不能调用工具；需要额外环境检查时必须转人工，不推测检查结果。未配置人工回调时，auto 模式拒绝未获批准的操作。

`initialize.protocol_capabilities` 必须协商 `auto_review_v1`。bridge 在连接与运行中切换 auto 时检查该能力；旧 nxs 或未声明能力的运行时明确报错，不静默降级。原有 `permission_mode` 只作为一个产品预设，界面不另增审核开关。

审核完成发出 `system/permission_review`，包括 `tool_use_id`、`tool_name` 与 `review`（status、risk、authorization、rationale）。需要人工时，同一结构同时进入 `can_use_tool.review`，原因进入 `decision_reason`，`requires_human` 禁止代替真人确认。宿主继续拥有身份、业务授权、任务持久审批和恢复；自动审核不授予这些领域权限。

## Claude 原生自动权限审核

Claude 的 `permission_mode=auto` 直接交给 Claude Code，自行使用其分类器、缓存与拒绝策略，不要求 nxs 专用 `auto_review_v1`，也不执行第二层 nxs 审核。`CapabilityAutoReview` 在 Claude 上表示已适配原生控制接口，不保证账号、模型或组织策略允许启用。启动和动态切换均要求 `set_permission_mode` 明确回复 `mode=auto`；错误、空确认或降级模式返回错误，启动失败关闭连接。

Claude 的未通过请求可能被拒绝并交给模型尝试替代方案，非交互回退也可能结束执行；现有 `can_use_tool` 人工回调保持不变。宿主不得假定 Claude 会发出 nxs 的审核事件或包含其审核证据。支持情况服从 Claude 当前版本、模型、服务可用性与组织设置。
