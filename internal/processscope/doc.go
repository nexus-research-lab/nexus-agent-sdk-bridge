// Package processscope 提供 macOS coalition 观察与精确清理组件，供 supervision transport 使用。
//
// L2 | 父级: AGENTS.md
// scope.go 持有登记身份、控制连接的内核 peer 核验、恢复校验和有界清理；
// CapturePeer 要求可信 launcher 提供 expectedPID，仍不验证 job 或可执行文件。
// peer.go 提供固定宿主身份/boot 快照、编码与控制连接反向认证。
// exit.go/exit_darwin.go 在放行前绑定根进程 kqueue 退出事件；等待取消不撤销共享观察，
// 停止观察不算退出，主进程状态与 coalition 回收证明相互独立。
// kernel_darwin.go 与 native_darwin.c
// 只实现内核观察及 audit-token 信号；不支持的平台或构建明确返回 ErrUnavailable。
// 调用者必须在运行不可信代码前，将 Capture 的登记持久绑定到其已认证执行身份。
// Restore 只接收该可信存储的原记录，不能消费模型或网络提交的 coalition ID。
// 组件不创建 launchd job、不直接回收 scratch；supervision 负责 job、Close 和持久恢复，
// 仅在被可信宿主配置时声明登记 coalition 的后代清理，不把普通未监督 transport 伪装成受控运行。
package processscope
