// Package protocol 定义 bridge 收发消息、内容块、control wire 模型，以及把
// typed task 与原生 Agent progress/completion 归一到统一 Tool/Subagent lifecycle 的投影。
// required_sandbox_file_tools 与 required_sandbox 只用于 nxs 的分项初始化要求。
// sandbox.go 定义 required_sandbox_resources 的唯一版本化线格式，不进入普通 sandbox_policy。
package protocol
