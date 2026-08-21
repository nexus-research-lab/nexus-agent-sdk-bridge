# Nexus Agent SDK Bridge

[English](./README.md) | 简体中文

这是一个开源 Go client 和协议合同，用于让宿主应用通过 `stream-json` 连接本地
Agent runtime。

```text
宿主应用 -> nexus-agent-sdk-bridge -> runtime 子进程
```

Bridge 负责启动或连接 runtime、传递类型化消息并暴露运行期控制。它不实现
agent loop，也不包含模型 runtime。

## 前置条件

- Go 1.24 及以上版本
- 任选一个 runtime
  - 单独安装 Claude Code
  - 使用官方 Nexus 发布包或其他已授权来源提供的 `nxs` 可执行程序

原生 `nxs` runtime 是闭源程序，本仓库不包含、下载或构建它。

## 安装

```bash
go get github.com/nexus-research-lab/nexus-agent-sdk-bridge@latest
```

## 选择 Runtime

| Runtime | 配置方式 |
| --- | --- |
| Claude Code | `client.NewOptions().WithRuntime(client.RuntimeClaude)` |
| 原生 `nxs` | `NEXUS_NXS_COMMAND_PATH=/path/to/nxs` 或 `WithCLIPath(...)` |
| Direct connect | `WithDirectConnect(...)` |
| 宿主管理 transport | `WithTransport(...)` |

`nxs` 是默认 runtime kind，因此独立程序必须提供其命令路径。Claude Code 始终是
显式选择的兼容 runtime。

## 使用 Claude Code 快速开始

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
		Prompt:  "用一句话概括这个项目。",
		Options: options,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Result)
}
```

Claude Code 发现会优先使用原生可执行文件，并仅在安全时使用平台包装脚本。可通过
`NEXUS_CLAUDE_COMMAND_PATH` 或 `WithCLIPath` 跳过自动发现。

## 持久 Session

```go
session, err := client.NewSession(ctx, options)
if err != nil {
	return err
}
defer session.Close(ctx)

stream, err := session.Send(ctx, "给出一份简洁的实现计划。")
if err != nil {
	return err
}

result, err := stream.Result(ctx)
if err != nil {
	return err
}
fmt.Println(result.Result)
```

增量消息通过 `stream.Recv` 消费。宿主暴露可选控制前，应调用
`session.Supports(capability)`，不要按 runtime 名称猜测能力。
协商 `client.CapabilityMessageExecutionPolicy` 后，宿主可以用
`OutboundMessageOptions.ToolAccess = "none"` 和 `MaxOutputTokens` 签发单次
message-only 回合；未支持该能力的 runtime 必须拒绝，不能假定已经安全收窄。
`OutboundMessageOptions.MessageUUID` 允许宿主指定 transcript 消息身份，便于在
提交前通过 `Session.Control().RemoveMessages` 删除未准入回合及其输出。
使用 `client.ForkSession(ctx, sourceSessionID, completedMessageID, options)`
可从精确的已完成消息边界创建独立 Session；`nxs` 与 Claude Code 都声明
`client.CapabilitySessionFork`。
宿主若以另一个 OS 身份运行子进程，可通过 `WithProcessSignalHandler` 提供可信且
校验 PID 的进程信号边界，统一处理中断、关闭和遗留子进程清理。

## 文档

- [文档索引](./docs/README.md)
- [Runtime 契约](./docs/runtime-contract.zh-CN.md)
- [Go package reference](https://pkg.go.dev/github.com/nexus-research-lab/nexus-agent-sdk-bridge)
- [变更记录](./CHANGELOG.md)

## 公开 Package

| Package | 职责 |
| --- | --- |
| `client` | Query、Session、Options、transport 选择、capability 与 runtime control |
| `protocol` | 流式消息、content block、lifecycle event 与 control wire 类型 |
| `agent` | Subagent 配置类型的唯一公开来源 |
| `hook` | Runtime Hook 事件、匹配器与回调 |
| `permission` | 权限模式、请求与决策 |
| `mcp` | MCP 配置与状态类型 |
| `tools` | Go 原生 MCP tool 与结果辅助函数 |
| `runtimes/nxs` | 只检查原生 runtime 路径，不下载可执行程序 |

`internal/` 下的 package 只属于实现细节，不是受支持的导入路径。

## 开发

```bash
make test
```

## 许可证

Apache License 2.0 · [LICENSE](./LICENSE)
