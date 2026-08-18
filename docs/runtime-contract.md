# Runtime Contract

## Boundary

`nexus-agent-sdk-bridge` is the open-source process and protocol boundary
between a Go host and an Agent runtime.

```text
Go host
  -> bridge client
       -> runtime process or host-managed transport
            -> stream-json messages and controls
```

The bridge owns process startup, transport lifecycle, typed messages, control
requests, hooks, permission callbacks, and in-process MCP integration. It does
not implement the agent loop, call model providers, or contain the source code
for the native `nxs` runtime.

## Runtime selection

| Runtime | Selection | Command resolution |
| --- | --- | --- |
| `nxs` | Default, or `WithRuntime(client.RuntimeNXS)` | Explicit `WithCLIPath` or `NEXUS_NXS_COMMAND_PATH` |
| Claude Code | `WithRuntime(client.RuntimeClaude)` | Explicit path, `NEXUS_CLAUDE_COMMAND_PATH`, or safe platform discovery |
| Direct connect | `WithDirectConnect(...)` | Host owns the remote runtime process |
| Custom transport | `WithTransport(...)` | Host owns the full connection |

The bridge does not download `nxs`, search application bundles, inspect caches,
or fall back to `PATH` for the native runtime. Official Nexus distributions
provide the closed-source `nxs` executable separately and pass its path to the
bridge.

## Wire format

Process runtimes exchange line-delimited JSON over stdio using the public
`stream-json` contract in `protocol/`. Native `nxs` and Claude Code share the
same mixed-casing control wire.

Compatibility aliases are declared field by field where runtime payloads
differ. The bridge never applies a global snake_case or camelCase conversion to
control messages, tool arguments, hook inputs, or provider payloads.

## Capability negotiation

Hosts must call `Session.Supports` before exposing optional controls. Common
capabilities include typed usage, terminal categories, task stopping,
in-process MCP, exact-boundary session forking, and provider-neutral runtime
lifecycle events. Native-only controls currently include task follow-up,
environment hot updates, and AutoDream. Hook response acknowledgement is
negotiated during initialization.

Capability values are defined in [`client/capability.go`](../client/capability.go).
Runtime names are not a substitute for capability checks.

## Session lifecycle

1. `client.NewSession` resolves a transport and initializes the runtime.
2. `Session.Send` or `SendWithOptions` starts a turn.
3. The host consumes typed messages with `Recv` or waits for `Result`.
4. Controls use the active session and preserve the runtime request identity.
5. `Session.Close` releases the transport and owned process resources.

`client.ForkSession` creates an independent target from a source session through
the exact supplied message ID. Claude Code may not persist the target transcript
until its first user turn, but the bridge assigns the target session ID before
returning the session.

Context cancellation and user aborts remain distinguishable through Go error
matching. Long-running controls propagate cancellation with the same
`request_id`, allowing the runtime to cancel the original operation.

## Security responsibilities

- The host decides which runtime executable and environment to trust.
- The bridge passes sandbox policy but does not claim to enforce a sandbox.
- Direct process signals assume the host and runtime share an OS identity. A
  host that crosses this boundary must provide `WithProcessSignalHandler` and
  validate PID ownership before forwarding lifecycle signals.
- Generated or inline MCP configuration is materialized in restricted argument
  files rather than exposed directly in process arguments.
- Provider credentials remain runtime process environment values; the bridge
  does not interpret provider request payloads.

Runtime enforcement and product authorization remain outside this library.

## Compatibility policy

Public Go packages follow repository releases. `internal/` packages are not
supported imports. New runtime-specific behavior must first receive a public
capability and a typed protocol shape; hosts should not branch on undocumented
payload fields.
