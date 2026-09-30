// Package windowssandbox 承载Windows机器沙箱的显式宿主监督协议。
//
// wire.go 校验固定nxs机器Host的准备与清理回执；关联、执行和双摘要分别绑定。
// native_identity/pipe/machine/job_windows负责固定机器映像、认证管道和创建时Job归属。
// process_windows负责持久宿主回调、单次启动、广播清理状态和未知结果重试；非Windows明确拒绝。
// 当前代码未编译/原生验收，Nexus持久投影与App安装装配仍在实现。
// 不能把本包协议解析或helper退出当作完整集合/资源回收证明，不启用任何capability。
package windowssandbox
