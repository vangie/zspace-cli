# zspace-cli

极空间 NAS 的命令行和 MCP 服务。一个二进制覆盖文件、存储、回收站、百度网盘、夸克网盘、下载器、相册，以及 Docker 和 Compose。

极空间没有公开 API，也没有官方 CLI。桌面客户端在本机开一个代理（`127.0.0.1:13579`，占用时为 `13580`），把请求转到 NAS。本项目读客户端已经登录的状态，不需要密码、SSH 或 DDNS。

```bash
zspace check
zspace ls /sata1/my/data
zspace docker ps
```

## 安装

发布后用 Homebrew：

```bash
brew install vangie/tap/zspace-cli
```

从源码：

```bash
go install github.com/vangie/zspace-cli/cmd/zspace@latest
go install github.com/vangie/zspace-cli/cmd/zspace-mcp@latest
```

或者克隆后 `go build -o zspace ./cmd/zspace`。

## 前提

极空间桌面客户端已安装、已打开、已登录。macOS 上登录态在
`~/Library/Application Support/zspace/vuex.json`，Windows 在 `%APPDATA%\zspace\vuex.json`。
Linux 依次查找 `~/.zspace`、`~/.config/zspace`、`~/zspace`。

位置不对时用 `--config-dir` 或环境变量 `ZS_CONFIG_DIR`。代理地址用 `--base-url` 或 `ZS_BASE_URL`。

请求绕过系统代理。开着 Surge、Clash 一类工具时不用额外配置。

## 命令

写操作都要 `--yes`。删除进入个人回收站，不立即释放空间。`--json` 输出原始结构。

| 命令 | 作用 |
| --- | --- |
| `ls` `info` `find` `glob` `tree` | 列目录、详情、全文搜索、通配符、目录树 |
| `rename` `mkdir` `mv` `cp` `rm` | 重命名、新建、移动、复制、删除 |
| `up` `down` | 上传、下载。大文件按桌面客户端的分片协议上传 |
| `pools` `disks` `usage` `hardware` `smart` | 存储池、磁盘、用量、硬件、SMART。存储池未就绪时 `pools` 返回业务码 `N300477`，此时用 `disks` 和 `usage` |
| `recycle list\|empty\|restore\|config\|config-set` | 回收站。`config-set --my-days` 改保留天数，`-1` 为永不清除 |
| `cloud status\|info\|ls\|tasks\|fails` | 百度网盘、夸克网盘的状态、账号、目录、任务、失败明细 |
| `cloud down\|task\|share-verify\|share-ls` | 下载到 NAS、暂停/继续/清理任务、百度分享链接 |
| `download list\|add` | NAS 下载器 |
| `photo status\|home\|albums\|info\|feeds\|timeline\|list\|dirs` | 相册首页、列表、详情、照片流、时间线、扫描目录 |
| `photo picks\|pick` | AI 挑选分类和结果 |
| `photo search` | 内容搜索（`--mode ai\|ocr\|similar`） |
| `photo like\|thumb\|create\|delete` | 喜欢、缩略图、新建和删除相册。写操作要 `--yes` |
| `video status\|classes\|home\|list\|info\|search\|records\|favorites\|series` | 极影视片库、搜索、播放记录、收藏 |
| `music status\|songs\|albums\|artists\|playlists\|latest\|history\|favorites\|picks\|search\|lyrics` | 极音乐曲库、歌单、最近播放、歌词 |
| `docker info\|ps\|images\|networks\|projects` | Docker 只读 |
| `docker pull\|create\|op\|rmi` | 拉镜像、建容器、启停、删镜像 |
| `docker compose-create` `docker compose` | Compose 项目创建与 start/stop/restart/delete/rebuild |

`zspace help` 有每个命令的参数。

### Docker

NAS 上的 Docker 走极空间自己的 `/zdocker` 接口，不是 Docker Engine API。官方 `docker` 命令不能直接打到这个代理上。容器创建的请求体是 Engine `ContainerCreate` 的一个子集，字段与极空间网页版一致。

```bash
zspace docker ps
zspace docker pull hello-world --yes
zspace docker create hello-world demo --yes
zspace docker op start <容器ID> --yes
zspace docker op remove <容器ID> --yes
```

`docker op` 的 action：`start` `stop` `restart` `pause` `resume` `force_stop` `remove` `force_remove`。

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

工具名以 `zspace_` 开头，例如 `zspace_ls`、`zspace_glob`、`zspace_docker_ps`、`zspace_video_list`、`zspace_music_songs`。`--write` 另外打开回收站天数、网盘任务和网盘下载。

## 接口从哪里来

极空间的接口没有官方文档。本项目按三处已经公开、并且都是 MIT 许可的社区成果重新实现，没有复制它们的代码：

- [skyzhao1223/zspace-cli](https://github.com/skyzhao1223/zspace-cli)：文件、存储池、磁盘用量、回收站、百度网盘、分片上传。
- [huhuanming/zspace-cli](https://github.com/huhuanming/zspace-cli)：夸克网盘、下载器、相册搜索与 AI 挑选。
- [philipxiaoxi/z-cli](https://github.com/philipxiaoxi/z-cli)：文件接口的另一份独立核对。

Docker 与 Compose 的路径、请求体和 `container_op` 的 action 取值，来自 NAS 网页版按需加载的前端分块（`home/static/js/async/1682`、`77720`、`25400`、`46504`、`87030`）。这组接口在上述项目里没有覆盖。

接口随客户端版本变化。命令遇到非 `200` 的业务码时，原样打印 `code` 和 `msg`。

## 安全

- token 只在发请求时使用，不进日志，不进 `--json` 输出。
- 写操作默认拒绝，必须显式 `--yes`。
- MCP 的删除工具默认不注册。

## 开发

```bash
go test ./...
go build -o zspace ./cmd/zspace
go build -o zspace-mcp ./cmd/zspace-mcp
```

打 tag `v*` 后，GitHub Actions 用 goreleaser 发布二进制，并把 Homebrew formula 写到 [vangie/homebrew-tap](https://github.com/vangie/homebrew-tap)。

## 许可

MIT。
