# nxs Runtime Resolver

这里放 bridge 对 `nxs` runtime release manifest 的解析和本地缓存逻辑；不提交 runtime 二进制。

默认下载源是公开 bridge release 下的 `nxs-stable/nxs-manifest.json`。这个 stable manifest 是 runtime 索引，resolver 会选择 `min_bridge_version` 兼容当前 bridge module 的最新 runtime。默认 stable 索引不可用时会回退到 bootstrap runtime release，避免首次发布新版 resolver 时出现空窗；单个 `nxs-vX.Y.Z` release 下的 `nxs-manifest.json` 仍然保留，可用于显式固定版本。

可用环境变量：

- `NEXUS_NXS_RUNTIME_RELEASE`：固定 `nxs-vX.Y.Z` runtime release tag。
- `NEXUS_NXS_RUNTIME_MANIFEST_URL`：直接指定单版本 manifest 或 stable manifest URL。
- `NEXUS_NXS_RUNTIME_CACHE_DIR`：覆盖本地 runtime cache 目录。
- `NEXUS_NXS_RUNTIME_RESOLVER_DISABLED`：跳过下载 resolver，回退到 PATH 中的 `nxs`。

`nxs` 多平台产物由私有 Go SDK 仓库构建，并上传到公开 bridge release。发布 workflow 会在单版本 manifest 里写入 `min_bridge_version`，再更新 `nxs-stable` 索引；不涉及 bridge 行为的 runtime 更新不需要发布 bridge 新版本。

宿主应用可以用 `InspectRuntime` 只探测本地 `NEXUS_NXS_COMMAND_PATH`、`NEXUS_APP_ROOT/bin/nxs` 和 cache 状态；用 `EnsureRuntime` 在本地不可用时触发 resolver 下载。`NEXUS_NXS_COMMAND_PATH` 是显式覆盖，优先级高于随包内置 runtime。
