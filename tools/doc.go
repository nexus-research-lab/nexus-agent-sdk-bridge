// Package tools provides public helpers for SDK-hosted custom tools and
// headless tool adapter types.
//
// Nexus Agent SDK custom tools are exposed through an in-process MCP server:
// a Go-native Tool is registered on the server, and client.Options attaches
// that server to a session. At runtime the tool is discovered as a dynamic MCP
// tool named mcp__<server>__<tool>. Use New or NewTyped for simple tools, or
// implement Tool directly when the caller needs custom execution semantics. Host
// adapter request, result and handler types for built-in tool boundaries also
// live here so the client package can stay focused on session/query
// orchestration.
package tools
