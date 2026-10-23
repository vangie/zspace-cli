# Changelog

本文件记录每个已发布版本。GitHub Release 的说明从对应版本的这一节生成，不使用提交标题自动拼出来的列表。

版本号遵循语义化版本。`0.1.x` 仍可能调整命令参数。

## [0.1.3] - 2026-10-22

### 修复

- Cloudflare DDNS 不再要求 `--access-id`。极空间页面上这个字段是可选的 ZONE ID，`--access-key` 才是 API Token。只填 Token 时，`zspace ddns add --provider cloudflare` 和 MCP 的 `zspace_ddns_add` 会被拒绝。阿里云和腾讯云的密钥 ID 仍然必填。

## [0.1.2] - 2026-10-22

### 新增

- `zspace docker proxy` 和 `zspace docker proxy-set`。可以查看并修改 Docker 引擎的 HTTP/HTTPS 代理。保存后等待引擎恢复，并核对 `HttpProxy`。
- `zspace docker create` 支持 `--publish`、`--volume`、`--env`、`--network`、`--restart`。
- `zspace docker load` 从 NAS 上的 tar 导入镜像。

## [0.1.1] - 2026-10-22

### 新增

- 极影视和极音乐的只读命令。
- `zspace ddns` 的查看、新增、更新、删除和刷新。列表不输出密钥。

### 修复

- 参数错误在连接 NAS 之前返回，不再先报登录失败。
- Homebrew 配方同时安装 `zspace` 和 `zspace-mcp`。

## [0.1.0] - 2026-10-22

### 新增

- 首个公开版本。覆盖文件、存储、回收站、百度网盘、夸克网盘、下载器、相册，以及 Docker 和 Compose。
- `zspace-mcp` stdio 服务。默认只注册读工具。

[0.1.3]: https://github.com/vangie/zspace-cli/releases/tag/v0.1.3
[0.1.2]: https://github.com/vangie/zspace-cli/releases/tag/v0.1.2
[0.1.1]: https://github.com/vangie/zspace-cli/releases/tag/v0.1.1
[0.1.0]: https://github.com/vangie/zspace-cli/releases/tag/v0.1.0
