// INPUT: 运行时发送的 MCP tools/call 请求元数据。
// OUTPUT: 本次调用独立的公开 tools.Context；不从业务参数推断身份。
// POS: SDK 托管工具的调用上下文解码边界。
package tools

import "context"

type callContextKey struct{}

// withMCPCallContext 每次请求都替换上下文，防止缺省元数据继承上一层调用身份。
func withMCPCallContext(ctx context.Context, message map[string]any) context.Context {
	call := Context{}
	if message["method"] == "tools/call" {
		params, _ := message["params"].(map[string]any)
		meta, _ := params["_meta"].(map[string]any)
		call.ToolUseID, _ = meta["claudecode/toolUseId"].(string)
		for key, value := range meta {
			if text, ok := value.(string); ok {
				if call.Metadata == nil {
					call.Metadata = map[string]string{}
				}
				call.Metadata[key] = text
			}
		}
	}
	return context.WithValue(ctx, callContextKey{}, call)
}

// toolCallContext 返回调用独立副本，工具修改元数据不会影响并发或嵌套调用。
func toolCallContext(ctx context.Context) *Context {
	call, _ := ctx.Value(callContextKey{}).(Context)
	if call.Metadata != nil {
		cloned := make(map[string]string, len(call.Metadata))
		for key, value := range call.Metadata {
			cloned[key] = value
		}
		call.Metadata = cloned
	}
	return &call
}
