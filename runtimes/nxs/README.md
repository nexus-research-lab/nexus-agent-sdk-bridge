# nxs Runtime Resolver

这里放 bridge 对 `nxs` runtime release manifest 的解析和本地缓存逻辑；不提交 runtime 二进制。

默认下载源是公开 bridge release 下的 `nxs-manifest.json`。可用环境变量：

- `NEXUS_NXS_RUNTIME_RELEASE`：选择 `nxs-vX.Y.Z` runtime release tag。
- `NEXUS_NXS_RUNTIME_MANIFEST_URL`：直接指定 manifest URL。
- `NEXUS_NXS_RUNTIME_CACHE_DIR`：覆盖本地 runtime cache 目录。
- `NEXUS_NXS_RUNTIME_RESOLVER_DISABLED`：跳过下载 resolver，回退到 PATH 中的 `nxs`。

`nxs` 多平台产物由私有 Go SDK 仓库构建，并上传到公开 bridge release。
