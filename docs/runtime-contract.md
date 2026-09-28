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
its success is not independent exit verification by Bridge. On Windows, the
bridge creates the runtime with `CREATE_SUSPENDED`, assigns a per-runtime Job
Object with `KILL_ON_JOB_CLOSE`, then validates the initial thread's owner and
resumes it. The runtime cannot execute entry code or create early descendants
before assignment. Creation, assignment, thread lookup or resume failure rejects
startup. Cleanup waits for the Job to become empty, and closing the owning bridge
process kills admitted members. The Windows 11 amd64 native suite verifies immediate
descendants, cancellation with inherited output pipes and forced host termination.

CLI help/settings/restricted/version probes use the same process boundary.
Cancellation cleans descendants before waiting for exit, and post-exit pipe waits
are bounded; inherited pipes cannot turn a successful parent exit into an indefinite
admission wait. Version probes use the same scrubbed environment and bounded output
as capability probes.

Creation and assignment are still two OS operations: a bridge crash before Job
assignment can leave a suspended runtime, although its entry code has not executed.
This is not an atomic creation receipt or proof of private scratch recovery, identity
reuse safety, file/network confinement or clean-host installation acceptance. Hosts
must retain a failed cleanup boundary instead of treating a closed stream as proof
that resources can be reused.

The internal `processscope` package is an unconnected macOS supervision component,
not a public capability or a stronger default `Close` guarantee. It registers a
distinct same-user resource coalition before execution, binds it to the boot UUID,
and signals members using audited PID versions. `CapturePeer` binds registration
to the Unix control connection kernel audit token and a trusted launcher-supplied
PID; an exact token mismatch (including PID reuse) rejects admission. The connection
remains owned by the caller. This does not authenticate the launcher job or executable.
The bootstrap must wait for durable registration before receiving permission to run.
Restore requires an authenticated
original registration; arbitrary caller-provided coalition IDs are not authority.
Cleanup succeeds only after kernel coalition retirement or a changed boot UUID.
Empty enumeration, matching counters and successful launchd bootout are insufficient.
The caller must revoke launch rights before reaping. Native API absence fails
unavailable; initial macOS 14.0 lacks the required signal API. Trusted bootstrap,
durable host binding, default transport integration and scratch recovery remain
unimplemented by this component.

`cmd/nexus-runtime-bootstrap` and internal `processbootstrap` supply the receiving
half of the launch protocol. The helper authenticates the fixed host audit token,
waits with a deadline for one bounded request and exactly three directional pipes,
and uses in-place exec with explicit argv/environment/cwd. Neither the task nor
Provider credentials belong in the launchd plist. The control socket and received
source descriptors are close-on-exec; only standard descriptors are duplicated for
the runtime. Missing native support or admission failure must terminate the helper
process, never retry or fall back to ordinary execution. Ancillary storage covers
XNU's maximum descriptor count; an unexpected control truncation is fatal to the
helper, which cannot be reused. The host must still verify its job/helper identity,
persist the original scope before sending, bound writes by the startup context,
observe exit, revoke launch rights, and reap before resource recovery. This helper
is not shipped or used by the default transport yet.

Internal `Scope.WatchRoot` binds kqueue `NOTE_EXIT | NOTE_EXITSTATUS` before
admission, verifying the same control-peer identity and registered scope before
and after attachment. It preserves normal exit codes and termination signals
across in-place exec. Waiting is shared; canceling a caller only ends that wait.
Closing the observer releases its descriptor and returns an explicit stopped
observation to waiters, without terminating the process or creating exit evidence.
The descriptor is close-on-exec with Go fork coordination. Root exit never substitutes
for coalition reaping: a native fixture observes normal root exit while its detached
child remains alive. Restart recovery still needs durable scope registration and
independent reaping; an exit observer cannot reconstruct a lost historical exit code.

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

## Claude native restricted launch

Desktop hosts use `SandboxSettings.RequireClaudeNativeSandbox=true` for the
default restricted Claude path. Bridge generates a single structured
`--settings` object with `sandbox.enabled=true`,
`sandbox.failIfUnavailable=true`, and
`sandbox.allowUnsandboxedCommands=false`, and exposes
`CapabilityClaudeNativeSandbox`. Claude therefore keeps Bash/build tools while
its own native OS backend restricts command, file, and network access. Bridge
rejects missing, duplicated, or `ExtraArgs`-overridden settings before starting
the stream-json process, then runs a bounded no-model
`--settings <generated-json> --help` probe against the exact resolved CLI. The
probe only checks that the selected CLI accepts and advertises the settings
entry point; Claude remains authoritative for native OS initialization and
command enforcement. Nexus supplies the network grant; an empty grant is
serialized as deny-all. This capability is separate from nxs
`required_sandbox_v1`, and settings admission is not an effective-policy
receipt: real allow/deny, cancellation, descendant cleanup, and platform
evidence remain separate acceptance requirements.

The older `RequireClaudeRestricted=true` contract still means Claude's
`--restricted` tool-removal mode. That mode removes Bash and other code-running
tools and cannot replace the desktop command sandbox; the two contracts cannot
be enabled together. Full Access installs neither native contract and remains
subject to host lifecycle and domain authorization.

`SandboxSettings.RequireClaudeRestricted=true` is the typed Bridge contract for
Claude Code's native `--restricted` mode. It is valid only with
`RuntimeClaude`; Bridge adds exactly one `--restricted` argument to the Claude
process launch and exposes `CapabilityClaudeRestricted` for that session. This
is a separate adapter and does not send or claim nxs `required_sandbox_v1`.

The option rejects `RuntimeNXS`, bypass permissions, and untyped `ExtraArgs` or
`ExtraBoolArgs` attempts to inject the flag before transport startup. A change
to this requirement returns `sandbox_policy_changed` and requires a new
runtime process. Full Access (for example `ModeBypassPermissions`) does not
require this contract and does not receive `--restricted`.

The bridge verifies only the typed launch contract and the argument it passes
to the selected executable. The executable must support `--restricted`; if an
older or substituted Claude command rejects that flag, startup fails closed.
Before the stream-json process is admitted, a restricted session runs the exact
resolved executable (including the safe Windows PowerShell shim) with
`--restricted --help`. The bounded probe must exit successfully and advertise
the flag; probe output is capped and common Provider, proxy, cookie, and other
secret variables are removed from its environment. Probe success is only a
parser/launch check and is not a runtime enforcement receipt.
This does not prove the CLI version, OS enforcement, provider/network policy,
or complete Claude SDK file, settings, hook, MCP, transcript, and background IO
isolation. Hosts must retain those as separate acceptance evidence.

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

## Remote image networking

`SandboxSettings.RequireMediaNetwork=true` requires command, file and local media requirements. Bridge sends `required_sandbox_media_network` only in initialize and checks the independent nxs `sandbox_media_network_v1` acknowledgement before any prompt. Missing prerequisites fail before transport startup; changing the requirement replaces the runtime. Ordinary settings cannot supply it.

Current macOS nxs downloads remote images through the captured network policy before passing bytes to either Provider, including deferred references and nested tool results. Every request and redirect is checked; deny wins, managed-only policy cannot be expanded by approval, and environment proxies/Provider credentials are not inherited. ViewImage approval binds the exact input, tool-use, cwd, destination and permission epoch. Preprocessing without a tool identity uses existing grants or an explicit host callback. Permission changes and cleanup cancel pending approval and body reads. This contract does not cover Provider transport, other auxiliary networking, external MCP or Claude.

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

## Project definition file confinement

`SandboxSettings.RequireProjectFiles=true` requires the command and file contracts and a separate nxs `sandbox_project_files_v1` acknowledgement before task writes. It covers startup and explicit refresh of project Agent/command/Skill definitions and selected hook-setting files. Read failure prevents execution; changed Agent/hook bindings require a new runtime. The host-only requirement participates in process replacement and cannot be injected through ordinary settings. Global permission/provider settings, persistence, hook execution and other backends remain outside this capability.

## Managed policy integrity

`SandboxSettings.RequireManagedPolicy=true` requires both the command and file contracts and a separate nxs `sandbox_managed_policy_v1` acknowledgement before any task write. It is sent only as initialize `required_sandbox_managed_policy`, excluded from ordinary settings, and participates in process replacement.

The runtime fixes the managed-policy source before settings environment projection, keeps an immutable snapshot, rejects invalid managed files and checks effective policy integrity before query, compact, tool dispatch and permission updates. Changed policy requires runtime recreation; restoring the original effective content permits recovery. Required execution excludes task settings before reading them. This capability currently acknowledges the verified macOS implementation. Ordinary settings, credentials, persistence, other platforms and Claude native adaptation remain separate contracts.


## Ordinary settings files and snapshots

`SandboxSettings.RequireSettingsFiles=true` requires the command and file contracts and a separate nxs `sandbox_settings_files_v1` acknowledgement before task admission. It travels only as initialize `required_sandbox_settings_files`, is excluded from ordinary settings, and participates in process replacement. Claude cannot advertise this nxs capability.

The runtime fixes config roots and selected sources before settings profile projection. Required execution reads ordinary files through the confined worker, rejects incomplete/invalid snapshots, and checks source integrity before query, compact, tools and configuration controls. Disabled sources are filtered before IO. Children keep independent bound snapshots; external changes require runtime recreation or restoration of the original content. Dynamic updates reject static execution fields instead of reporting unapplied changes as successful; get_settings uses the bound snapshot.

The acknowledgement currently covers verified native macOS. Provider credential separation, rooted atomic permission writes, cross-process concurrency, durable approval/receipts, background IO and other platforms remain separate contracts. Snapshot checks do not undo side effects or establish a complete persistence transaction.

## Ordinary settings writes

`SandboxSettings.RequireSettingsWrites=true` requires `RequireSandbox`, `RequireFileTools` and `RequireSettingsFiles`. Bridge sends it only as initialize boolean `required_sandbox_settings_writes`, independently of ordinary `sandbox_policy`, and requires all four nxs acknowledgements: `required_sandbox_v1`, `sandbox_file_tools_v1`, `sandbox_settings_files_v1` and `sandbox_settings_writes_v1`. Invalid combinations fail before transport startup. A missing acknowledgement disconnects before Bridge exposes the runtime Session or sends any user, raw or internal task message. The host-only option participates in process replacement and the restart-sensitive options fingerprint. Claude cannot advertise or require this nxs extension; older nxs versions fail closed.

The current native macOS nxs contract routes Config and explicit permission persistence through the same startup-bound settings store. A file-form `--settings` is the Config target; otherwise Config uses user settings, and an inline flag falls back to user settings. No writable source is an error. Config reads the selected document value rather than claiming the layered effective value, and writes the canonical nested settings key while preserving unrelated and legacy fields. Explicit SDK Options and process environment still have higher precedence, so persisted settings are defaults for the next runtime rather than proof of its final effective value. Before a write, nxs plans the full layered snapshot and rejects a value still overridden by a higher-priority project, local, flag or managed-policy source. An actual Config change reports `runtimeRestartRequired=true` and blocks further execution in that runtime; a no-op does not require recreation.

The physical writer binds directory identities, rejects later link, directory-generation and special-file substitutions, and protects lexical and resolved physical aliases of both the target and its random temporary-entry pattern from sandboxed tasks. Required mode rejects an existing settings file with multiple hard links because a path-based sandbox cannot enumerate every alias. It syncs and keeps the temporary descriptor open through an identity-checked same-directory replacement. One document is atomically visible; permission updates that span documents are ordered and poison the shared store as unknown after a partial commit. Runtime clones keep independent logical snapshots while sharing the write lock, directory generations, unknown result and recreation gate.

This capability does not guarantee concurrency against another unsandboxed same-UID process, a cross-process lock or CAS, all-or-nothing multi-document commits, parent-directory fsync or power-loss durability. It also does not provide durable request/approval/revision binding, a durable receipt, restart reconciliation of unknown outcomes or automatic replay. Unix replacement preserves ordinary permission bits but makes no owner, ACL, xattr or file-flag claim. Provider credentials, task environments and all other SDK IO remain separate boundaries. Windows currently has cross-compilation evidence only; its Go writable-bit checks do not establish DACL privacy, and it is outside native acceptance.

SDK-hosted MCP call context: `params._meta["claudecode/toolUseId"]` is propagated unchanged to `tools.Context.ToolUseID`, for nxs and compatible runtimes using this existing wire field. Each call receives isolated metadata. SessionID/RoundID are not inferred. Missing metadata does not inherit a parent call identity.

### Remote MCP networking

RequireMCPNetwork / sandbox_mcp_network_v1 requires an explicit MCP configuration (MCP.StrictConfig), mandatory sandboxing, and negotiated nxs support. Configured HTTP and legacy SSE servers receive only their own scheme/host/port grant; redirects and SSE POST endpoints cannot leave that origin. Tool network grants remain independent; explicit denies and managed-only domain policy still apply. Retirement, configuration replacement, shutdown and permission changes cancel the covered requests. Authentication helpers, stdio processes, OAuth discovery and model Provider networking remain separate contracts.

`RequireMCPHelpers` / `sandbox_mcp_helpers_v1` independently requires confined macOS authentication helpers, mandatory sandboxing and explicit MCP configuration. Each request refreshes credentials under the command resource policy and filtered task environment; it cannot borrow the MCP endpoint grant. Execution/output are bounded, policy changes cancel helpers, shutdown waits for cleanup, and failures do not fall back to stale/static credentials. Stdio and detached-descendant supervision remain separate contracts.

`RequireMCPStdio` / `sandbox_mcp_stdio_v1` separately confirms explicit macOS stdio MCP execution under the current command sandbox. It requires mandatory sandboxing and strict MCP configuration. Cancellation retires the whole service and all its pending calls; replacement waits for the old named process, and session shutdown awaits owned process cleanup. Requests are not replayed. Inherited model Provider credentials are filtered before explicit service credentials are applied; service environment cannot override reserved runtime, home or temporary-root inputs. Network access uses the command policy, without an endpoint or tool-approval grant. JSONL messages are capped at 10 MiB, pending calls at 64, and stderr is drained without exposing credentials. Ordinary process groups are covered; independently detached descendants and host-crash recovery require separate evidence.
