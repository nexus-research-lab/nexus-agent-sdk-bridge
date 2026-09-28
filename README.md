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


Hosts may set `SandboxSettings.RequireSandbox` to require `required_sandbox_v1` from nxs before sending a task. Unsupported runtimes fail connection. This guarantees required execution handling, not platform backend availability; see [the contract](docs/runtime-contract.md#required-sandbox-execution).

Add `RequireFileTools: true` to require the separate `sandbox_file_tools_v1` contract for native Read/Write/Edit. A command-only runtime is rejected before receiving a task. This does not claim that all SDK IO is isolated; see [file-tool scope](docs/runtime-contract.md#native-file-tool-confinement).

Add `RequireSearchTools: true` alongside both requirements to require `sandbox_search_tools_v1` for Glob/Grep path checks, ripgrep and result metadata. Older file-only runtimes are rejected before tasks; see [search scope](docs/runtime-contract.md#search-tool-confinement).

Add `RequireMediaNetwork: true` with command, file and local media requirements to require per-request remote-image network checks, controlled redirects and URL materialization. See [remote image networking](docs/runtime-contract.md#remote-image-networking).

Add `RequireMediaFiles: true` with the command and file requirements to require local image confinement for ViewImage and model preprocessing. Older runtimes are rejected before task writes. Remote image networking is separate; see [media scope](docs/runtime-contract.md#local-media-file-confinement).

Add `RequireContextFiles: true` with the command and file requirements for startup instructions and compact file restoration. See [context scope](docs/runtime-contract.md#context-file-confinement).

Add `RequireSkillFiles: true` with the command and file requirements for Skill catalogs, bodies, dynamic discovery, Git ignore queries and memory-availability settings. Startup settings, hooks and background IO remain separate; see [Skill scope](docs/runtime-contract.md#skill-file-confinement).

`SandboxSettings.Resources` additionally requires `sandbox_resources_v1` to select a workspace write scope and a host-prepared private scratch directory. See [resource scope](docs/runtime-contract.md#host-resource-write-scope) for current macOS support and lifecycle limits.

For Claude Code's own restricted mode, set `SandboxSettings.RequireClaudeRestricted: true`. Bridge adds and checks the typed `--restricted` launch contract only for `RuntimeClaude`; Full Access does not require it. This is not nxs `required_sandbox_v1` and does not prove complete Claude SDK IO isolation; see [Claude native restricted launch](docs/runtime-contract.md#claude-native-restricted-launch).

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
`client.ProcessCleanupError` preserves cleanup failures through `Wait` and repeated
`Close`, including forced termination. The built-in Unix sweep covers visible
members of the original session. Windows runtimes and CLI probes start suspended
and resume only after joining a per-runtime kill-on-close Job Object. Native tests
cover immediate descendants, cancellation with inherited pipes and host termination
after admission. A host crash before Job assignment can still leave a suspended
process; atomic creation and durable resource recovery still require host/platform
evidence. See [lifecycle limits](docs/runtime-contract.md#session-lifecycle).

An internal macOS process-scope component now tests exact termination of detached
descendants and kernel coalition retirement. Its control-connection registration
compares the kernel peer audit identity against the expected launcher process;
launcher/job authentication remains the caller’s responsibility. It is not connected to default runtime
launch or cleanup, and does not change the lifecycle guarantees above. Its native
API requirements and macOS 14.0 support remain separate integration work.

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

Hosts can call `nxs.NewRuntimeInspector().SandboxStatus(ctx)` to query the configured runtime for versioned native backend diagnostics. This bounded local query is separate from runtime-path availability and does not prove effective isolation. See [runtime inspection](runtimes/nxs/README.md).

Add `RequireProjectFiles: true` with command and file requirements for project definitions and hook-setting reads. See [project file scope](docs/runtime-contract.md#project-definition-file-confinement).

Use `RequireManagedPolicy: true` alongside command and file requirements to require fixed managed-policy sources and integrity checks. See [managed policy scope](docs/runtime-contract.md#managed-policy-integrity).

Use `RequireSettingsFiles: true` to require confined ordinary settings reads and checked snapshots. See [ordinary settings scope](docs/runtime-contract.md#ordinary-settings-files-and-snapshots).

Use `RequireSettingsWrites: true` together with the required sandbox, file-tool and settings-file options to require nxs-controlled Config and permission persistence. Claude Code and older nxs runtimes are rejected before a task is sent. See [ordinary settings write scope](docs/runtime-contract.md#ordinary-settings-writes).

SDK-hosted tools preserve `params._meta["claudecode/toolUseId"]` as `tools.Context.ToolUseID`. Missing metadata stays empty; business arguments and JSON-RPC request IDs are never treated as tool-use identity.

### Remote MCP networking

RequireMCPNetwork / sandbox_mcp_network_v1 requires an explicit MCP configuration (MCP.StrictConfig), mandatory sandboxing, and negotiated nxs support. Configured HTTP and legacy SSE servers receive only their own scheme/host/port grant; redirects and SSE POST endpoints cannot leave that origin. Tool network grants remain independent; explicit denies and managed-only domain policy still apply. Retirement, configuration replacement, shutdown and permission changes cancel the covered requests. Authentication helpers, stdio processes, OAuth discovery and model Provider networking remain separate contracts.

`RequireMCPHelpers` / `sandbox_mcp_helpers_v1` independently requires confined macOS authentication helpers, mandatory sandboxing and explicit MCP configuration. Each request refreshes credentials under the command resource policy and filtered task environment; it cannot borrow the MCP endpoint grant. Execution/output are bounded, policy changes cancel helpers, shutdown waits for cleanup, and failures do not fall back to stale/static credentials. Stdio and detached-descendant supervision remain separate contracts.

`RequireMCPStdio` / `sandbox_mcp_stdio_v1` separately confirms explicit macOS stdio MCP execution under the current command sandbox. It requires mandatory sandboxing and strict MCP configuration. Cancellation retires the whole service and all its pending calls; replacement waits for the old named process, and session shutdown awaits owned process cleanup. Requests are not replayed. Inherited model Provider credentials are filtered before explicit service credentials are applied; service environment cannot override reserved runtime, home or temporary-root inputs. Network access uses the command policy, without an endpoint or tool-approval grant. JSONL messages are capped at 10 MiB, pending calls at 64, and stderr is drained without exposing credentials. Ordinary process groups are covered; independently detached descendants and host-crash recovery require separate evidence.
