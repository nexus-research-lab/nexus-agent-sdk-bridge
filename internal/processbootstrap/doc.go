// Package processbootstrap 提供 macOS 可信引导进程的单次执行协议。
//
// L2 | 父级: AGENTS.md
// launch.go 定义可在登记前校验的限长、闭合启动输入；wire_darwin.go 仅接收三条标准管道；
// helper_darwin.go 核验启动宿主后等待启动输入，最终原地 exec，不创建未登记子进程。
// helper_unavailable.go 拒绝不支持的构建。Run 只供独立 helper 入口调用，返回后
// 该进程必须退出，不能复用可能收到截断控制消息的进程。
// 此包不实现 launchd job 创建或持久登记；
// 宿主必须在发送启动输入之前验证 helper 身份并持久保存原集合登记。
package processbootstrap
