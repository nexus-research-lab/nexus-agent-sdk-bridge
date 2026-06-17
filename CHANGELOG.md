# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- Added `tools.NewHostCommandTool` so bridge-owned MCP servers can expose explicit host command tools for capabilities removed from SDK-core env-command fallbacks.

## [0.1.11] - 2026-06-10

### Changed

- Changed the default runtime to `RuntimeNXS` while keeping Claude Code available through explicit `RuntimeClaude` configuration.
- Unified Claude-compatible transcript project hashing and config root propagation for Nexus native runtime sessions.

### Fixed

- Fixed `RuntimeNXS` command resolution so hosts must provide an explicit `NEXUS_NXS_COMMAND_PATH` or `WithCLIPath` value instead of relying on runtime downloads, app-root scans, or `PATH` fallback.

## [0.1.10] - 2026-06-09

### Fixed

- Fixed Windows process launch options for `RuntimeNXS` sessions that use SDK-hosted MCP servers, preserving the in-memory SDK MCP registry after MCP config arg-file materialization.
- Fixed Windows process command discovery for npm-installed Claude Code shims such as `claude.cmd`.
- Fixed Claude Code 2.x session startup by allowing the first user message to trigger `system init` when the initialize control response does not include a session id.
- Fixed process close/wait teardown so inherited stderr pipes cannot block session cleanup.
- Fixed `nxs` runtime inspector executability checks when tests override the target platform on Windows hosts.

## [0.1.9] - 2026-06-08

### Added

- Added runtime launch snapshots, option fingerprints, and hot reconfiguration support so hosts can inspect and update active runtime sessions without leaking secrets.
- Added the `runtimes/nxs` runtime inspector API for checking packaged, environment-provided, and cached `nxs` executables before triggering downloads.

### Changed

- Moved process runtime command resolution, launch argument construction, and runtime restart decisions into the bridge client layer.

## [0.1.8] - 2026-06-06

### Changed

- Ignored macOS `.DS_Store` metadata files in the repository.

## [0.1.7] - 2026-06-05

### Changed

- Changed the default `RuntimeNXS` resolver to read the `nxs-stable` runtime channel and choose the newest runtime compatible with the linked bridge module version.
- Kept explicit runtime release and manifest overrides available for hosts that need to pin a specific `nxs` runtime.

## [0.1.6] - 2026-06-05

### Changed

- Updated the default `RuntimeNXS` release manifest to `nxs-v0.1.2`.

### Fixed

- Fixed `RuntimeNXS` command resolution so release-backed runtime download/cache failures surface as explicit resolver errors instead of silently falling back to a bare `nxs` command in `PATH`.

## [0.1.5] - 2026-06-04

### Added

- Added `client.NewOptions().WithRuntime(client.RuntimeNXS)` so development hosts can run the native Nexus runtime without depending on the private Go SDK checkout.
- Added a release-backed `nxs` runtime resolver that downloads platform assets from a bridge runtime release manifest, verifies SHA-256, and caches the executable locally.

## [0.1.4] - 2026-06-02

### Changed

- Documented `client.NewOptions().WithCLIPath("nxs")` as the supported way to run the Go-native Nexus runtime executable while keeping default Claude Code command discovery unchanged.

### Fixed

- Fixed process transport teardown so inherited stderr pipes and failed control response writes cannot leave SDK sessions stuck after hook callback stream closure.

## [0.1.3] - 2026-06-01

### Added

- Runtime primitives for long-running host features: typed token usage helpers, terminal result classification, outbound send options, capability detection, and structured stream-close errors.
- Additive session APIs for meta/synthetic outbound messages via `SendWithOptions` and `SendMessageWithOptions`.
- `SetNextTurnContext` now maps runtime-owned next-turn context to a one-shot Claude Code `<system-reminder>` block instead of forcing hosts to prepend their own fallback text.

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
