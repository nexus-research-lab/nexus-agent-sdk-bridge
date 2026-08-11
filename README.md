# Nexus Agent SDK Bridge

English | [简体中文](./README_zh.md)

Open-source Go client and protocol contract for connecting a host application
to a local Agent runtime over `stream-json`.

```text
Host application -> nexus-agent-sdk-bridge -> runtime process
```

The bridge starts or connects to a runtime, streams typed messages, and exposes
runtime controls. It does not implement the agent loop or include a model
runtime.

## Requirements

- Go 1.24 or later
- One runtime:
  - Claude Code installed separately, or
  - an `nxs` executable supplied by an official Nexus distribution or another
    authorized source

The native `nxs` runtime is closed source and is not included, downloaded, or
built by this repository.

## Install

```bash
go get github.com/nexus-research-lab/nexus-agent-sdk-bridge@latest
```

## Choose a runtime

| Runtime | Configuration |
| --- | --- |
| Claude Code | `client.NewOptions().WithRuntime(client.RuntimeClaude)` |
| Native `nxs` | `NEXUS_NXS_COMMAND_PATH=/path/to/nxs` or `WithCLIPath(...)` |
| Direct connect | `WithDirectConnect(...)` |
| Host-managed transport | `WithTransport(...)` |

`nxs` is the default runtime kind, so standalone programs must provide its
command path. Claude Code is always an explicit compatibility runtime.

## Quick start with Claude Code

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
	options := client.NewOptions().
		WithRuntime(client.RuntimeClaude).
		WithCWD(".")

	result, err := client.Prompt(ctx, client.PromptRequest{
		Prompt:  "Summarize this project in one sentence.",
		Options: options,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Result)
}
```

Claude Code discovery uses a native executable when available and safe
platform-specific wrappers otherwise. Set `NEXUS_CLAUDE_COMMAND_PATH` or
`WithCLIPath` to bypass discovery.

## Persistent sessions

```go
session, err := client.NewSession(ctx, options)
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

Use `stream.Recv` for incremental messages. Before exposing optional controls,
check `session.Supports(capability)` rather than branching on a runtime name.

## Documentation

- [Documentation index](./docs/README.md)
- [Runtime contract](./docs/runtime-contract.md)
- [Go package reference](https://pkg.go.dev/github.com/nexus-research-lab/nexus-agent-sdk-bridge)
- [Changelog](./CHANGELOG.md)

## Public packages

| Package | Responsibility |
| --- | --- |
| `client` | Queries, sessions, options, transport selection, capabilities, and runtime control |
| `protocol` | Streamed messages, content blocks, lifecycle events, and control wire types |
| `agent` | Public subagent configuration types |
| `hook` | Runtime hook events, matchers, and callbacks |
| `permission` | Permission modes, requests, and decisions |
| `mcp` | MCP configuration and status types |
| `tools` | Go-native MCP tool and result helpers |
| `runtimes/nxs` | Native runtime path inspection without downloading the executable |

Packages under `internal/` are implementation details and are not supported
imports.

## Development

```bash
make test
```

## License

Apache License 2.0 · [LICENSE](./LICENSE)
