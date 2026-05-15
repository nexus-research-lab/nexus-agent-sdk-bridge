# Nexus Agent SDK Bridge

[English](./README.md) | 简体中文

## 安装

```bash
go get github.com/nexus-research-lab/nexus-agent-sdk-bridge@latest
```

要求 Go 1.24 及以上版本。

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func main() {
    ctx := context.Background()

    result, err := client.Prompt(ctx, client.PromptRequest{
        Prompt:  "用一句话概括这个项目。",
        Options: client.NewOptions().WithCWD("."),
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Result)
}
```

## 会话管理

使用 `client.NewSession` 创建多轮会话，支持流式消息接收和运行时控制：

```go
session, err := client.NewSession(ctx, client.NewOptions().WithCWD("."))
if err != nil {
    return err
}
defer session.Close(ctx)

stream, err := session.Send(ctx, "给出一份简洁的实现计划。")
if err != nil {
    return err
}

result, err := stream.Result(ctx)
if err != nil {
    return err
}
fmt.Println(result.Result)
```

### 流式输出

通过 `stream.Recv()` 逐条读取 Agent 返回的消息，实时处理文本、工具调用等内容：

```go
stream, err := session.Send(ctx, "解释这段代码的逻辑。")
if err != nil {
    return err
}

for {
    msg, err := stream.Recv(ctx)
    if err != nil {
        break
    }
    switch msg.Type {
    case protocol.MessageTypeAssistant:
        for _, block := range msg.Assistant.Content {
            if text, ok := protocol.AsTextBlock(block); ok {
                fmt.Print(text.Text)
            }
        }
    case protocol.MessageTypeResult:
        fmt.Println("\n--- done ---")
        return
    }
}
```

### 流式输入

通过 `QueryRequest.Messages` 或 `PromptRequest.Messages` 传入消息 channel，支持在会话过程中持续发送消息：

```go
messages := make(chan protocol.OutboundMessage, 16)

go func() {
    messages <- protocol.NewUserTextMessage("第一步：分析需求。")
    messages <- protocol.NewUserTextMessage("第二步：给出方案。")
    close(messages)
}()

stream, err := client.Query(ctx, client.QueryRequest{
    Messages: messages,
    Options:  client.NewOptions().WithCWD("."),
})
if err != nil {
    return err
}
defer stream.Close(ctx)

result, err := stream.Result(ctx)
if err != nil {
    return err
}
fmt.Println(result.Result)
```

## 连接方式

SDK 对外提供统一的 API，底层传输方式根据部署形态决定：

| 后端 | 说明 |
|------|------|
| Process（默认） | 自动启动本地 bridge 子进程 |
| `client.ProcessBackend` | 指定可执行文件路径启动本地进程 |
| `client.DirectConnectBackend` | 连接远端 bridge 端点，地址格式 `cc://host:port/token` |
| `client.TransportBackend` | 注入自定义 Transport，用于测试或自管运行时 |

```go
options := client.NewOptions().
    WithBackend(client.DirectConnectBackend(client.DirectConnectOptions{
        URL:                  "cc://127.0.0.1:54321/token",
        SessionKey:           "project-session",
        DeleteSessionOnClose: true,
    })).
    WithCWD(".")
```

## 配置项

所有配置通过 `client.NewOptions()` 链式构建。

### 系统提示词

```go
client.NewOptions().
    WithSystemPrompt("你是一个代码审查助手。").
    WithAppendSystemPrompt("始终用中文回答。")
```

### 自定义工具

通过 `tools` 包以纯 Go 定义工具，注册后作为 MCP 工具供 Agent 调用：

```go
searchTool, _ := tools.NewTyped[SearchInput](
    "search_docs",
    "搜索内部文档",
    func(ctx context.Context, input SearchInput, tc tools.ToolContext) (tools.Result, error) {
        results := search(input.Query)
        return tools.Text(results), nil
    },
    tools.ReadOnly(),
)

client.NewOptions().WithCustomTools("my-server", searchTool)
```

### MCP 服务器

支持外部 MCP 服务器（stdio / SSE / HTTP / WebSocket）和进程内 SDK Server：

```go
client.NewOptions().
    WithMCPServer("github", mcp.StdioServerConfig{
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-github"},
        Env:     map[string]string{"GITHUB_TOKEN": token},
    }).
    WithSDKMCPServer("internal", myInProcessServer)
```

### 权限管理

通过 `permission` 包配置权限模式，并支持实时拦截工具调用请求：

```go
client.NewOptions().
    WithPermissionMode(permission.ModeAcceptEdits).
    WithPermissionHandler(func(ctx context.Context, req permission.Request) (permission.Decision, error) {
        return permission.Allow(nil, nil), nil
    })
```

运行时可通过 `session.Control()` 动态调整权限模式：

```go
session.Control().SetPermissionMode(ctx, permission.ModeBypassPermissions)
```

### Hook

支持在 Agent 生命周期的关键节点注册回调，包括工具调用前后、会话开关、上下文压缩等：

```go
client.NewOptions().AddHookMatcher(hook.EventPreToolUse, hook.Matcher{
    Matcher: "Bash",
    Hooks: []hook.Callback{
        func(ctx context.Context, input hook.Input, id string) (hook.Output, error) {
            return hook.Output{}, nil
        },
    },
})
```

### 模型配置

```go
client.NewOptions().
    WithThinking(true).
    WithMaxThinkingTokens(10000).
    WithMaxBudgetUSD(5.0)
```

### 运行时控制

会话建立后支持动态调整模型、查看上下文用量、管理 MCP 服务器：

```go
session.Control().SetModel(ctx, "claude-sonnet-4-6")
usage, _ := session.Control().ContextUsage(ctx)
session.MCP().SetServers(ctx, newServerConfig)
session.MCP().Status(ctx)
```

## 回调接口

通过 `client.Options` 的回调方法接入 Agent 运行过程中的外部交互：

```go
client.NewOptions().
    WithPermissionHandler(myPermissionHandler).
    WithElicitationHandler(myElicitationHandler).
    WithUserDialogHandler(myDialogHandler).
    WithOAuthTokenHandler(myOAuthHandler).
    WithStderr(func(line string) {
        log.Printf("[agent] %s", line)
    })
```

## 包结构

| 包 | 说明 |
|----|------|
| `client` | 会话管理、查询接口、后端选择、配置构建与回调 |
| `protocol` | 消息格式定义，包括内容块、控制请求与 Agent 定义 |
| `mcp` | MCP 服务器配置（stdio / SSE / HTTP / WebSocket / 进程内） |
| `tools` | Go 原生自定义工具定义与结果构造 |
| `permission` | 权限模式、请求、决策与权限更新 |
| `hook` | Hook 事件、匹配器与回调签名 |

## 环境变量

| 变量 | 说明 |
|------|------|
| `NEXUS_CONFIG_DIR` | SDK 配置根目录，默认 `~/.nexus` |

会话记录默认存储在 `~/.nexus/projects/` 目录下。

## 开发与发布

```bash
go test ./...
```