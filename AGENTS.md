# AGENTS.md

## 项目定位

本仓库是 Nexus 开源 runtime bridge。它只承载 Go 宿主与 runtime 子进程之间的公开契约、进程生命周期和 stdio `stream-json` 传输，不实现 agent loop，也不 import 闭源 `nexus-agent-sdk-go`。

```
Nexus product
  -> nexus-agent-sdk-bridge
       -> exec nxs 或 Claude Code
```

## 边界约定

- `client/`：Session、能力发现、runtime control 与消息投影
- `protocol/`：stdio control/message wire 真相源
  - `protocol/` 直接承载 Claude Code 的 mixed-casing control wire；不要再引入
  全局 snake/camel 转换层。工具参数、hook 输入和 provider payload 各自遵循
  所属协议（例如 Agent 的 `subagent_type`、Bash 的 `run_in_background`、Read 的
  `file_path` 仍为 CC 原生 snake_case），不能按字段外观批量改名。
- `internal/transport/`：子进程和传输实现，不向产品泄漏；显式 `Options.ProcessSupervision` 为正式会话与每个 CLI probe 申请独立 Host，清理失败保留类型化错误；该模式禁止跨用户/裸 PID 信号覆盖，中断走已有 control 协议，失败不回退普通 exec
- `internal/processbootstrap/` 与 `cmd/nexus-runtime-bootstrap/`：macOS 独立引导入口及限长启动协议；核验固定宿主连接身份后只接收三个标准管道和显式启动输入，原地 exec。宿主须先核验 helper/job 并持久登记再放行；失败入口进程必须退出，不复用未知控制消息状态。已支持显式监督 transport；Nexus 默认装配与发布包仍未接入。
- `internal/processscope/`：显式监督传输使用的 macOS coalition 观察与 audit-token 精确清理组件；登记必须来自执行前可信持久身份，CapturePeer 另核对连接的内核 audit identity 与可信 launcher 指定的 PID，不能替代 job/可执行文件认证；仅内核回收或 boot 变化可证明退出，不以空枚举、计数或 launchd 卸载成功确认。缺少原生接口时返回 unavailable，不声明完整监督或 scratch 回收。WatchRoot 在执行前绑定根进程 kqueue 退出事件；取消等待或停止观察不能构造退出事实，根退出状态不能代替集合回收。
- `supervision/`：显式 macOS 宿主启动器，串联持久意图、受保护 job 发布、内核身份、原集合登记、一次性放行、根退出与集合回收；调用方负责可信路径及 owner/session/generation 事务，由显式监督 transport 调用；普通调用方默认不启用，也不授予沙箱或工具能力。`Recover` 仅消费可信持久原登记，先校验再撤销/回收，不重放任务；宿主必须取得排他生命周期所有权，boot 改变时不碰新 boot 的同名 job。
- `runtimes/`：runtime kind 的公开能力差异
- `docs/`：面向开源使用者的文档索引与 runtime 契约，不收录宿主产品内部设计
- Subagent control 只在 `subagent_control_v1` 协商后按活跃父 MCP identity 调用；不实现执行循环。
- 新能力必须先定义 capability；产品不能按 runtime 名称猜测 control 是否存在
- 原生 Read/Write/Edit 通过 sandbox_file_tools_v1 单独确认；RequireFileTools 依赖 RequireSandbox，只走 initialize，不进入普通 settings
- Glob/Grep 通过 sandbox_search_tools_v1 独立确认；RequireSearchTools 依赖命令和文件合同，要求变化必须替换进程，不借用 Claude 的原生沙箱声明
- stdio MCP 通过 sandbox_mcp_stdio_v1 与 RequireMCPStdio 独立确认受限进程，依赖必需沙箱与 strict 配置；取消撤销服务，同名替换和会话关闭等待普通进程组清理，脱离后代仍独立验收。
- MCP 认证 helper 通过 sandbox_mcp_helpers_v1 与 RequireMCPHelpers 独立确认受限执行和有界收口，要求 RequireSandbox 与 MCP.StrictConfig；不声明 stdio 或脱离 session 后代监督。
- 远端 MCP 通过 sandbox_mcp_network_v1 与 RequireMCPNetwork 独立确认 HTTP/SSE 端点网络和撤销，依赖 RequireSandbox 与 MCP.StrictConfig；不代表认证 helper、stdio 或 Provider 网络隔离。
- 远程图片通过 sandbox_media_network_v1 与 RequireMediaNetwork 独立确认，依赖命令、文件和媒体文件合同；逐请求网络准入与 URL 物化不代表 Provider transport 或外部 MCP 网络隔离。
- 本地图片读取通过 sandbox_media_files_v1 独立确认；RequireMediaFiles 依赖命令和文件合同，只走 initialize，不能代表远程图片网络或 Claude 的沙箱。
- 普通配置通过 sandbox_settings_files_v1 与 RequireSettingsFiles 独立确认受限读取和完整快照；只走 initialize 并参与进程替换，不代表凭据隔离或原子权限持久化。
- 普通配置写入通过 sandbox_settings_writes_v1 与 RequireSettingsWrites 独立确认，依赖命令、文件工具和配置读取合同；只走 initialize，不代表跨进程 CAS、掉电持久性或持久回执。
- 托管策略通过 sandbox_managed_policy_v1 与 RequireManagedPolicy 独立确认固定来源和执行前完整性；宿主要求只走 initialize，变化须替换进程，不借用其他后端声明。
- 项目定义文件通过 sandbox_project_files_v1 与 RequireProjectFiles 独立确认，不推断全局权限/Provider 设置或 hook 执行。
- 上下文文件通过 sandbox_context_files_v1 与 RequireContextFiles 独立确认，覆盖指令及 compact 恢复，不推断全 SDK 配置或后台 IO。
- Skill 目录、正文、动态发现及 remember 设置通过 sandbox_skill_files_v1 独立确认；RequireSkillFiles 依赖命令和文件合同，不代表启动配置、hook 或后台 IO 已收口。
- Resources 通过 sandbox_resources_v1 独立协商写范围与宿主 scratch，依赖命令和文件合同；Bridge 不拥有目录生命周期，也不推断全 SDK IO 隔离
- 单条消息的无工具与输出预算共用 `message_execution_policy_v1`；未协商时宿主必须拒绝受限消息
- 下一条消息的宿主上下文由 bridge 确定性排序；nxs 将其作为只进入 live model history、不落 transcript 的隐藏 reminder，Claude Code 通过 `UserPromptSubmit` hook 的 `additionalContext` 生成同语义 attachment
- AutoDream 只由原生 nxs 提供；宿主负责唤醒，nxs 负责最终执行判断
- 长时 control 的 context 取消必须携带同一 `request_id` 传播到 runtime
- 进程清理错误通过 ProcessCleanupError 穿过 Wait 与重复 Close；Unix session 清理不代表全部脱离后代或宿主资源租约已经回收
- Windows runtime/CLI probe 必须 CREATE_SUSPENDED，在绑定 kill-on-close Job 后校验并恢复初始线程；probe 取消先清后代再等待管道，版本预检同样清理并裁剪环境。Job 绑定前宿主崩溃仍可能留下挂起进程，不能声称创建/绑定原子性。

## 开发约定

- 用户可见能力、协议或路径变化同步更新 `CHANGELOG.md`、英文 README 与中文 README
- README 只保留安装、可运行示例和公开边界；完整类型以 GoDoc 为准，Runtime 差异以 `docs/runtime-contract.md` 为准
- 注释使用中文，解释意图和边界
- Go 代码执行 `gofmt`，实现遵循 Google Go Style
- 全量验证使用 `go test ./...`
- 提交使用 emoji 前缀和英文摘要，例如 `:sparkles: Add native AutoDream control`

nxs 自动审核必须协商 `auto_review_v1`；Claude 使用原生 `set_permission_mode` 并明确确认 `mode=auto`。bridge 不执行审核模型；能力入口表示协议适配，账号、模型和策略可用性由 runtime 确认。
