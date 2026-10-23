# zspace-cli

[![CI](https://github.com/vangie/zspace-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/vangie/zspace-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/vangie/zspace-cli)](https://github.com/vangie/zspace-cli/releases)
[![License](https://img.shields.io/github/license/vangie/zspace-cli)](LICENSE)
[![Homebrew](https://img.shields.io/badge/homebrew-vangie%2Ftap-blue)](https://github.com/vangie/homebrew-tap)

极空间 NAS 的命令行和 MCP 服务。桌面客户端已经登录时，本机就能管理文件、存储、相册、影视、音乐、网盘、DDNS 和 Docker。

极空间没有公开 API，也没有官方 CLI。桌面客户端在本机开一个代理（`127.0.0.1:13579`，占用时为 `13580`），把请求转到 NAS。本项目读取客户端已经保存的登录态，不需要密码、SSH，也不需要先配公网地址。

```bash
brew install vangie/tap/zspace-cli
zspace check
zspace ls /sata1/my/data
zspace docker ps
```

## 能做什么

| 能力 | 命令 |
| --- | --- |
| 文件 | `ls` `info` `find` `glob` `tree` `rename` `mkdir` `mv` `cp` `rm` `up` `down` |
| 存储 | `pools` `disks` `usage` `hardware` `smart` |
| 回收站 | `recycle list` `empty` `restore` `config` `config-set` |
| 网盘和下载 | `cloud`（百度、夸克）`download` |
| 相册 | `photo`，含 AI 挑选和内容搜索 |
| 极影视、极音乐 | `video` `music` |
| DDNS | `ddns list` `add` `update` `rm` `refresh` |
| Docker | `docker`，含引擎代理、镜像、容器、Compose |
| Agent | `zspace-mcp`，stdio，默认只读 |

写操作都要 `--yes`。`rm` 把文件移入个人回收站，不立即释放空间。`--json` 输出原始结构。完整参数见 `zspace help`。

## 安装

macOS 和 Linux：

```bash
brew install vangie/tap/zspace-cli
```

这条命令同时安装 `zspace` 和 `zspace-mcp`。升级：

```bash
brew update
brew upgrade vangie/tap/zspace-cli
```

也可以从 [Releases](https://github.com/vangie/zspace-cli/releases) 下载对应系统的压缩包。包里有两个二进制、`LICENSE` 和 `README.md`。校验和在同一个 Release 的 `checksums.txt`。

从源码：

```bash
go install github.com/vangie/zspace-cli/cmd/zspace@latest
go install github.com/vangie/zspace-cli/cmd/zspace-mcp@latest
```

## 前提

极空间桌面客户端已安装、已打开、已登录。登录态位置：

| 系统 | 路径 |
| --- | --- |
| macOS | `~/Library/Application Support/zspace/vuex.json` |
| Windows | `%APPDATA%\zspace\vuex.json` |
| Linux | `~/.zspace`、`~/.config/zspace`、`~/zspace`，按这个顺序查找 |

路径不对时用 `--config-dir` 或 `ZS_CONFIG_DIR`。代理地址用 `--base-url` 或 `ZS_BASE_URL`。

请求绕过系统代理。开着 Surge、Clash 一类工具时不用额外配置。

## 常用示例

```bash
zspace ls /sata1/my/data
zspace find 合同 --limit 20
zspace glob '/sata1/my/data/**/*.mkv'
zspace down /sata1/my/data/notes.md ./notes.md

zspace photo search 海边 --mode ai --limit 10
zspace video search 星际
zspace music search 夜曲

zspace ddns list
zspace docker ps
zspace docker proxy
```

存储池未就绪时，`pools` 可能返回业务码 `N300477`。这时用 `disks` 和 `usage`。

## Docker

NAS 上的 Docker 走极空间自己的 `/zdocker` 接口，不是 Docker Engine API。本机的 `docker` 命令不能直接打到这个代理上。

```bash
zspace docker info
zspace docker proxy-set --http-proxy 192.168.1.10:6152 --yes
zspace docker pull hello-world --yes
zspace docker create hello-world demo --yes \
  --publish 8080:80 \
  --volume /tmp/zfsv3/sata1/<用户>/data/docker_vol/demo:/data \
  --network bridge \
  --restart unless-stopped
zspace docker load /sata1/my/data/image.tar --yes
zspace docker op stop <容器ID> --yes
```

`docker op` 的 action：`start` `stop` `restart` `pause` `resume` `force_stop` `remove` `force_remove`。

`proxy-set` 保存后，极空间会重启 Docker。命令会等到引擎恢复，并核对 `HttpProxy`。挂载路径用容器 inspect 里的宿主机路径，文件管理器里的 `/sata1/my/data/...` 对应 `/tmp/zfsv3/sata1/<用户名>/data/...`。

## DDNS

一条记录是一个服务商加上它的主机名。服务商：`tencent` `aliyun` `cloudflare` `duckdns`。

```bash
zspace ddns add --provider cloudflare --yes \
  --record4 nas.example.com \
  --access-key "$CF_API_TOKEN"
```

Cloudflare 的 `--access-key` 是 API Token，`--access-id` 是可选的 ZONE ID。阿里云和腾讯云的 `--access-id` 必填。`ddns list` 不输出密钥。

## MCP

`zspace-mcp` 是 stdio 服务。默认只暴露读工具。写工具要 `--write`，删除要再加 `--allow-delete`。

```json
{
  "mcpServers": {
    "zspace": {
      "command": "zspace-mcp",
      "args": ["--write"]
    }
  }
}
```

工具名以 `zspace_` 开头，例如 `zspace_ls`、`zspace_glob`、`zspace_docker_ps`、`zspace_video_list`、`zspace_music_songs`。

## 接口来源

极空间的接口没有官方文档。本项目按三处已经公开、并且都是 MIT 许可的社区成果重新实现，没有复制它们的代码：

- [skyzhao1223/zspace-cli](https://github.com/skyzhao1223/zspace-cli)：文件、存储池、磁盘用量、回收站、百度网盘、分片上传。
- [huhuanming/zspace-cli](https://github.com/huhuanming/zspace-cli)：夸克网盘、下载器、相册搜索与 AI 挑选。
- [philipxiaoxi/z-cli](https://github.com/philipxiaoxi/z-cli)：文件接口的另一份独立核对。

Docker 与 Compose 的路径、请求体和 `container_op` 的 action 取值，来自 NAS 网页版按需加载的前端分块。这组接口在上述项目里没有覆盖。

接口随客户端版本变化。命令遇到非 `200` 的业务码时，原样打印 `code` 和 `msg`。

## 安全

- token 只在发请求时使用，不进日志，不进 `--json` 输出。
- 写操作默认拒绝，必须显式 `--yes`。
- MCP 的删除工具默认不注册。
- `ddns list` 不返回 `access_id` 和 `access_key`。

## 开发

```bash
go test ./...
go build -o zspace ./cmd/zspace
go build -o zspace-mcp ./cmd/zspace-mcp
```

推送到 `main` 和 pull request 会跑测试。

发布时在 [CHANGELOG.md](CHANGELOG.md) 写好 `## [X.Y.Z]`，说明新增、修复和破坏性变更。提交后打 tag `vX.Y.Z` 并推送。GitHub Actions 跑测试，从 CHANGELOG 取出这一节作为 Release 说明，再用 GoReleaser 发布二进制，并把 Homebrew formula 写到 [vangie/homebrew-tap](https://github.com/vangie/homebrew-tap)。CHANGELOG 里没有对应版本时，发布会中止。

## 许可

[MIT](LICENSE)。
