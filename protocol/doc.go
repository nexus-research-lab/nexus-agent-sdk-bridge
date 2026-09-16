// Package protocol 定义 bridge 收发消息、内容块、control wire 模型，以及把
// typed task 与原生 Agent progress/completion 归一到统一 Tool/Subagent lifecycle 的投影。
// required_sandbox_file_tools 与 required_sandbox 只用于 nxs 的分项初始化要求。
// required_sandbox_search_tools 独立要求搜索执行边界，依赖前两项，不进入普通 settings。
// required_sandbox_media_files 独立要求本地图片读取，同样依赖命令及文件合同。
// required_sandbox_skill_files 独立要求 Skill 文件读取，也仅通过 initialize 消费。
// required_sandbox_context_files 独立要求指令与 compact 文件读取，仅用于 nxs initialize。
// sandbox.go 定义 required_sandbox_resources 的唯一版本化线格式，不进入普通 sandbox_policy。
// required_sandbox_project_files 只通过 nxs initialize 传递项目定义文件要求。
// required_sandbox_managed_policy 只通过 nxs initialize 要求托管策略保证。
package protocol
