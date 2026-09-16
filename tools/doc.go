// Package tools 提供 SDK 托管 custom tool 的公开辅助类型。
//
// custom tool 通过进程内 MCP server 暴露：调用方注册 Go 原生 Tool，再通过
// client.Options 绑定到会话。运行时会按 mcp__<server>__<tool> 命名发现它。
// 简单场景使用 New 或 NewTyped；进程内 MCP server 使用 CreateSDKMCPServer 构造。
// tools/call 的 params._meta["claudecode/toolUseId"] 原样投影到 Context.ToolUseID；
// 不从 arguments、JSON-RPC id 或正文生成调用身份。缺省字段保持为空。
// 宿主 shell 能力使用 NewHostCommandTool 明确挂到 bridge 侧 MCP tool。
package tools
