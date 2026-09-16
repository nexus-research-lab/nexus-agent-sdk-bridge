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
5. `Session.Close` waits for both transport `Close` and `Wait`, then the read loop, before completing cleanup. A failed termination attempt does not prove process exit. Caller cancellation stops only that caller's wait; shared cleanup remains pending. Close and exit diagnostics are both preserved.

Process cleanup failures are returned as `client.ProcessCleanupError`, matchable
with `errors.As`. They survive a successful main-process exit, forced shutdown,
and repeated process `Close` calls. The Unix sweep performs a final observation
after bounded signaling and rejects observation errors or remaining visible
members. It ignores only processes that disappeared during enumeration.

This is a session sweep, not a complete process-tree receipt. Descendants can
create another Unix session; Linux PID namespaces and `/proc` visibility also
bound the observation. A host signal callback supplies its own cleanup semantics;
its success is not independent exit verification by Bridge. Native Windows,
process identity reuse, crash recovery and private scratch ownership need separate
host/platform lifecycle guarantees. Hosts must retain a failed cleanup boundary
instead of treating a closed stream as proof that resources can be reused.

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

## Subagent control

`CapabilitySubagentControl` requires the initialize response to acknowledge
`subagent_control_v1`; runtime kind alone is insufficient. `Session.Control().ControlSubagent`
sends a `subagent_control` request containing `tool_use_id`, `operation` and `input`.
The caller identity must be taken from the current SDK MCP call metadata
(`claudecode/toolUseId`), never from model business input. Operations are spawn,
list, get, wait, send and stop; operation input semantics belong to the runtime.

The runtime validates the active parent-session call and retains native permission
and task hooks. Spawn returns an asynchronous child identity; acceptance is not
completion. The host must service incoming hooks while awaiting the control
response. Context cancellation sends cancellation for that exact request ID; it
must not trigger automatic replay of a spawn whose result is unknown. The bridge
does not add an Agent tool, CLI shim, durable mutation receipt or recursive child
capability. Older nxs versions and Claude Code report unsupported capability.

## nxs 自动权限审核

产品预设 `permission_mode=auto` 组合基础权限检查与独立模型审核，不等同于 `acceptEdits` 或完全访问。静态 deny 优先；显式 ask、用户提问、配置授权与尚未支持审核的工具保留人工处理。当前审核覆盖 Bash、PowerShell、Read、Write、Edit、NotebookEdit、Glob、Grep、WebFetch、WebSearch 的未决请求；MCP、领域命令和子 Agent 授权仍走宿主。

审核由 SDK runtime 使用当前 Provider/模型发起独立无工具请求。输入只保留真人文字、历史工具调用参数、已加载的项目规范与准确操作参数。工具返回、助手自述、思考和 synthetic/meta 消息不进入审核；项目规范只提供约束，不替代真人授权。规则禁止高风险自动允许。只有低风险且授权中/高，或中风险且授权高的结构化结果可以一次性放行；不会改写参数或保存 allow 规则。显式破坏性警告直接转人工。

审核超时为 45 秒。模型失败、无效输出、缺少上下文或证据超过 128 KiB 时转人工；取消或审核期间权限变化不能自动放行。审核请求和耗时复用 nxs 现有 classifier 诊断状态。审核模型不能调用工具；需要额外环境检查时必须转人工，不推测检查结果。未配置人工回调时，auto 模式拒绝未获批准的操作。

`initialize.protocol_capabilities` 必须协商 `auto_review_v1`。bridge 在连接与运行中切换 auto 时检查该能力；旧 nxs 或未声明能力的运行时明确报错，不静默降级。原有 `permission_mode` 只作为一个产品预设，界面不另增审核开关。

审核完成发出 `system/permission_review`，包括 `tool_use_id`、`tool_name` 与 `review`（status、risk、authorization、rationale）。需要人工时，同一结构同时进入 `can_use_tool.review`，原因进入 `decision_reason`，`requires_human` 禁止代替真人确认。宿主继续拥有身份、业务授权、任务持久审批和恢复；自动审核不授予这些领域权限。

## Claude 原生自动权限审核

Claude 的 `permission_mode=auto` 直接交给 Claude Code，自行使用其分类器、缓存与拒绝策略，不要求 nxs 专用 `auto_review_v1`，也不执行第二层 nxs 审核。`CapabilityAutoReview` 在 Claude 上表示已适配原生控制接口，不保证账号、模型或组织策略允许启用。启动和动态切换均要求 `set_permission_mode` 明确回复 `mode=auto`；错误、空确认或降级模式返回错误，启动失败关闭连接。

Claude 的未通过请求可能被拒绝并交给模型尝试替代方案，非交互回退也可能结束执行；现有 `can_use_tool` 人工回调保持不变。宿主不得假定 Claude 会发出 nxs 的审核事件或包含其审核证据。支持情况服从 Claude 当前版本、模型、服务可用性与组织设置。

## Required sandbox execution

`SandboxSettings.RequireSandbox=true` requires nxs capability `required_sandbox_v1`.
Bridge sends boolean `required_sandbox` on initialize, independently of ordinary
settings. nxs validates the type and capability before creating its SDK session,
then installs the host requirement and acknowledges the capability. Missing
acknowledgement disconnects Bridge before ConnectWithPrompt sends its user message.
Claude is rejected before starting transport for this option.

This is an execution contract: exclusions and unavailable backends cannot silently
select direct execution. It is not a claim of Windows backend availability, complete
filesystem/network policy ownership, or live permission-mode/sandbox synchronization.
The option defaults to false. Mode changes do not clear the requirement; a new
session with a changed effective policy is currently required.

Required sandbox admission also gates raw and internal continuation writes. A transport
being connected is not enough: concurrent sends during initialization fail until
the current connection acknowledges the capability, without consuming queued context.

For required execution, initialize includes object `sandbox_policy` containing the
explicit host Sandbox settings. nxs rejects malformed objects before session creation;
ordinary inline/project settings do not expand required-mode resource grants.

`Session.Reconfigure` returns `ErrRestartRequired` with reason `sandbox_policy_changed`
when Sandbox settings change in either direction. It applies no other hot controls
and preserves current options. The host must retire/drain the old process and create
a new session; changing permission mode alone is not a sandbox policy transition.

## Native file-tool confinement

`SandboxSettings.RequireFileTools=true` additionally requires `RequireSandbox=true` and nxs capability `sandbox_file_tools_v1`. Bridge sends boolean `required_sandbox_file_tools` in initialize; it is not an inline sandbox setting. Invalid combinations fail before starting transport. Missing either required acknowledgement disconnects before the first task, and raw/internal/concurrent task sends use the same gate. Changing this requirement requires process replacement.

The capability covers the built-in Read/Write/Edit file execution boundary, including content, directory suggestions, link/metadata and freshness reads. Preparation or execution failure must not fall back to direct host IO. Current nxs declares it only on macOS. It does not prove current dependencies, effective policy, whole-process confinement, Glob/Grep, Notebook, startup settings, Skills, memory or external MCP coverage.

An old host can still request only `required_sandbox_v1`; a host requiring native file confinement must request both. `Session.Supports(CapabilitySandboxFileTools)` reports the current nxs acknowledgement, not a user permission or an activation switch. Claude Code does not implement this nxs extension; its native sandbox remains a separate adapter contract.

## Skill file confinement

`SandboxSettings.RequireSkillFiles=true` requires `RequireSandbox` and `RequireFileTools`. Bridge sends `required_sandbox_skill_files` only in initialize and requires the separate nxs `sandbox_skill_files_v1` acknowledgement before every task write. Invalid combinations fail before transport startup; missing acknowledgement disconnects without a prompt. Ordinary settings and Extra cannot inject this requirement. Changing it requires process replacement.

Current macOS nxs routes Skill catalogs, bodies, dynamic discovery, Git ignore checks and remember-availability settings through the same file boundary. Dynamic discovery captures the triggering Read context and cwd; allowed sources, conditional activation and Git ignore behavior remain supported. Git receives a minimal environment with network denied. Denial, cancellation and unknown command results do not fall back to host IO; unknown observations can be checked again on a later file access. This contract does not cover all startup settings, hooks, background memory IO, effective-policy receipts, other platforms or Claude native sandbox behavior.

## Local media-file confinement

`SandboxSettings.RequireMediaFiles=true` requires both `RequireSandbox` and `RequireFileTools`. Bridge sends `required_sandbox_media_files` only in initialize, then checks the separate nxs `sandbox_media_files_v1` acknowledgement before all task writes. Invalid combinations fail before transport startup; missing acknowledgement disconnects before a prompt. Ordinary settings and Extra cannot inject the requirement, and changing it requires process replacement.

The current macOS contract covers local image reads by ViewImage and main-model preprocessing, including paths, file URLs, symlinks, deferred references and nested tool-result images. File-executor preparation and reads cannot fall back; access is checked before cached analysis. Old command/file/search capabilities cannot imply this coverage. Remote media networking, effective-policy receipts, whole-SDK IO, other platforms and Claude native sandbox adaptation remain separate.

## Search-tool confinement

`SandboxSettings.RequireSearchTools=true` additionally requires both `RequireSandbox` and `RequireFileTools`. Bridge sends `required_sandbox_search_tools` only in initialize and requires the separate nxs `sandbox_search_tools_v1` acknowledgement before all task writes. Ordinary settings and Extra cannot insert the host requirement. Invalid combinations fail before transport startup; changes require process replacement. An older runtime can support commands and Read/Write/Edit while lacking search confinement, so those capabilities cannot substitute for this acknowledgement.

Current macOS nxs routes Glob/Grep path checks, missing-path suggestions, rg and result metadata through the file execution boundary. Search auxiliaries use a minimal environment, deny network and inherit the same resource policy. Preparation, cancellation, output limits and execution failure must not cause host IO fallback, replay or partial success; single-file content/count results preserve filenames. This contract does not establish current effective policy, whole-SDK IO, detached-process cleanup, scratch reclamation, other platforms or Claude native sandbox support.

## Host resource write scope

`SandboxSettings.Resources` uses `SandboxResourcePolicy` (`version`, `write_scope`, `scratch_root`) and additionally requires `sandbox_resources_v1`. Bridge sends it as `required_sandbox_resources` outside ordinary `sandbox_policy`, requires both command and file contracts, and checks all three acknowledgements before any task write. `Resources` and Extra cannot insert the object into inline settings. Cloned options own their policy value; changing it requires process replacement.

Version 1 supports `read-only` or `workspace-write` and an explicit absolute private scratch directory. Read-only rejects additional write grants; both scopes reject explicit unsandboxed command permission. Current nxs supports these modes only on macOS and rejects malformed, unsupported or overlapping directory inputs before session initialization. Within the covered command/file executors, ambient HOME/TMPDIR and legacy workspace/cache write grants cannot expand the selected scope.

The host prepares, owns and reclaims scratch after execution and descendants end; Bridge does not create a lease or prove privacy. This is not a whole-SDK IO or effective-policy receipt guarantee. Ordinary read/deny and network settings retain their separate contracts. Claude native integration, unsupported platforms and host product defaults remain separate acceptance work. Omitting Resources preserves the existing contract. A session rejected before startup can still finish cleanup; active transport and read-loop termination remain required once startup has been admitted.

## Context file confinement

`SandboxSettings.RequireContextFiles=true` requires the command and file contracts and a separate nxs `sandbox_context_files_v1` acknowledgement before task writes. It covers startup/dynamic instructions, instruction exclusion settings, and compact file restoration. Missing acknowledgement disconnects without sending a prompt; changed requirements replace the process. The host-only option cannot enter ordinary settings through Extra.

Unreadable exclusion settings prevent startup/reload; failed reloads clear stale instructions and block subsequent model requests until reading recovers. Global permission/provider configuration, project definitions, hooks, persistence and background IO remain separate. Claude native adaptation does not advertise this capability.
