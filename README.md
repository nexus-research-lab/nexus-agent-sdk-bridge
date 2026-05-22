# Nexus Agent SDK Bridge

English | [简体中文](./README_zh.md)

## Installation

```bash
go get github.com/nexus-research-lab/nexus-agent-sdk-bridge@latest
```

Requires Go 1.24 or later.

## Quick Start

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
        Prompt:  "Summarize this project in one sentence.",
        Options: client.NewOptions().WithCWD("."),
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Result)
}
```

## Session Management

Use `client.NewSession` to create a persistent session with streaming message delivery and runtime control:

```go
session, err := client.NewSession(ctx, client.NewOptions().WithCWD("."))
if err != nil {
    return err
}
defer session.Close(ctx)

stream, err := session.Send(ctx, "Prepare a concise implementation plan.")
if err != nil {
    return err
}

result, err := stream.Result(ctx)
if err != nil {
    return err
}
fmt.Println(result.Result)
```

### Streaming Output

Use `stream.Recv()` to read messages one by one and handle text, tool calls, and other content in real time:

```go
stream, err := session.Send(ctx, "Explain the logic of this code.")
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

### Streaming Input

Pass a message channel via `QueryRequest.Messages` or `PromptRequest.Messages` to send messages continuously during a session:

```go
messages := make(chan protocol.OutboundMessage, 16)

go func() {
    messages <- protocol.NewUserTextMessage("Step 1: Analyze the requirements.")
    messages <- protocol.NewUserTextMessage("Step 2: Propose a solution.")
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

### Runtime Primitives

Hosts can attach generic metadata to outbound messages without changing normal `Send` behavior:

```go
stream, err := session.SendWithOptions(ctx, "Continue the internal task.", protocol.OutboundMessageOptions{
    Meta:           true,
    Synthetic:      true,
    HiddenFromUser: true,
    Purpose:        "host_continuation",
})
```

Result messages expose helper methods for host-side orchestration:

```go
usage, ok := result.TokenUsage()
category := result.TerminalCategory()
```

`session.Supports(client.CapabilityInternalContext)` reports whether the backend can inject true internal context. When unsupported, `session.Control().SetNextTurnContext(...)` returns `client.ErrUnsupportedCapability`; hosts should choose their own fallback policy.

## Backend Modes

The SDK provides a stable public API regardless of the underlying transport:

| Backend | Description |
|---------|-------------|
| Process (default) | Spawns a local bridge subprocess automatically |
| `client.ProcessBackend` | Starts a local process with an explicit executable path |
| `client.DirectConnectBackend` | Connects to a remote bridge endpoint (`cc://host:port/token`) |
| `client.TransportBackend` | Injects a custom transport for testing or host-managed runtimes |

```go
options := client.NewOptions().
    WithBackend(client.DirectConnectBackend(client.DirectConnectOptions{
        URL:                  "cc://127.0.0.1:54321/token",
        SessionKey:           "project-session",
        DeleteSessionOnClose: true,
    })).
    WithCWD(".")
```

## Configuration

All options are configured through the `client.NewOptions()` builder.

### System Prompt

```go
client.NewOptions().
    WithSystemPrompt("You are a code review assistant.").
    WithAppendSystemPrompt("Always respond in Chinese.")
```

### Custom Tools

Define tools in Go via the `tools` package. Registered tools are exposed as MCP tools for the agent to call:

```go
searchTool, _ := tools.NewTyped[SearchInput](
    "search_docs",
    "Search the internal documentation",
    func(ctx context.Context, input SearchInput, tc tools.ToolContext) (tools.Result, error) {
        results := search(input.Query)
        return tools.Text(results), nil
    },
    tools.ReadOnly(),
)

client.NewOptions().WithCustomTools("my-server", searchTool)
```

### MCP Servers

Supports external MCP servers (stdio / SSE / HTTP / WebSocket) and in-process SDK servers:

```go
client.NewOptions().
    WithMCPServer("github", mcp.StdioServerConfig{
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-github"},
        Env:     map[string]string{"GITHUB_TOKEN": token},
    }).
    WithSDKMCPServer("internal", myInProcessServer)
```

### Permissions

Configure permission modes via the `permission` package, with support for real-time tool call interception:

```go
client.NewOptions().
    WithPermissionMode(permission.ModeAcceptEdits).
    WithPermissionHandler(func(ctx context.Context, req permission.Request) (permission.Decision, error) {
        return permission.Allow(nil, nil), nil
    })
```

Permission mode can be changed at runtime via `session.Control()`:

```go
session.Control().SetPermissionMode(ctx, permission.ModeBypassPermissions)
```

### Hooks

Register callbacks at key points in the agent lifecycle — before/after tool calls, session start/end, context compaction, and more:

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

### Model Settings

```go
client.NewOptions().
    WithThinking(true).
    WithMaxThinkingTokens(10000).
    WithMaxBudgetUSD(5.0)
```

### Runtime Control

After a session is established, you can dynamically adjust the model, check context usage, and manage MCP servers:

```go
session.Control().SetModel(ctx, "claude-sonnet-4-6")
usage, _ := session.Control().ContextUsage(ctx)
session.MCP().SetServers(ctx, newServerConfig)
session.MCP().Status(ctx)
```

## Callbacks

Wire up callbacks via `client.Options` to participate in the agent's decision-making process:

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

## Package Layout

| Package | Description |
|---------|-------------|
| `client` | Session management, queries, backend selection, option builder, and callbacks |
| `protocol` | Message format definitions — content blocks, control requests, and agent definitions |
| `mcp` | MCP server configuration (stdio / SSE / HTTP / WebSocket / in-process) |
| `tools` | Go-native custom tool definitions and result helpers |
| `permission` | Permission modes, requests, decisions, and permission updates |
| `hook` | Hook events, matchers, and callback signatures |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NEXUS_CONFIG_DIR` | SDK configuration root directory (default: `~/.nexus`) |

Session transcripts are stored under `~/.nexus/projects/` by default.

## Development & Release

```bash
go test ./...
```
