# nxs Runtime Path Inspection

This package inspects an externally supplied `nxs` executable. It does not
contain, download, update, or build the closed-source runtime.

Hosts must set `NEXUS_NXS_COMMAND_PATH=/path/to/nxs` before startup, or pass an
explicit path through `client.Options.WithCLIPath`.

`InspectRuntime` and `EnsureRuntime` only verify that the configured path exists
and is executable. They do not access the network, scan application bundles,
inspect caches, or fall back to `PATH`.

See the [runtime contract](../../docs/runtime-contract.md) for the full boundary.

## Native sandbox diagnostics

`NewRuntimeInspector().SandboxStatus(ctx)` runs the exact configured executable
with `--sandbox-status`, without a shell, path fallback, download or model request.
The query has a five-second deadline (or the caller's earlier cancellation), a
16 KiB stdout limit and a bounded pipe-drain wait. Relative configured paths are
made absolute before execution so they cannot resolve to a different PATH entry.

The version-1 result contains `platform`, `backend_supported`,
`dependencies_available` and an optional `unavailable_reason`. These report the
runtime's native backend and default environment dependencies, not user settings,
policy enforcement, isolation acceptance or `required_sandbox_v1` negotiation.
Old runtimes, failed probes and invalid responses return an error: show unknown,
without changing the independent `Status().Available` result or automatically
switching permission mode. The diagnostic executable is trusted host software;
this query is not a sandbox for arbitrary programs and does not prove descendant
cleanup for a malfunctioning executable.
