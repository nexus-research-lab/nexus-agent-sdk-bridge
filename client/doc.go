// Package client 提供 bridge SDK 的查询、会话、执行连接与运行期控制 API。
// subagent.go 提供协商后、当前父 MCP 调用内的原生子任务控制；不实现任务执行。
// 必需沙箱通过 required_sandbox_v1 初始化协商，旧运行时不得静默降级。
// RequireFileTools 额外要求 sandbox_file_tools_v1；不以命令能力推断原生文件工具已隔离。
// RequireSearchTools 另要求 sandbox_search_tools_v1，Glob/Grep 覆盖不从旧文件合同推断。
// RequireMediaNetwork 独立要求远程图片网络准入和 URL 物化，依赖命令、文件与本地媒体合同。
// RequireMediaFiles 独立确认图片入口的本地读取，不能推断远程网络或 Claude 能力。
// RequireNotebookFiles 独立确认 Notebook 内容与 cell output 的本地读取，不能推断 Notebook 执行或远程网络。
// RequireSkillFiles 独立确认 Skill 发现/正文、Git 忽略与 remember 设置；不推断全 SDK IO。
// RequireContextFiles 独立确认指令及 compact 文件读取；依赖命令和文件合同。
// Resources 通过独立版本合同确认只读/工作区写及宿主 scratch；复制选项、启动准入和进程替换保持此边界。
// RequireClaudeRestricted 只为 Claude Code 安装原生 --restricted 工具裁剪；它不冒用 nxs 合同，Full Access 不需要该要求，且不证明 Claude SDK 全部 IO 隔离。
// RequireClaudeNativeSandbox 要求 Claude Code 通过 sandbox settings 保留命令工具并启用原生 OS 沙箱，缺失或允许 unsandboxed command 时失败关闭。
// sandbox.go 在消息写入前独立校验当前初始化能力，阻止握手期间的并发提前发送。
// reconfigure.go 对沙箱策略变化返回明确的重启要求，禁止仅更新本地 options 伪装生效。
// conn.go 的共享关闭先等待 transport Close 与 Wait，再确认清理完成；调用方超时不释放退出栅栏。
// Start 前拒绝配置的会话没有读取循环，关闭不会等待未启动的循环；已启动的循环仍必须终态。
// ProcessCleanupError 保留后代清理失败；主进程退出、主动终止和重复 Close 都不能消除它。
// Unix session 清理只观察仍属于该 session 的进程，不提供脱离后代或跨重启回收证明。
// permission_boundary distinguishes tool access from a single sandbox escape; unknown boundaries are rejected at transport admission.
// RequireProjectFiles 独立确认项目定义和 hook 设置读取；变化要求替换进程。
// RequireManagedPolicy 独立确认托管来源和完整性；依赖命令/文件合同并参与进程替换。
// RequireSettingsFiles 独立要求普通配置读取和快照完整性；不能推断凭据或持久化保证。
// RequireSettingsWrites 独立要求普通配置受控写入；依赖读取合同且不表示跨进程事务或持久回执。
// RequireMCPNetwork 独立要求显式 HTTP/SSE 端点网络和撤销，要求 RequireSandbox 与 MCP.StrictConfig。
package client
