// Package client 提供 bridge SDK 的查询、会话、执行连接与运行期控制 API。
// subagent.go 提供协商后、当前父 MCP 调用内的原生子任务控制；不实现任务执行。
// 必需沙箱通过 required_sandbox_v1 初始化协商，旧运行时不得静默降级。
// sandbox.go 在消息写入前独立校验当前初始化能力，阻止握手期间的并发提前发送。
// reconfigure.go 对沙箱策略变化返回明确的重启要求，禁止仅更新本地 options 伪装生效。
// conn.go 的共享关闭先等待 transport Close 与 Wait，再确认清理完成；调用方超时不释放退出栅栏。
// permission_boundary distinguishes tool access from a single sandbox escape; unknown boundaries are rejected at transport admission.
package client
