# Runtime 契约

## 边界

`nexus-agent-sdk-bridge` 是 Go 宿主与 Agent runtime 之间的开源进程和协议边界。

```text
Go 宿主
  -> bridge client
       -> runtime 子进程或宿主管理的 transport
            -> stream-json 消息与控制请求
```

Bridge 负责进程启动、transport 生命周期、类型化消息、控制请求、Hook、权限回调和
进程内 MCP 接入。它不实现 agent loop，不调用模型 Provider，也不包含原生 `nxs`
runtime 的源代码。

## Runtime 选择

| Runtime | 选择方式 | 命令解析 |
| --- | --- | --- |
| `nxs` | 默认，或 `WithRuntime(client.RuntimeNXS)` | 显式 `WithCLIPath` 或 `NEXUS_NXS_COMMAND_PATH` |
| Claude Code | `WithRuntime(client.RuntimeClaude)` | 显式路径、`NEXUS_CLAUDE_COMMAND_PATH` 或安全的平台发现 |
| Direct connect | `WithDirectConnect(...)` | 宿主管理远端 runtime 进程 |
| 自定义 transport | `WithTransport(...)` | 宿主管理完整连接 |

Bridge 不下载 `nxs`，不扫描应用包、缓存或 PATH。官方 Nexus 发布包会单独提供
闭源的 `nxs` 可执行程序，并把明确路径交给 bridge。

## Wire 格式

进程 runtime 通过 stdio 交换行分隔 JSON，公开合同位于 `protocol/`。原生 `nxs`
与 Claude Code 共用同一条 mixed-casing control wire。

Runtime payload 确有差异时，兼容别名按字段声明。Bridge 不会对控制消息、工具参数、
Hook 输入或 Provider payload 做全局 snake_case/camelCase 转换。

## Capability 协商

宿主暴露可选控制前必须调用 `Session.Supports`。通用能力包括类型化 usage、终态分类、
停止任务、进程内 MCP、精确边界 Session fork 和 provider-neutral runtime lifecycle。
当前原生专属控制包括任务续聊、环境热更新和 AutoDream。Hook response ack 在初始化
阶段协商。原生 runtime 还可协商 `CapabilityMessageExecutionPolicy`，让宿主对单条消息
执行 `tool_access=none`、`max_output_tokens` 与 `skip_auto_memory`，无需修改 Agent 的持久配置。

`SetNextTurnContext` 会按优先级降序，再按名称、正文和 metadata 确定性排序内部
上下文块。NXS 在持久化 user 消息前提取隐藏提醒，将它保留在当前模型历史中但不写入
transcript。Claude Code 则通过原生 `UserPromptSubmit` hook 的 `additionalContext`
生成 attachment。

Capability 真相源位于 [`client/capability.go`](../client/capability.go)。不能用 Runtime
名称替代 capability 检查。

## Session 生命周期

1. `client.NewSession` 解析 transport 并初始化 runtime。
2. `Session.Send` 或 `SendWithOptions` 启动一轮执行。
3. 宿主通过 `Recv` 消费类型化消息，或通过 `Result` 等待终态。
4. 控制请求复用活跃 session，并保留 runtime request identity。
5. `Session.Close` 依次等待 transport 的 `Close`、`Wait` 和读取循环退出，之后才确认清理完成。终止尝试失败不代表进程已经退出；调用方取消只结束自身等待，不解除共享清理栅栏。关闭与退出错误均保留。

`client.ForkSession` 会复制源 Session 到传入消息 ID 的精确边界，并创建独立目标。
Claude Code 可能到首个用户回合才持久化目标 transcript，但 bridge 会在返回 Session
前分配目标 Session ID。

Context 取消和用户中断可通过 Go error matching 区分。长时 control 会使用同一个
`request_id` 传播取消，使 runtime 能停止原始操作。

## 安全职责

- 宿主决定信任哪个 runtime 可执行程序和进程环境。
- Bridge 只传递 sandbox policy，不宣称自己执行沙箱隔离。
- 默认进程信号假设宿主与 runtime 使用同一 OS 身份；跨身份运行时，宿主必须通过
  `WithProcessSignalHandler` 提供边界，并在转发生命周期信号前校验 PID 归属。
- 自动生成或内联的 MCP 配置会写入权限受限的参数文件，不直接暴露在进程参数中。
- Provider 凭据保留为 runtime 进程环境；bridge 不解释 Provider 请求 payload。

Runtime 强制隔离与产品授权不属于本库职责。

## 兼容策略

公开 Go package 按仓库版本演进，`internal/` 不属于可依赖 API。新增 runtime 专属行为
必须先定义公开 capability 和类型化协议；宿主不应依赖未文档化 payload 字段。

## 必需沙箱执行

原生 Read/Write/Edit 覆盖须额外使用下述分项合同；基本命令能力不能推断文件工具隔离。

`SandboxSettings.RequireSandbox=true` 要求 nxs 协商 `required_sandbox_v1`。
Bridge 在 initialize 发送布尔 `required_sandbox`，独立于普通 settings；nxs 在创建 SDK
会话前校验类型和能力、安装约束并确认能力。缺少确认时 Bridge 断开连接，
ConnectWithPrompt 不发送用户消息。Claude 对该选项在启动 transport 前拒绝。

该合同保证命令排除和后端不可用不触发隐式直跑，不表示 Windows 后端可用、文件/网络
策略已全部归宿主控制或运行中权限模式已与沙箱同步。默认关闭；切换权限模式不清除此
要求，当前需要以新的有效策略创建新会话。

必需沙箱还在普通/原始消息和内部续跑的写入入口检查本次连接的能力确认；
transport 已连接不代表握手完成。提前并发发送会失败，且不会消费待发送的轮次上下文。

必需模式在 initialize 的对象 `sandbox_policy` 中传递显式宿主 Sandbox 配置；
nxs 在创建会话前拒绝类型错误。普通 inline/project settings 不扩展必需模式的资源授权。

`Session.Reconfigure` 遇到任一方向的 Sandbox 变化时，在其他热更新前返回
`ErrRestartRequired`（`sandbox_policy_changed`），保留当前 options。宿主必须退出旧进程
并创建新会话；仅修改权限模式不代表沙箱策略切换。

## 原生文件工具隔离

`SandboxSettings.RequireFileTools=true` 额外要求 `RequireSandbox=true` 及 nxs 的 `sandbox_file_tools_v1`。Bridge 只在 initialize 发送布尔 `required_sandbox_file_tools`，普通 settings 不承载此要求。矛盾配置在启动 transport 前拒绝；缺少任一能力确认时先断开，再返回错误，不发送任务。原始/内部/并发消息使用同一准入检查；改变要求必须替换进程。

该能力覆盖内置 Read/Write/Edit 的文件执行边界，包括内容、目录建议、链接/元数据及读取新鲜度检查；准备或执行失败不能回退为宿主直接 IO。当前 nxs 仅在 macOS 声明此能力，不表示依赖可用、当次策略生效、整个 SDK 主进程或 Glob/Grep、Notebook、启动配置、Skill、记忆与外部 MCP 已受限。

旧宿主仍可只要求基本命令合同；需要原生文件隔离的宿主必须同时要求两个合同。`Session.Supports(CapabilitySandboxFileTools)` 只报告当前 nxs 确认，不授予权限，也不是用户启停开关。Claude Code 的原生沙箱另行适配，不使用该扩展。

## 宿主资源写入范围

`SandboxSettings.Resources` 使用 `SandboxResourcePolicy`（`version`、`write_scope`、`scratch_root`），另要求 `sandbox_resources_v1`。Bridge 在普通 `sandbox_policy` 外独立发送 `required_sandbox_resources`，同时要求命令和文件合同，并在任何任务写入前检查三项确认。Resources 和 Extra 不能把该对象放进普通 settings。选项复制持有独立策略值，改变策略必须替换进程。

版本 1 接受 `read-only`、`workspace-write` 及显式绝对 scratch 目录。只读模式拒绝额外写授权，两种模式都拒绝明确允许未隔离命令。当前 nxs 只在 macOS 支持，并在初始化前拒绝无效、不支持或与工作区重叠的目录输入。已覆盖的命令/文件执行器不再从 HOME/TMPDIR、旧工作区临时位置或缓存兼容项扩大写入范围。

宿主负责独占准备 scratch，并在执行和后代结束后回收；Bridge 不创建租约或证明目录私有性。此合同不覆盖全 SDK IO，也不是实际生效回执。普通读/deny 和网络保持各自合同，Claude 原生适配、其他平台及宿主默认接入仍需分别验收。未提供 Resources 时保留现有合同。启动前拒绝的会话仍能完成清理；已受理启动的 transport 和读取循环继续必须退出。
