# nxs Runtime Path

运行期不再下载或猜测 `nxs` 路径。

宿主必须在启动 bridge 前提供明确路径：

- 打包后的 Nexus 桌面端由 sidecar 将随包 `bin/nxs` 写入 `NEXUS_NXS_COMMAND_PATH`。
- 开发环境直接设置 `NEXUS_NXS_COMMAND_PATH=/path/to/nxs`，或在 SDK options 中使用 `WithCLIPath("/path/to/nxs")`。

`InspectRuntime` / `EnsureRuntime` 只检查 `NEXUS_NXS_COMMAND_PATH` 是否存在且可执行，不访问网络、不扫描 app root、不读取缓存。
