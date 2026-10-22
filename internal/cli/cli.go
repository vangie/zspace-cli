// Package cli 是 zspace 命令的入口。
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/vangie/zspace-cli/internal/api"
)

// version 由 -ldflags 覆盖。
var version = "dev"

// Run 执行一次命令。args 不含程序名。
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, usage)
		return 0
	}
	cmd := args[0]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Fprint(os.Stdout, usage)
		return 0
	}
	if cmd == "version" || cmd == "--version" || cmd == "-v" {
		fmt.Println(version)
		return 0
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "")
	yes := fs.Bool("yes", false, "")
	configDir := fs.String("config-dir", "", "")
	baseURL := fs.String("base-url", "", "")
	hidden := fs.Bool("hidden", false, "")
	public := fs.Bool("public", false, "")
	liked := fs.Bool("liked", false, "")
	unlike := fs.Bool("unlike", false, "")
	force := fs.Bool("force", false, "")
	start := fs.Bool("start", true, "")
	page := fs.Int("page", 1, "")
	limit := fs.Int("limit", 50, "")
	startAt := fs.Int("offset", 0, "")
	pickType := fs.Int("type-id", 0, "")
	mode := fs.String("mode", "ai", "")
	thumbSize := fs.String("size", "small", "")
	status := fs.String("status", "all", "")
	kind := fs.String("type", "loading", "")
	parentID := fs.String("parent-id", "0", "")
	rest, err := parseInterspersed(fs, args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	out, code, err := dispatch(ctx, cmd, rest, opts{
		json: *jsonOut, yes: *yes, configDir: *configDir, baseURL: *baseURL,
		hidden: *hidden, public: *public, force: *force, start: *start,
		liked: *liked, unlike: *unlike, thumbSize: *thumbSize,
		page: *page, limit: *limit, startAt: *startAt, pickType: *pickType,
		mode: *mode, status: *status, kind: *kind, parentID: *parentID,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			return 1
		}
		return code
	}
	if out == nil {
		return 0
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 0
	}
	printHuman(cmd, out)
	return 0
}

type opts struct {
	json, yes, hidden, public, force, start bool
	liked, unlike                           bool
	thumbSize                               string
	configDir, baseURL, mode, status, kind  string
	parentID                                string
	page, limit, startAt, pickType          int
}

func dispatch(ctx context.Context, cmd string, args []string, o opts) (any, int, error) {
	need := func(n int) error {
		if len(args) < n {
			return fmt.Errorf("%s 需要 %d 个参数，运行 zspace help 查看用法", cmd, n)
		}
		return nil
	}
	switch cmd {
	case "check":
		return cmdCheck(ctx, o)
	}

	c, err := connect(o)
	if err != nil {
		return nil, 1, err
	}
	switch cmd {
	case "ls":
		dir := "/sata1/my/data"
		if len(args) > 0 {
			dir = args[0]
		}
		return call(c.List(ctx, dir, o.hidden))
	case "info":
		if err := need(1); err != nil {
			return nil, 2, err
		}
		return call(c.Info(ctx, args[0]))
	case "find":
		if err := need(1); err != nil {
			return nil, 2, err
		}
		entries, err := c.Search(ctx, args[0])
		if err != nil {
			return nil, 1, err
		}
		if len(args) > 1 {
			prefix := args[1]
			var kept []api.FileEntry
			for _, e := range entries {
				if strings.HasPrefix(e.Path, prefix) {
					kept = append(kept, e)
				}
			}
			entries = kept
		}
		if o.limit > 0 && len(entries) > o.limit {
			entries = entries[:o.limit]
		}
		return entries, 0, nil
	case "rename":
		if err := need(2); err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "重命名 "+args[0]); err != nil {
			return nil, 2, err
		}
		return call(c.Rename(ctx, args[0], args[1]))
	case "mkdir":
		if err := need(2); err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "新建目录 "+args[1]); err != nil {
			return nil, 2, err
		}
		return call(c.Mkdir(ctx, args[0], args[1]))
	case "mv":
		if err := need(2); err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "移动到 "+args[len(args)-1]); err != nil {
			return nil, 2, err
		}
		env, err := c.Move(ctx, args[:len(args)-1], args[len(args)-1])
		return env.Data, 0, err
	case "cp":
		if err := need(2); err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "复制到 "+args[len(args)-1]); err != nil {
			return nil, 2, err
		}
		env, err := c.Copy(ctx, args[:len(args)-1], args[len(args)-1])
		return env.Data, 0, err
	case "rm":
		if err := need(1); err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "删除 "+strings.Join(args, " ")+"（进入个人回收站）"); err != nil {
			return nil, 2, err
		}
		env, err := c.Remove(ctx, args)
		return env.Data, 0, err
	case "up":
		if err := need(2); err != nil {
			return nil, 2, err
		}
		if err := confirm(o, "上传 "+args[0]); err != nil {
			return nil, 2, err
		}
		name := ""
		if len(args) > 2 {
			name = args[2]
		}
		return call(c.Upload(ctx, args[0], args[1], name))
	case "down":
		if err := need(1); err != nil {
			return nil, 2, err
		}
		local := "."
		if len(args) > 1 {
			local = args[1]
		}
		if !strings.HasSuffix(local, "/") && !isDir(local) {
			return nil, 0, c.Download(ctx, args[0], local)
		}
		dest := strings.TrimRight(local, "/") + "/" + baseName(args[0])
		return map[string]string{"path": dest}, 0, c.Download(ctx, args[0], dest)
	case "tree":
		dir := "/sata1/my/data"
		if len(args) > 0 {
			dir = args[0]
		}
		depth := o.limit
		if depth <= 0 || depth > 6 {
			depth = 2
		}
		return call(buildTree(ctx, c, dir, depth, o.hidden))
	case "pools":
		return call(c.PoolInfo(ctx))
	case "disks":
		return call(c.DiskStats(ctx))
	case "usage":
		return call(c.DiskUsage(ctx))
	case "hardware":
		return call(c.Hardware(ctx))
	case "smart":
		if err := need(1); err != nil {
			return nil, 2, err
		}
		return call(c.Smart(ctx, args[0]))
	case "recycle":
		return cmdRecycle(ctx, c, args, o)
	case "cloud":
		return cmdCloud(ctx, c, args, o)
	case "download":
		return cmdDownload(ctx, c, args, o)
	case "photo":
		return cmdPhoto(ctx, c, args, o)
	case "docker":
		return cmdDocker(ctx, c, args, o)
	default:
		return nil, 2, fmt.Errorf("未知命令 %s，运行 zspace help 查看用法", cmd)
	}
}

// parseInterspersed 允许选项出现在位置参数之后。
// 标准库的 FlagSet 遇到第一个非选项就停止，zspace ls /path --json 会失效。
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			rest = append(rest, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		f := fs.Lookup(name)
		if f == nil {
			return nil, fmt.Errorf("未知选项 %s", a)
		}
		// 布尔选项不吃下一个参数。
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			if err := fs.Parse([]string{a}); err != nil {
				return nil, err
			}
			continue
		}
		if strings.Contains(a, "=") {
			if err := fs.Parse([]string{a}); err != nil {
				return nil, err
			}
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("选项 %s 缺少值", a)
		}
		if err := fs.Parse([]string{a, args[i+1]}); err != nil {
			return nil, err
		}
		i++
	}
	return rest, nil
}

// call 把两返回值的 API 调用收成 dispatch 需要的三返回值。
func call[T any](v T, err error) (any, int, error) {
	if err != nil {
		return nil, 1, err
	}
	return v, 0, nil
}

func connect(o opts) (*api.Client, error) {
	creds, err := api.LoadCredentials(o.configDir)
	if err != nil {
		return nil, err
	}
	return api.New(creds, o.baseURL)
}

func cmdCheck(ctx context.Context, o opts) (any, int, error) {
	creds, err := api.LoadCredentials(o.configDir)
	result := map[string]any{"logged_in": err == nil}
	if err != nil {
		result["auth_error"] = err.Error()
	}
	c, cerr := api.New(creds, o.baseURL)
	if cerr != nil {
		result["connected"] = false
		result["proxy_error"] = cerr.Error()
		return result, 1, nil
	}
	result["connected"] = true
	result["base_url"] = c.BaseURL
	if pools, perr := c.PoolInfo(ctx); perr == nil {
		result["pools"] = pools
	} else {
		result["pools_error"] = perr.Error()
	}
	return result, 0, nil
}

func confirm(o opts, what string) error {
	if o.yes {
		return nil
	}
	return errors.New(what + "：写操作需要 --yes")
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func baseName(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

type treeNode struct {
	Name  string     `json:"name"`
	Path  string     `json:"path"`
	IsDir bool       `json:"is_dir"`
	Size  int64      `json:"size"`
	Kids  []treeNode `json:"children,omitempty"`
}

func buildTree(ctx context.Context, c *api.Client, dir string, depth int, hidden bool) ([]treeNode, error) {
	entries, err := c.List(ctx, dir, hidden)
	if err != nil {
		return nil, err
	}
	var nodes []treeNode
	for _, e := range entries {
		n := treeNode{Name: e.Name, Path: e.Path, IsDir: e.Dir(), Size: int64(e.Size)}
		if e.Dir() && depth > 1 {
			kids, err := buildTree(ctx, c, e.Path, depth-1, hidden)
			if err == nil {
				n.Kids = kids
			}
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func cmdRecycle(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	switch sub {
	case "list":
		dir := ""
		if len(args) > 0 {
			dir = args[0]
		}
		return call(c.RecycleList(ctx, dir))
	case "empty":
		if err := confirm(o, "清空回收站"); err != nil {
			return nil, 2, err
		}
		return call(c.RecycleEmpty(ctx, o.public))
	case "restore":
		if len(args) == 0 {
			return nil, 2, errors.New("recycle restore 需要回收站内的路径")
		}
		if err := confirm(o, "还原 "+strings.Join(args, " ")); err != nil {
			return nil, 2, err
		}
		var failed []string
		for _, p := range args {
			if err := c.RecycleRestore(ctx, p); err != nil {
				failed = append(failed, p+": "+err.Error())
			}
		}
		return map[string]any{"restored": len(args) - len(failed), "failed": failed}, 0, nil
	case "config":
		return call(c.RecycleConfig(ctx))
	default:
		return nil, 2, fmt.Errorf("未知子命令 recycle %s", sub)
	}
}

func cmdCloud(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	if len(args) < 2 {
		return nil, 2, errors.New("用法：zspace cloud <status|ls|tasks> <baidu|quark>")
	}
	p := api.CloudProvider(args[1])
	switch args[0] {
	case "status":
		return call(c.CloudStatus(ctx, p))
	case "ls":
		dir := "/"
		if len(args) > 2 {
			dir = args[2]
		}
		return call(c.CloudList(ctx, p, dir, o.parentID, o.page, o.limit))
	case "tasks":
		return call(c.CloudTasks(ctx, p))
	default:
		return nil, 2, fmt.Errorf("未知子命令 cloud %s", args[0])
	}
}

func cmdDownload(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	switch sub {
	case "list":
		return call(c.ListDownloads(ctx, o.kind, o.status, 0, o.limit))
	case "add":
		if len(args) < 2 {
			return nil, 2, errors.New("用法：zspace download add <链接> <NAS目录> --yes")
		}
		if err := confirm(o, "添加下载任务"); err != nil {
			return nil, 2, err
		}
		return call(c.AddDownload(ctx, args[0], args[1]))
	default:
		return nil, 2, fmt.Errorf("未知子命令 download %s", sub)
	}
}

func cmdPhoto(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	num := o.limit
	if num <= 0 {
		num = 50
	}
	switch sub {
	case "status":
		return call(c.PhotoStatus(ctx))
	case "home":
		return call(c.AlbumHome(ctx))
	case "albums":
		return call(c.Albums(ctx, o.startAt, num))
	case "info":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace photo info <相册ID>")
		}
		return call(c.AlbumInfo(ctx, args[0]))
	case "feeds":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace photo feeds <相册ID>")
		}
		return call(c.AlbumFeeds(ctx, args[0], o.startAt, num))
	case "timeline":
		return call(c.PhotoTimeline(ctx))
	case "list":
		return call(c.Photos(ctx, o.startAt, num, o.liked))
	case "dirs":
		return call(c.AlbumDirs(ctx))
	case "picks":
		return call(c.PhotoPickTypes(ctx))
	case "pick":
		if o.pickType == 0 && len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &o.pickType)
		}
		if o.pickType == 0 {
			return nil, 2, errors.New("用法：zspace photo pick <分类ID>")
		}
		return call(c.PhotoPicks(ctx, o.pickType, o.startAt, num, ""))
	case "like":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace photo like <照片ID> --yes")
		}
		if err := confirm(o, "修改照片喜欢状态"); err != nil {
			return nil, 2, err
		}
		if err := c.PhotoLike(ctx, args[0], !o.unlike); err != nil {
			return nil, 1, err
		}
		return map[string]any{"feed_id": args[0], "liked": !o.unlike}, 0, nil
	case "search":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace photo search <文字或图片路径> --mode ai|ocr|similar")
		}
		return call(c.PhotoSearch(ctx, o.mode, args[0], num))
	case "thumb":
		if len(args) < 2 {
			return nil, 2, errors.New("用法：zspace photo thumb <NAS路径> <本地文件> --size small|large")
		}
		if err := c.PhotoThumb(ctx, args[0], args[1], o.thumbSize); err != nil {
			return nil, 1, err
		}
		return map[string]string{"path": args[1]}, 0, nil
	case "create":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace photo create <名字> --yes")
		}
		if err := confirm(o, "新建相册 "+args[0]); err != nil {
			return nil, 2, err
		}
		desc := ""
		if len(args) > 1 {
			desc = args[1]
		}
		return call(c.CreateAlbum(ctx, args[0], desc))
	case "delete":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace photo delete <相册ID> --yes")
		}
		if err := confirm(o, "删除相册 "+args[0]); err != nil {
			return nil, 2, err
		}
		if err := c.DeleteAlbum(ctx, args[0]); err != nil {
			return nil, 1, err
		}
		return map[string]any{"album_id": args[0], "deleted": true}, 0, nil
	default:
		return nil, 2, fmt.Errorf("未知子命令 photo %s", sub)
	}
}

func printHuman(cmd string, out any) {
	switch cmd {
	case "ls", "find":
		entries, ok := out.([]api.FileEntry)
		if !ok {
			printJSON(out)
			return
		}
		for _, e := range entries {
			kind := "FILE"
			if e.Dir() {
				kind = "DIR "
			}
			fmt.Printf("%s %12d  %s\n", kind, int64(e.Size), e.Name)
		}
		return
	case "tree":
		nodes, ok := out.([]treeNode)
		if !ok {
			printJSON(out)
			return
		}
		printTree(nodes, 0)
		return
	}
	printJSON(out)
}

func printTree(nodes []treeNode, depth int) {
	for _, n := range nodes {
		mark := "FILE"
		if n.IsDir {
			mark = "DIR "
		}
		fmt.Printf("%s%s %s\n", strings.Repeat("  ", depth), mark, n.Name)
		printTree(n.Kids, depth+1)
	}
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

const usage = `zspace — 极空间 NAS 命令行

用法：zspace <命令> [参数] [选项]

文件
  ls [路径]                    列出目录（--hidden 显示隐藏文件）
  info <路径>                  文件或目录详情
  find <关键词> [路径前缀]     全文搜索（--limit）
  tree [路径]                  目录树（--limit 作深度，默认 2）
  rename <路径> <新名字>       重命名（--yes）
  mkdir <父目录> <名字>        新建目录（--yes）
  mv <路径>... <目标目录>      移动（--yes）
  cp <路径>... <目标目录>      复制（--yes）
  rm <路径>...                 移入个人回收站（--yes）
  up <本地文件> <NAS目录>      上传（--yes）
  down <NAS路径> [本地路径]    下载

存储
  pools                        存储池
  disks                        磁盘统计
  usage                        物理用量快照
  hardware                     硬盘硬件信息
  smart <序列号>               SMART 报告

回收站
  recycle list [路径]          默认个人回收站 /.recycle/my
  recycle empty --yes          清空（--public 清公共回收站）
  recycle restore <路径> --yes 从回收站还原
  recycle config               保留天数

网盘、下载、相册
  cloud status <baidu|quark>
  cloud ls <baidu|quark> [路径]    --page --limit --parent-id
  cloud tasks <baidu|quark>
  download list                    --type loading|complete --status all
  download add <链接> <目录> --yes
  photo status                     相册开关和 AI 索引进度
  photo home                       相册首页分组
  photo albums                     相册列表（--offset --limit）
  photo info <相册ID>
  photo feeds <相册ID>             相册内的照片
  photo timeline                   时间线分组
  photo list                       全部照片（--liked 只看喜欢的）
  photo dirs                       相册扫描目录
  photo picks                      AI 挑选分类
  photo pick <分类ID>              一个分类下的结果
  photo like <照片ID> --yes        标记喜欢（--unlike 取消）
  photo search <文字或路径>        --mode ai|ocr|similar --limit
  photo thumb <NAS路径> <本地文件> 保存缩略图（--size small|large）
  photo create <名字> --yes        新建相册
  photo delete <相册ID> --yes      删除相册

Docker（极空间私有 /zdocker 接口，不是 Docker Engine API）
  docker info
  docker ps
  docker images
  docker networks
  docker projects
  docker pull <镜像>
  docker create <镜像> <名字> --yes
  docker op <action> <容器ID> --yes
  docker rmi <镜像ID> --yes        --force 强制删除
  docker compose-create <名字> <yaml文件> <NAS目录> --yes
  docker compose <action> <项目ID> --yes

其他
  check                        检查客户端代理和登录态
  version

全局选项
  --json           机器可读输出
  --yes            确认写操作
  --config-dir     vuex.json 所在目录（或环境变量 ZS_CONFIG_DIR）
  --base-url       本地代理地址（或环境变量 ZS_BASE_URL）

登录态从极空间桌面客户端的 vuex.json 读取。请先打开并登录客户端。
MCP 服务：zspace-mcp
`
