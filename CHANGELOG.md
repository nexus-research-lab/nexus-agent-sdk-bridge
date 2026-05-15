# Changelog

All notable changes to this project are documented in this file.

## [0.1.0] - 2026-05-15

Initial public release of the Nexus Agent SDK Bridge for Go.

### Added

- Go client API for one-shot queries, streaming sessions, session resume, and runtime control.
- Backend abstraction for local process, direct-connect, and host-provided transports.
- Protocol models for streamed messages, content blocks, control operations, agent definitions, and structured results.
- MCP configuration helpers and in-process SDK MCP server support.
- Go-native custom tool definitions, permission handling, hook callbacks, diagnostics, and host integration callbacks.
- Tag-based GitHub Release workflow with changelog-driven release notes.

### Notes

- Releases are versioned as Go module tags, with a source archive and checksum attached to GitHub Release.
- The bridge package focuses on host-facing SDK integration; Nexus native runtime internals remain outside this module.
