// Package processscope 提供尚未接入传输的 macOS coalition 观察与精确清理组件。
//
// L2 | 父级: AGENTS.md
// scope.go 持有登记身份、恢复校验和有界清理；kernel_darwin.go 与 native_darwin.c
// 只实现内核观察及 audit-token 信号；不支持的平台或构建明确返回 ErrUnavailable。
// 调用者必须在运行不可信代码前，将 Capture 的登记持久绑定到其已认证执行身份。
// Restore 只接收该可信存储的原记录，不能消费模型或网络提交的 coalition ID。
// 组件不创建 launchd job、不自动进入现有 Close、不回收 scratch，也不声明完整后代监督。
package processscope
