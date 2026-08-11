# nxs Runtime Path Inspection

This package inspects an externally supplied `nxs` executable. It does not
contain, download, update, or build the closed-source runtime.

Hosts must set `NEXUS_NXS_COMMAND_PATH=/path/to/nxs` before startup, or pass an
explicit path through `client.Options.WithCLIPath`.

`InspectRuntime` and `EnsureRuntime` only verify that the configured path exists
and is executable. They do not access the network, scan application bundles,
inspect caches, or fall back to `PATH`.

See the [runtime contract](../../docs/runtime-contract.md) for the full boundary.
