# Changelog

All notable changes to this project are documented in this file.

## Unreleased

## [0.1.2] - 2026-05-30

### Changed

- Added `client.ErrAborted` as the Go sentinel equivalent of the official SDK `AbortError`; cancellation and interrupt paths keep `context.Canceled` discoverable through `errors.Is`.
- Added `client.SessionMutationOptions` and changed `RenameSession` / `TagSession` to use it instead of lookup options, matching the native Go SDK API shape.

## [0.1.1] - 2026-05-29

### Changed

- Moved public agent definitions and runtime control result types out of `protocol`: agent definitions and control results now belong to `client`, while MCP status and set-servers results belong to `mcp`.
- Kept runtime initialization, slash command, model, agent, account, and plugin reload snapshots internal to the bridge client lifecycle instead of exporting them as protocol API.
- Exposed runtime initialization snapshots, supported commands/models/agents, account info, task stop, flag settings, streaming input, and MCP reconnect/toggle through `client.Session` capability objects.
- Moved SDK MCP server and typed tool builders fully into `tools`; `mcp` now only carries MCP server configuration, status models, and the SDK server interface.
- Added official-style process options for explicit runtime executable wiring, fixed session IDs, resume-at-message, sandbox inline settings, structured settings, debug/debug-file, load timeout aliases, and built-in tool configuration validation.
- Replaced the old execution selector layer with official-style transport configuration: `WithCLIPath`, `WithTransport`, and `WithDirectConnect`. Error types now use `CLINotFoundError`, `CLIConnectionError`, `ProcessError`, and `CLIJSONDecodeError`.
- Reorganized `client` internals so public session facades, active session lifecycle, control RPC, session history, transport config, and option resolution each live in focused files.
- Narrowed public protocol message models to official SDK-facing messages while keeping task progress and notification messages strongly typed; bridge-private stream markers now remain available through raw payloads without becoming exported API.
- Removed host adapter and product callback types from the public `tools` package so it only represents SDK-hosted custom tools and SDK MCP helpers.
- Removed the unsupported WebSocket MCP config from the public bridge API; supported server config types are stdio, SSE, HTTP, and SDK-hosted servers.
- Aligned public hook event constants with the installed official SDK hook table, including `PostCompact`, `TaskCreated`, `Elicitation`, `ElicitationResult`, `InstructionsLoaded`, `CwdChanged`, and `FileChanged`, while dropping non-public bridge-specific event constants.

## [0.1.0] - 2026-05-15

Initial public release of the Nexus Agent SDK Bridge for Go.

### Added

- Go client API for one-shot queries, streaming sessions, session resume, and runtime control.
- Transport configuration for local process, direct-connect, and host-provided transports.
- Protocol models for streamed messages, content blocks, control operations, and structured results.
- MCP configuration helpers and in-process SDK MCP server support.
- Go-native custom tool definitions, permission handling, hook callbacks, diagnostics, and host integration callbacks.
- Tag-based GitHub Release workflow with changelog-driven release notes.

### Notes

- Releases are versioned as Go module tags, with a source archive and checksum attached to GitHub Release.
- The bridge package focuses on host-facing SDK integration; Nexus native runtime internals remain outside this module.
