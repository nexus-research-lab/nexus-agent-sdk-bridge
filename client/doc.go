// Package client 提供 bridge SDK 的查询、会话、执行连接与运行期控制 API。
// subagent.go 提供协商后、当前父 MCP 调用内的原生子任务控制；不实现任务执行。
// 必需沙箱通过 required_sandbox_v1 初始化协商，旧运行时不得静默降级。
// RequireFileTools 额外要求 sandbox_file_tools_v1；不以命令能力推断原生文件工具已隔离。
// RequireSearchTools 另要求 sandbox_search_tools_v1，Glob/Grep 覆盖不从旧文件合同推断。
// RequireMediaFiles 独立确认图片入口的本地读取，不能推断远程网络或 Claude 能力。
// Resources 通过独立版本合同确认只读/工作区写及宿主 scratch；复制选项、启动准入和进程替换保持此边界。
// sandbox.go 在消息写入前独立校验当前初始化能力，阻止握手期间的并发提前发送。
// reconfigure.go 对沙箱策略变化返回明确的重启要求，禁止仅更新本地 options 伪装生效。
// conn.go 的共享关闭先等待 transport Close 与 Wait，再确认清理完成；调用方超时不释放退出栅栏。
// Start 前拒绝配置的会话没有读取循环，关闭不会等待未启动的循环；已启动的循环仍必须终态。
// ProcessCleanupError 保留后代清理失败；主进程退出、主动终止和重复 Close 都不能消除它。
// Unix session 清理只观察仍属于该 session 的进程，不提供脱离后代或跨重启回收证明。
// permission_boundary distinguishes tool access from a single sandbox escape; unknown boundaries are rejected at transport admission.
package client
