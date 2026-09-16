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

Hosts can use `Session.Control().ControlSubagent` after negotiating `CapabilitySubagentControl` with nxs. This control runs within an active parent MCP call; see the [runtime contract](docs/runtime-contract.md#subagent-control). Claude Code does not provide this extension.

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
When `client.CapabilityMessageExecutionPolicy` is negotiated, hosts may use
`OutboundMessageOptions.ToolAccess = "none"`, `MaxOutputTokens`, and
`SkipAutoMemory` to restrict one turn; unsupported runtimes must be rejected
rather than treated as safely restricted.
`Session.Control().SetNextTurnContext` accepts internal context blocks. The
bridge orders and binds them to the next user message. NXS retains the reminder
in live model history without writing it to the transcript. Claude Code's public
stdin schema has no attachment input, so the bridge returns it as native
`UserPromptSubmit` hook `additionalContext`; Claude Code creates the attachment.
`OutboundMessageOptions.MessageUUID` lets a host assign the transcript identity
needed to remove an uncommitted turn and its emitted messages with
`Session.Control().RemoveMessages`.
Use `client.ForkSession(ctx, sourceSessionID, completedMessageID, options)` to
start an independent session at an exact completed message boundary. Both
`nxs` and Claude Code advertise `client.CapabilitySessionFork`.
Hosts that run the child under another OS identity can use
`WithProcessSignalHandler` as the trusted, PID-validating boundary for
interrupt, shutdown, and descendant cleanup.

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
| `agent` | Sole public source of subagent configuration types |
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

Automatic permission review (`permission_mode=auto`) uses negotiated `auto_review_v1` on nxs and native mode confirmation on Claude Code. Claude owns its classifier and rejection behavior; unsupported or unconfirmed mode changes return an error. See [runtime contract](docs/runtime-contract.md).

SDK-hosted tools preserve `params._meta["claudecode/toolUseId"]` as `tools.Context.ToolUseID`. Missing metadata stays empty; business arguments and JSON-RPC request IDs are never treated as tool-use identity.
