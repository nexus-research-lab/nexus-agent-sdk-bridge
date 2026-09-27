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

进程清理失败通过 `client.ProcessCleanupError` 返回，可用 `errors.As` 匹配。
主进程正常退出、主动终止以及重复调用进程 Close 都不能消除该错误。Unix 清理在
有界发送信号后再次观察；枚举失败或仍有可见成员时返回错误，只忽略观察期间已消失的进程。

这是 session 内清理步骤，不是完整进程树回执。后代可以另建 Unix session，Linux 的
PID namespace 和 `/proc` 可见范围也限制观察。宿主信号回调自行定义清理语义，回调
返回成功不代表 Bridge 又独立验证了退出。Windows 以 `CREATE_SUSPENDED` 创建
runtime，先绑定带 `KILL_ON_JOB_CLOSE` 的 Job，再核对初始线程所属进程并恢复执行。
入口代码不能在 Job 绑定前执行或派生后代；创建、绑定、线程查询或恢复失败均拒绝准入。
清理等待 Job 变为空，Bridge 崩溃后句柄关闭也会终止已准入成员。Windows 11 amd64
原生测试覆盖入口立即派生、继承输出管道时取消及强制终止宿主。

CLI help/settings/restricted/version 预检共用该边界。取消先清后代再等退出，父进程
退出后的管道等待有界；版本预检也使用裁剪后的环境和有界输出。

进程创建和 Job 绑定仍是两个 OS 操作：绑定前 Bridge 崩溃可能留下尚未执行入口的
挂起进程。这不代表原子创建回执、进程身份复用安全、私有 scratch 恢复、文件/网络
隔离或 clean-host 安装验收。宿主必须保留失败边界，不能因流已关闭就复用资源。

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

## Claude 原生受限启动

桌面产品默认使用 `SandboxSettings.RequireClaudeNativeSandbox=true`。该合同要求
Bridge 生成的 `--settings` JSON 同时包含 `sandbox.enabled=true`、
`sandbox.failIfUnavailable=true` 和 `sandbox.allowUnsandboxedCommands=false`，并
提供 `CapabilityClaudeNativeSandbox`。Claude 原生 sandbox 因而保留 Bash/构建命令，
再由 Claude 自己的 OS 后端限制命令、文件和网络；Bridge 会在正式进程前拒绝缺失、
重复或被 `ExtraArgs` 覆盖的 settings。网络域名仍由 Nexus 的宿主准入提供，空准入
序列化为 deny-all。该能力不冒用 nxs `required_sandbox_v1`，也不把 settings 注入
当成运行期有效策略回执；真实允许/拒绝、取消、后代和平台证据仍需独立验收。
正式进程前 Bridge 还会在脱敏环境中对同一个已解析的 Claude 可执行程序执行有界的无模型
`--settings <generated-json> --help` 探测；CLI 不接受或不声明该入口时失败关闭。该探测
只证明启动参数入口契约，Claude 仍是原生 OS 初始化和命令执行约束的权威。

旧的 `RequireClaudeRestricted=true` 仍表示 Claude 的 `--restricted` 工具裁剪模式。
它会移除 Bash 等代码执行工具，不能作为桌面默认命令沙箱的替代；两种合同不能同时
启用。Full Access 不安装原生合同，仍受宿主生命周期和领域授权约束。

`SandboxSettings.RequireClaudeRestricted=true` 是 Bridge 对 Claude Code 原生
`--restricted` 模式的类型化合同。它只能与 `RuntimeClaude` 一起使用；Bridge
会在 Claude 进程启动参数中加入且只加入一次 `--restricted`，并为当前会话提供
`CapabilityClaudeRestricted`。这是独立的适配合同，不发送也不冒用 nxs 的
`required_sandbox_v1`。

该选项在 transport 启动前拒绝 `RuntimeNXS`、bypass 权限以及通过普通
`ExtraArgs`/`ExtraBoolArgs` 注入该标记的方式。要求变化返回
`sandbox_policy_changed`，必须替换 runtime 进程。Full Access（例如
`ModeBypassPermissions`）不要求该合同，也不会收到 `--restricted`。

Bridge 只验证类型化启动合同和传给所选可执行程序的参数。可执行程序必须支持
`--restricted`；旧版本或被替换的 Claude 命令拒绝该参数时，启动会失败关闭。
在正式 stream-json 进程准入前，Bridge 会对同一个已解析的可执行程序（Windows
也包括安全的 PowerShell shim）执行有界的 `--restricted --help` 探测。探测必须
成功退出并在帮助文本中声明该标记；探测输出有大小上限，常见 Provider、代理、
Cookie 及其他秘密环境变量不会传给探测进程。探测成功只证明参数解析/启动合同，
不是运行期隔离回执。
不证明 CLI 版本、OS 执行隔离、Provider/网络策略，也不证明 Claude SDK 的文件、
设置、hook、MCP、transcript 和后台 IO 已完整隔离；这些仍需独立验收证据。

`Session.Reconfigure` 遇到任一方向的 Sandbox 变化时，在其他热更新前返回
`ErrRestartRequired`（`sandbox_policy_changed`），保留当前 options。宿主必须退出旧进程
并创建新会话；仅修改权限模式不代表沙箱策略切换。

## 原生文件工具隔离

`SandboxSettings.RequireFileTools=true` 额外要求 `RequireSandbox=true` 及 nxs 的 `sandbox_file_tools_v1`。Bridge 只在 initialize 发送布尔 `required_sandbox_file_tools`，普通 settings 不承载此要求。矛盾配置在启动 transport 前拒绝；缺少任一能力确认时先断开，再返回错误，不发送任务。原始/内部/并发消息使用同一准入检查；改变要求必须替换进程。

该能力覆盖内置 Read/Write/Edit 的文件执行边界，包括内容、目录建议、链接/元数据及读取新鲜度检查；准备或执行失败不能回退为宿主直接 IO。当前 nxs 仅在 macOS 声明此能力，不表示依赖可用、当次策略生效、整个 SDK 主进程或 Glob/Grep、Notebook、启动配置、Skill、记忆与外部 MCP 已受限。

旧宿主仍可只要求基本命令合同；需要原生文件隔离的宿主必须同时要求两个合同。`Session.Supports(CapabilitySandboxFileTools)` 只报告当前 nxs 确认，不授予权限，也不是用户启停开关。Claude Code 的原生沙箱另行适配，不使用该扩展。

## Skill 文件隔离

`SandboxSettings.RequireSkillFiles=true` 依赖 `RequireSandbox` 和 `RequireFileTools`。Bridge 只在 initialize 发送 `required_sandbox_skill_files`，并在所有任务写入前要求 nxs 独立确认 `sandbox_skill_files_v1`。矛盾要求在启动 transport 前拒绝；缺少确认时先断开、不发送任务。普通 settings 和 Extra 不能注入此要求，改变要求必须替换进程。

当前 macOS nxs 将 Skill 目录、正文、动态发现、Git 忽略与 remember 可见性设置绑定到同一文件边界；动态发现保留触发 Read 的上下文和 cwd，允许的来源、条件激活及 Git 忽略行为继续成立。Git 使用最小环境并拒绝网络；拒绝、取消和未知结果不回退宿主 IO，未知观察可以在下次实际文件访问时重新核对。该能力不代表全局启动配置、hook、后台记忆、生效回执、其他平台或 Claude 原生沙箱已验收。

## 本地媒体文件隔离

`SandboxSettings.RequireMediaFiles=true` 依赖 `RequireSandbox` 和 `RequireFileTools`。Bridge 只在 initialize 发送 `required_sandbox_media_files`，并在所有任务写入前要求 nxs 独立确认 `sandbox_media_files_v1`。矛盾配置在启动 transport 前拒绝；缺少确认时先断开，不发送任务。普通 settings 和 Extra 不能塞入该宿主要求，要求变化必须替换进程。

当前 macOS 合同覆盖 ViewImage 与主模型预处理的本地图片读取，包括路径、file URL、符号链接、延迟引用和嵌套工具图片。准备或读取失败不能回退，辅助分析缓存前仍检查读取。旧命令、文件或搜索能力不能代替该确认；远程媒体网络、生效回执、全 SDK IO、其他平台和 Claude 原生沙箱继续独立验收。

## 远程图片网络

`SandboxSettings.RequireMediaNetwork=true` 依赖命令、文件与本地媒体要求。Bridge 只在 initialize 发送 `required_sandbox_media_network`，每次任务写入前要求 nxs 确认独立的 `sandbox_media_network_v1`。依赖缺失在 transport 启动前拒绝；旧版本不发送 prompt；要求变化必须换进程，普通 settings 不能注入。

当前 macOS nxs 先按本次网络策略下载图片，再将内容交给主/辅助 Provider，覆盖惰性引用及嵌套工具图片。每次请求和重定向检查目标，deny 优先，managed-only 不可经批准扩大，不继承环境代理或 Provider 凭据。ViewImage 批准绑定原始输入、tool-use、cwd、目标及权限代次；无工具身份的消息预处理仅使用既有授权或宿主明确回调。权限改变和清理取消在途批准及响应体读取。Provider transport、其他辅助网络、外部 MCP 和 Claude 保持独立边界。

## 搜索工具隔离

`SandboxSettings.RequireSearchTools=true` 同时依赖 `RequireSandbox` 和 `RequireFileTools`。Bridge 仅在 initialize 发送 `required_sandbox_search_tools`，并在所有任务写入前要求 nxs 独立确认 `sandbox_search_tools_v1`。普通 settings 和 Extra 不能塞入该宿主要求；矛盾配置在启动 transport 前拒绝，要求变化必须替换进程。旧版本具备命令和 Read/Write/Edit 能力不代表搜索受限。

当前 macOS nxs 将 Glob/Grep 的路径检查、缺失路径建议、rg 和结果元数据交给同一文件执行边界。辅助进程使用最小环境、拒绝网络并复用资源策略；准备、取消、输出限额及执行失败不得回退宿主 IO、重放或报告部分成功，单文件内容/计数保留文件名。该合同不证明当前策略已生效、全 SDK IO、脱离后代清理、scratch 回收、其他平台或 Claude 原生沙箱已验收。

## 宿主资源写入范围

`SandboxSettings.Resources` 使用 `SandboxResourcePolicy`（`version`、`write_scope`、`scratch_root`），另要求 `sandbox_resources_v1`。Bridge 在普通 `sandbox_policy` 外独立发送 `required_sandbox_resources`，同时要求命令和文件合同，并在任何任务写入前检查三项确认。Resources 和 Extra 不能把该对象放进普通 settings。选项复制持有独立策略值，改变策略必须替换进程。

版本 1 接受 `read-only`、`workspace-write` 及显式绝对 scratch 目录。只读模式拒绝额外写授权，两种模式都拒绝明确允许未隔离命令。当前 nxs 只在 macOS 支持，并在初始化前拒绝无效、不支持或与工作区重叠的目录输入。已覆盖的命令/文件执行器不再从 HOME/TMPDIR、旧工作区临时位置或缓存兼容项扩大写入范围。

宿主负责独占准备 scratch，并在执行和后代结束后回收；Bridge 不创建租约或证明目录私有性。此合同不覆盖全 SDK IO，也不是实际生效回执。普通读/deny 和网络保持各自合同，Claude 原生适配、其他平台及宿主默认接入仍需分别验收。未提供 Resources 时保留现有合同。启动前拒绝的会话仍能完成清理；已受理启动的 transport 和读取循环继续必须退出。

## 上下文文件隔离

`SandboxSettings.RequireContextFiles=true` 依赖命令和文件合同，并在每次任务写入前独立要求 nxs 确认 `sandbox_context_files_v1`。覆盖启动/动态指令、指令排除设置及 compact 文件恢复；缺少确认先断开，不发送任务。普通 settings 与 Extra 不能注入该要求，变化必须替换进程。

排除配置不可读取时拒绝启动/重载；失败重载清除旧缓存，恢复读取之前阻止后续模型请求。权限/Provider 设置、项目定义、hook、持久化、后台 IO 与 Claude 原生接入仍独立验收。

## 项目定义文件隔离

`SandboxSettings.RequireProjectFiles=true` 同时要求命令、文件合同及独立 `sandbox_project_files_v1` 确认。覆盖项目 Agent/命令/Skill 定义和所选 hook 设置的启动/显式刷新读取；失败阻止后续执行，Agent/hook 绑定变更须重建 runtime。每次任务写入前检查，变化须替换进程，普通 settings/Extra 不得注入此宿主要求。全局权限/Provider 设置、持久化、hook 执行及其他后端不在本能力中。

## 普通配置文件与快照

`SandboxSettings.RequireSettingsFiles=true` 依赖命令和文件合同，并要求 nxs 在任务准入前独立确认 `sandbox_settings_files_v1`。Bridge 只在 initialize 发送 `required_sandbox_settings_files`，不把它放入普通 settings；要求变化必须替换进程。Claude 不能声明该 nxs 能力。

runtime 在 settings profile 投影前固定配置根与所选来源。必需模式通过受限文件 worker 完整读取，拒绝不完整或无效快照，并在 query、compact、工具和配置控制前核对来源完整性。禁用来源在 IO 前过滤；子 runtime 保留独立绑定快照。外部内容改变后必须重建 runtime，或恢复原始内容后再继续。动态更新会拒绝静态执行字段，`get_settings` 使用已绑定快照。

当前确认范围是原生 macOS。Provider 凭据隔离、配置持久化、跨进程并发、持久批准/回执、后台 IO 与其他平台仍是独立合同；快照检查不会撤销已发生的副作用，也不构成完整持久化事务。

## 普通配置写入

`SandboxSettings.RequireSettingsWrites=true` 同时依赖 `RequireSandbox`、`RequireFileTools` 和 `RequireSettingsFiles`。Bridge 只在 initialize 发送 `required_sandbox_settings_writes`，不写入普通 `sandbox_policy`，并要求 nxs 确认 `required_sandbox_v1`、`sandbox_file_tools_v1`、`sandbox_settings_files_v1` 和 `sandbox_settings_writes_v1`。矛盾配置在启动 transport 前拒绝；缺少确认时先断开，不向宿主暴露 runtime Session，也不发送普通、原始或内部任务消息。该宿主选项参与进程替换与 restart-sensitive 指纹。Claude 不能声明或要求此 nxs 扩展，旧 nxs 会失败关闭。

当前原生 macOS nxs 将 Config 与显式权限持久化交给同一个启动时绑定的 settings store。Config 使用文件型 `--settings` 指定的文件，否则使用 user settings；inline flag 回退 user settings，没有可写来源时拒绝。Config 读取返回目标文档值，不把它声明为分层合并后的最终值；写入使用 canonical 嵌套键并保留无关及旧字段。显式 SDK Options 和进程环境仍有更高优先级，因此持久化值只是下一 runtime 的设置默认值，不是最终 effective 值证明。写入前先构造完整分层快照；如果 project、local、flag 或 managed policy 的高优先级来源仍覆盖目标值，则在落盘前拒绝。Config 实际变化返回 `runtimeRestartRequired=true` 并阻止当前 runtime 后续执行；no-op 不要求重建。

物理 writer 固定目录身份，拒绝后续符号链接、目录代次和特殊文件替换，并同时保护目标 settings 与随机临时文件名模式的字面路径及解析后物理别名，阻止沙箱内任务跨过配置写入边界。必需模式拒绝已有多硬链接 settings，因为路径沙箱不能枚举所有别名。writer 同步并持续持有临时文件，通过父目录句柄做同目录替换并核对提交身份。单文档替换具备原子可见性；跨多文档的权限更新按确定顺序提交，部分提交会把共享 store 标记为 unknown。runtime clone 的逻辑快照独立，但共享写锁、目录代次、unknown 与 recreate 栅栏。

该能力不保证宿主上另一个未受沙箱的同 UID 进程并发修改、跨进程锁或 CAS、多文档 all-or-nothing、父目录 fsync 或断电持久性，也不提供 exact request/approval/revision 的持久绑定、durable receipt、重启后的 unknown 对账或自动重放。Unix 替换只保留普通 permission bits，不承诺 owner、ACL、xattr 或文件 flags。Provider 凭据、任务环境和其他 SDK IO 仍是独立边界；Windows 当前只有交叉编译证据，Go 可写位检查不代表 DACL 私密性，也不属于原生验收。

SDK 托管 MCP 调用上下文：nxs 和使用同一线格式的 runtime 所发 `params._meta["claudecode/toolUseId"]` 原样进入 `tools.Context.ToolUseID`。每次调用的元数据独立，不从参数或 JSON-RPC id 生成身份，也不推断 SessionID/RoundID；缺省元数据不继承父调用身份。

### 远端 MCP 网络

`RequireMCPNetwork` / `sandbox_mcp_network_v1` 要求必需沙箱、`MCP.StrictConfig` 显式服务来源和 nxs 能力确认。HTTP 与旧式 SSE 服务仅获得自身协议、主机、端口的端点授权；跳转和 SSE POST 地址不能跨 origin，普通工具联网权限保持独立，显式禁止及托管域名规则仍优先。撤销、更换配置、关闭及权限变化取消对应请求。认证 helper、stdio 进程、OAuth 发现和模型 Provider 网络仍是独立合同。
