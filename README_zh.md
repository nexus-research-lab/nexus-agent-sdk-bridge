# Nexus Agent SDK Bridge

[English](./README.md) | 简体中文

这是一个开源 Go client 和协议合同，用于让宿主应用通过 `stream-json` 连接本地
Agent runtime。

```text
宿主应用 -> nexus-agent-sdk-bridge -> runtime 子进程
```

Bridge 负责启动或连接 runtime、传递类型化消息并暴露运行期控制。它不实现
agent loop，也不包含模型 runtime。

宿主在 nxs 协商 `CapabilitySubagentControl` 后，可通过 `Session.Control().ControlSubagent` 在当前父会话 MCP 调用内管理子任务。协议与取消边界见 [runtime contract](docs/runtime-contract.md#subagent-control)；Claude Code 不提供此扩展。


宿主可用 `SandboxSettings.RequireSandbox` 要求 nxs 确认 `required_sandbox_v1` 后才发送任务；不支持的运行时连接失败。这表示必需执行约束，不表示平台后端已可用，详见 [协议](docs/runtime-contract.zh-CN.md#必需沙箱执行)。

同时设置 `RequireFileTools: true` 可单独要求原生 Read/Write/Edit 的 `sandbox_file_tools_v1` 合同；只确认命令沙箱的旧运行时不能收到任务。这不代表所有 SDK IO 已受限，详见 [覆盖范围](docs/runtime-contract.zh-CN.md#原生文件工具隔离)。

再设置 `RequireSearchTools: true` 可要求 `sandbox_search_tools_v1`，确认 Glob/Grep 的路径检查、rg 和结果元数据边界。只具备旧文件能力的运行时不能收到任务，详见 [搜索合同](docs/runtime-contract.zh-CN.md#搜索工具隔离)。

设置 `RequireMediaNetwork: true` 独立确认远程图片的逐请求网络检查、重定向和受控物化，依赖命令、文件及本地媒体合同。详见 [媒体网络合同](docs/runtime-contract.zh-CN.md#远程图片网络)。

同时要求命令和文件合同后，设置 `RequireMediaFiles: true` 可独立确认 ViewImage 与模型预处理的本地图片读取。旧版本在任务写入前拒绝；远程媒体网络独立验收，详见 [媒体文件合同](docs/runtime-contract.zh-CN.md#本地媒体文件隔离)。

`SandboxSettings.Resources` 另要求 `sandbox_resources_v1`，选择工作区写入范围及宿主已准备的私有 scratch；当前 macOS 支持范围与生命周期责任见 [资源合同](docs/runtime-contract.zh-CN.md#宿主资源写入范围)。

Claude Code 需要使用自身受限模式时，设置 `SandboxSettings.RequireClaudeRestricted: true`。Bridge 只在 `RuntimeClaude` 下加入并检查类型化的 `--restricted` 启动合同；Full Access 不要求该合同。这不是 nxs 的 `required_sandbox_v1`，也不证明 Claude SDK 的完整 IO 已隔离，详见 [Claude 原生受限启动](docs/runtime-contract.zh-CN.md#claude-原生受限启动)。

同时要求命令和文件合同后，设置 `RequireContextFiles: true` 可确认启动指令与 compact 文件恢复的边界，详见 [上下文文件合同](docs/runtime-contract.zh-CN.md#上下文文件隔离)。

同时要求命令和文件合同后，设置 `RequireSkillFiles: true` 可独立确认 Skill 目录、正文、动态发现、Git 忽略与 remember 设置读取。启动配置、hook 和后台 IO 继续独立验收，详见 [Skill 文件合同](docs/runtime-contract.zh-CN.md#skill-文件隔离)。

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
`OutboundMessageOptions.ToolAccess = "none"`、`MaxOutputTokens` 和
`SkipAutoMemory` 收窄单次回合；未支持该能力的 runtime 必须拒绝，不能假定已经安全收窄。
`Session.Control().SetNextTurnContext` 接受内部上下文块。bridge 确定性排序后，把它们
绑定到下一条 user 消息。NXS 把提醒保留在当前模型历史中但不写入 transcript；Claude
Code 则通过原生 `UserPromptSubmit` hook 的 `additionalContext` 生成 attachment。
`OutboundMessageOptions.MessageUUID` 允许宿主指定 transcript 消息身份，便于在
提交前通过 `Session.Control().RemoveMessages` 删除未准入回合及其输出。
使用 `client.ForkSession(ctx, sourceSessionID, completedMessageID, options)`
可从精确的已完成消息边界创建独立 Session；`nxs` 与 Claude Code 都声明
`client.CapabilitySessionFork`。
宿主若以另一个 OS 身份运行子进程，可通过 `WithProcessSignalHandler` 提供可信且
校验 PID 的进程信号边界，统一处理中断、关闭和遗留子进程清理。
`client.ProcessCleanupError` 在 Wait、主动终止和重复 Close 中保留清理失败。
内置 Unix 清理只覆盖原 session 的可见成员；另建 session 的后代与跨重启资源恢复
仍需宿主监督。Windows runtime 与 CLI probe 挂起创建，绑定关闭即终止的 Job 后才
恢复执行。原生测试覆盖立即派生的后代、继承管道时取消及准入后宿主被终止；Job
绑定前宿主崩溃仍可能留下挂起进程，创建/绑定原子性与跨重启资源恢复仍需平台证据，详见
[生命周期边界](docs/runtime-contract.zh-CN.md#session-生命周期)。

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

自动审核模式 `permission_mode=auto` 在 nxs 上协商 `auto_review_v1`，在 Claude Code 上确认原生模式。Claude 自行负责分类审核与拒绝处理；不支持或未确认启用时返回错误，详见 [运行时契约](docs/runtime-contract.md)。

宿主可调用 `nxs.NewRuntimeInspector().SandboxStatus(ctx)` 查询配置运行时的版本化原生后端诊断。查询有时间和输出上限，独立于文件可用性，不代表隔离已经生效，详见 [运行时探测](runtimes/nxs/README.md)。

宿主可在命令/文件要求之外设置 `RequireManagedPolicy: true`，独立确认托管来源固定与执行前完整性。详见 [托管策略合同](docs/runtime-contract.md#managed-policy-integrity)。

普通配置受限读取与完整快照由 `RequireSettingsFiles` 独立要求，依赖命令/文件合同。见 [普通配置合同](docs/runtime-contract.zh-CN.md#普通配置文件与快照)。该读取合同不包含配置持久化。

在必需沙箱、文件工具和配置读取要求之外设置 `RequireSettingsWrites: true`，可要求 nxs 独立确认 Config 与权限持久化边界。Claude Code 和缺少该能力的旧 nxs 会在任务发送前拒绝。见 [普通配置写入合同](docs/runtime-contract.zh-CN.md#普通配置写入)。

SDK 托管工具把 `params._meta["claudecode/toolUseId"]` 传递为 `tools.Context.ToolUseID`。缺省元数据保持为空，不从业务参数或 JSON-RPC request ID 推断 tool-use 身份。

### 远端 MCP 网络

`RequireMCPNetwork` / `sandbox_mcp_network_v1` 要求必需沙箱、`MCP.StrictConfig` 显式服务来源和 nxs 能力确认。HTTP 与旧式 SSE 服务仅获得自身协议、主机、端口的端点授权；跳转和 SSE POST 地址不能跨 origin，普通工具联网权限保持独立，显式禁止及托管域名规则仍优先。撤销、更换配置、关闭及权限变化取消对应请求。认证 helper、stdio 进程、OAuth 发现和模型 Provider 网络仍是独立合同。

`RequireMCPHelpers` / `sandbox_mcp_helpers_v1` 独立要求 macOS 认证 helper 的受限命令执行，依赖必需沙箱与显式 MCP 配置。逐请求刷新认证，复用命令资源策略和任务环境过滤，不借用 MCP 端点授权；限时限量、权限变化取消、关闭等待清理，失败不回退静态或过期凭据。stdio 和脱离 session 后代监督仍是独立合同。

`RequireMCPStdio` / `sandbox_mcp_stdio_v1` 独立确认 macOS 显式 stdio MCP 的受限命令执行，依赖必需沙箱与 strict MCP 配置。取消会撤销整个服务及其在途请求，同名替换等待旧进程清理，会话关闭等待全部自有进程；不重放请求。先过滤继承的模型 Provider 凭据，再添加显式服务凭据；服务环境不能改写保留的 runtime、home 和临时根输入。网络沿用命令策略，不借用端点或工具批准。JSONL 消息限制为 10 MiB，在途请求最多 64 个，stderr 持续丢弃而不暴露凭据。当前覆盖普通进程组，独立脱离的后代及宿主崩溃恢复仍须单独验收。

内部 `internal/processscope` 组件已提供 macOS coalition 登记、audit-token 精确终止及内核回收观察。控制连接登记核对内核 peer audit identity 和可信 launcher 指定的进程；job 与可执行文件认证仍由调用方负责。原生测试覆盖登记后放行、脱离后代、恢复登记和独立进程不受影响；组件尚未接入默认启动、关闭或 scratch 回收，不改变既有生命周期保证。缺少原生接口时返回 unavailable，macOS 14.0 兼容路线仍待收口。

`cmd/nexus-runtime-bootstrap` 提供可从源码构建的 macOS 独立引导入口：固定宿主身份认证后接收限长启动输入和三个标准管道，使用显式环境原地 exec。任务参数与凭据不写入 launchd plist。该 helper 尚未打入产品或默认启用；宿主 launcher、持久放行及 transport 接入仍未完成。

内部 `WatchRoot` 在放行前核对连接身份并注册 kqueue 退出事件，能跨原地 exec 保留主进程退出码/信号；取消等待不丢失共享观察，主动停止观察不算退出。主进程退出与集合回收继续独立，仍未接入默认 transport。

显式 `supervision` 启动器已串联上述组件：宿主 `Host` 回调负责持久意图、受保护 job 发布、原集合登记、一次性放行和回收事实。根退出会主动清理后代释放继承管道；Close 共享结果，取消等待不撤销清理，失败不会在重复调用时消失。此入口尚未替代默认 client transport，Nexus 数据库适配、重启恢复和 App 打包仍待接入。
