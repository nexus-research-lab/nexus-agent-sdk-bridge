// Package supervision 为可信宿主提供显式 macOS runtime 启动器。
//
// L2 | 父级: AGENTS.md
// types.go 定义固定意图、受保护位置和持久阶段回调；start_darwin.go 执行启动握手；
// process.go 负责共享等待/关闭，job_darwin.go 限界调用系统 launchctl；不支持构建拒绝。
// 宿主负责 owner/session/generation、可信目录、job 文件发布、数据库事务和恢复调度。
// Reserve/Register/ClaimRelease 必须持久提交后返回；ClaimRelease 不可重放。
// 本包不是默认 transport，也不授予 sandbox、文件、网络或工具权限。
package supervision
