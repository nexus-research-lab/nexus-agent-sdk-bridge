# Runtime 契约

## 边界

`nexus-agent-sdk-bridge` 是 Go 宿主与 Agent runtime 之间的开源进程和协议边界。

```text
Go 宿主
  -> bridge client
       -> runtime 子进程或宿主管理的 transport
            -> stream-json 消息与控制请求
```

Bridge 负责进程启动、transport 生命周期、类型化消息、控制请求、Hook、权限回调和
进程内 MCP 接入。它不实现 agent loop，不调用模型 Provider，也不包含原生 `nxs`
runtime 的源代码。

## Runtime 选择

| Runtime | 选择方式 | 命令解析 |
| --- | --- | --- |
| `nxs` | 默认，或 `WithRuntime(client.RuntimeNXS)` | 显式 `WithCLIPath` 或 `NEXUS_NXS_COMMAND_PATH` |
| Claude Code | `WithRuntime(client.RuntimeClaude)` | 显式路径、`NEXUS_CLAUDE_COMMAND_PATH` 或安全的平台发现 |
| Direct connect | `WithDirectConnect(...)` | 宿主管理远端 runtime 进程 |
| 自定义 transport | `WithTransport(...)` | 宿主管理完整连接 |

Bridge 不下载 `nxs`，不扫描应用包、缓存或 PATH。官方 Nexus 发布包会单独提供
闭源的 `nxs` 可执行程序，并把明确路径交给 bridge。

## Wire 格式

进程 runtime 通过 stdio 交换行分隔 JSON，公开合同位于 `protocol/`。原生 `nxs`
与 Claude Code 共用同一条 mixed-casing control wire。

Runtime payload 确有差异时，兼容别名按字段声明。Bridge 不会对控制消息、工具参数、
Hook 输入或 Provider payload 做全局 snake_case/camelCase 转换。

## Capability 协商

宿主暴露可选控制前必须调用 `Session.Supports`。通用能力包括类型化 usage、终态分类、
停止任务、进程内 MCP、精确边界 Session fork 和 provider-neutral runtime lifecycle。
当前原生专属控制包括任务续聊、环境热更新和 AutoDream。Hook response ack 在初始化
阶段协商。原生 runtime 还可协商 `CapabilityMessageExecutionPolicy`，让宿主对单条消息
执行 `tool_access=none` 与 `max_output_tokens`，无需复制 Agent 的普通 allow/deny 规则。

Capability 真相源位于 [`client/capability.go`](../client/capability.go)。不能用 Runtime
名称替代 capability 检查。

## Session 生命周期

1. `client.NewSession` 解析 transport 并初始化 runtime。
2. `Session.Send` 或 `SendWithOptions` 启动一轮执行。
3. 宿主通过 `Recv` 消费类型化消息，或通过 `Result` 等待终态。
4. 控制请求复用活跃 session，并保留 runtime request identity。
5. `Session.Close` 释放 transport 和 bridge 拥有的进程资源。

`client.ForkSession` 会复制源 Session 到传入消息 ID 的精确边界，并创建独立目标。
Claude Code 可能到首个用户回合才持久化目标 transcript，但 bridge 会在返回 Session
前分配目标 Session ID。

Context 取消和用户中断可通过 Go error matching 区分。长时 control 会使用同一个
`request_id` 传播取消，使 runtime 能停止原始操作。

## 安全职责

- 宿主决定信任哪个 runtime 可执行程序和进程环境。
- Bridge 只传递 sandbox policy，不宣称自己执行沙箱隔离。
- 默认进程信号假设宿主与 runtime 使用同一 OS 身份；跨身份运行时，宿主必须通过
  `WithProcessSignalHandler` 提供边界，并在转发生命周期信号前校验 PID 归属。
- 自动生成或内联的 MCP 配置会写入权限受限的参数文件，不直接暴露在进程参数中。
- Provider 凭据保留为 runtime 进程环境；bridge 不解释 Provider 请求 payload。

Runtime 强制隔离与产品授权不属于本库职责。

## 兼容策略

公开 Go package 按仓库版本演进，`internal/` 不属于可依赖 API。新增 runtime 专属行为
必须先定义公开 capability 和类型化协议；宿主不应依赖未文档化 payload 字段。
