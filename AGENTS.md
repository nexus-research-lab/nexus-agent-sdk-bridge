# AGENTS.md

## 项目定位

本仓库是 Nexus 开源 runtime bridge。它只承载 Go 宿主与 runtime 子进程之间的公开契约、进程生命周期和 stdio `stream-json` 传输，不实现 agent loop，也不 import 闭源 `nexus-agent-sdk-go`。

```
Nexus product
  -> nexus-agent-sdk-bridge
       -> exec nxs 或 Claude Code
```

## 边界约定

- `client/`：Session、能力发现、runtime control 与消息投影
- `protocol/`：stdio control/message wire 真相源
- `internal/transport/`：子进程和传输实现，不向产品泄漏
- `runtimes/`：runtime kind 的公开能力差异
- 新能力必须先定义 capability；产品不能按 runtime 名称猜测 control 是否存在
- AutoDream 只由原生 nxs 提供；宿主负责唤醒，nxs 负责最终执行判断
- 长时 control 的 context 取消必须携带同一 `request_id` 传播到 runtime

## 开发约定

- 用户可见能力、协议或路径变化同步更新 `CHANGELOG.md`、英文 README 与中文 README
- 注释使用中文，解释意图和边界
- Go 代码执行 `gofmt`，实现遵循 Google Go Style
- 全量验证使用 `go test ./...`
- 提交使用 emoji 前缀和英文摘要，例如 `:sparkles: Add native AutoDream control`
