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

### 运行时原语

宿主可以给出站消息附加通用元数据，不影响普通 `Send` 行为：

```go
stream, err := session.SendWithOptions(ctx, "继续内部任务。", protocol.OutboundMessageOptions{
    Meta:           true,
    Synthetic:      true,
    HiddenFromUser: true,
    Purpose:        "host_continuation",
})
```

`ResultMessage` 提供 host 编排可用的 helper：

```go
usage, ok := result.TokenUsage()
category := result.TerminalCategory()
```

`session.Control().SetNextTurnContext(...)` 会把运行时拥有的上下文注入到下一轮用户输入中。对于 Claude Code transport，bridge 会将其映射成一次性的 `<system-reminder>` 文本块，让模型能使用这段上下文，但不把它当成用户亲自发出的指令。

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
    case protocol.MessageTypeTaskProgress:
        fmt.Printf("task %s: %s\n", msg.TaskProgress.TaskID, msg.TaskProgress.Summary)
    case protocol.MessageTypeResult:
        fmt.Println("\n--- done ---")
        return
    }
}
```

Task 事件使用强类型消息：`protocol.MessageTypeTaskStarted`、
`protocol.MessageTypeTaskProgress` 和 `protocol.MessageTypeTaskNotification`。
官方 system subtype 形态仍可从 `msg.System.Task*` 读取；顶层 task 事件使用
`msg.TaskStarted`、`msg.TaskProgress` 或 `msg.TaskNotification`。
task progress 和 notification 共用 `protocol.TaskUsage`。

### 宿主定时任务

宿主 daemon 可以观察 `.nexus/scheduled_tasks.json`，不用把 polling loop 放回 SDK core：

```go
handle, err := client.WatchScheduledTasks(ctx, client.WatchScheduledTasksOptions{
    Dir: ".",
})
if err != nil {
    return err
}

for event := range handle.Events() {
    if event.Type == client.ScheduledTaskMissed {
        prompt := client.BuildMissedTaskNotification(event.Tasks)
        _ = prompt
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

### 会话历史

本地会话记录默认存储在 `~/.nexus/projects/`。查询和修改使用不同 options：
读取入口使用 `SessionLookupOptions`，标题/标签修改使用 `SessionMutationOptions`。

```go
info, err := client.GetSessionInfo(sessionID, client.SessionLookupOptions{})
if err != nil {
    return err
}
_ = info

if err := client.RenameSession(sessionID, "review notes", client.SessionMutationOptions{}); err != nil {
    return err
}
tag := "review"
if err := client.TagSession(sessionID, &tag, client.SessionMutationOptions{}); err != nil {
    return err
}
```

### 取消语义

用户中断或 context 主动取消会返回可用 `client.ErrAborted` 判定的错误。
同时保留 `context.Canceled`，调用方可以同时判断两个 sentinel：

```go
if errors.Is(err, client.ErrAborted) || errors.Is(err, context.Canceled) {
    return nil
}
```

## Transport

SDK 按官方 Agent SDK 的形态收口：选项负责配置本地 CLI；如果宿主管理连接，则直接注入自定义 `Transport`。

| 模式 | 配置 |
|------|------|
| Nexus 原生 runtime（默认） | `NEXUS_NXS_COMMAND_PATH=/path/to/nxs`，或 `client.NewOptions().WithCLIPath("/path/to/nxs")` |
| Claude Code runtime | `client.NewOptions().WithRuntime(client.RuntimeClaude)` |
| 指定 CLI 路径 | `client.NewOptions().WithCLIPath("/path/to/nxs")` |
| JavaScript 运行时包装 | `client.NewOptions().WithPathToClaudeCodeExecutable("/path/to/cli.js").WithExecutable("node")` |
| Direct connect | `client.NewOptions().WithDirectConnect(client.DirectConnectOptions{...})` |
| 宿主管理 transport | `client.NewOptions().WithTransport(transport)` |

默认启动 Nexus 原生 runtime，但路径必须明确：打包后的 Nexus 宿主会在启动 bridge 前写入 `NEXUS_NXS_COMMAND_PATH`；开发环境也应设置同一个变量，或通过 `WithCLIPath` 传入路径。

```go
options := client.NewOptions().
    WithCLIPath("/path/to/nxs").
    WithCWD(".")
```

bridge 运行期不下载 `nxs`，不扫描 app root，不读取缓存，也不回退到 PATH。`NEXUS_NXS_COMMAND_PATH` 缺失或不可执行时，会直接作为启动配置错误返回。

Claude Code 仍作为显式兼容 runtime 保留：

```go
options := client.NewOptions().
    WithRuntime(client.RuntimeClaude).
    WithCWD(".")
```

当宿主自己管理 runtime 进程时，Direct connect 仍是独立配置：

```go
options := client.NewOptions().
    WithDirectConnect(client.DirectConnectOptions{
        URL:                  "cc://127.0.0.1:54321/token",
        SessionKey:           "project-session",
        DeleteSessionOnClose: true,
    }).
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

### 运行时设置

inline settings、sandbox、debug、固定 session ID 和 resume 截断点都通过 `client.Options` 设置，并会转成 process bridge 参数：

```go
enabled := true

client.NewOptions().
    WithSessionID("00000000-0000-0000-0000-000000000001").
    WithResumeSessionAt("11111111-1111-1111-1111-111111111111").
    WithSettingsObject(map[string]any{"model": "sonnet"}).
    WithSandbox(client.SandboxSettings{Enabled: &enabled}).
    WithDebugFile("/tmp/nexus-agent-sdk.log")
```

### 自定义工具

通过 `tools` 包以纯 Go 定义工具，注册后作为 MCP 工具供 Agent 调用：

```go
searchTool, _ := tools.NewTyped[SearchInput](
    "search_docs",
    "搜索内部文档",
    func(ctx context.Context, input SearchInput, tc *tools.Context) (tools.Result, error) {
        results := search(input.Query)
        return tools.Text(results), nil
    },
    tools.ReadOnly(),
)

client.NewOptions().WithCustomTools("my-server", searchTool)
```

原来依赖 SDK core env-command fallback 的宿主能力，应放在 bridge 侧，并显式暴露成 MCP 工具：

```go
sendMessage, _ := tools.NewHostCommandTool(tools.HostCommandOptions{
    Name:        "host_send_user_message",
    Description: "通过宿主 bridge 发送可见消息",
    Command:     "/opt/nexus/bin/send-user-message",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "message": map[string]any{"type": "string"},
        },
        "required": []any{"message"},
    },
})

client.NewOptions().WithCustomTools("host", sendMessage)
```

命令通过 stdin 接收工具参数 JSON；stdout 可以返回完整 MCP tool result JSON、作为
`structuredContent` 的普通 JSON 对象，或纯文本。

### MCP 服务器

支持外部 MCP 服务器（stdio / SSE / HTTP）和进程内 SDK Server：

```go
client.NewOptions().
    WithMCPServer("github", mcp.StdioServerConfig{
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-github"},
        Env:     map[string]string{"GITHUB_TOKEN": token},
    }).
    WithSDKMCPServer("internal", myInProcessServer)
```

Go 原生进程内 server 通过 `tools` 包构造：

```go
server := tools.CreateSDKMCPServer(tools.SDKMCPServerOptions{
    Name:  "internal",
    Tools: []tools.Tool{searchTool},
})

client.NewOptions().WithSDKMCPServer("internal", server)
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
commands, _ := session.Control().SupportedCommands(ctx)
_ = commands
_ = session.MCP().Reconnect(ctx, "github")
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
| `client` | 会话管理、查询接口、执行连接、配置构建、回调、Agent 定义与运行期控制结果 |
| `protocol` | 流式消息、内容块、出站消息与 control wire 模型 |
| `mcp` | MCP 服务器配置、SDK server 接口与 MCP 状态结果 |
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
