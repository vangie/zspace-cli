// Package api 调用极空间 NAS 的私有 HTTP 接口。
//
// 请求经桌面客户端的本地代理（默认 127.0.0.1:13579）转到 NAS。
// 文件、存储、网盘、相册、下载器使用表单 POST，业务码为字符串 "200"。
// Docker 管理使用 /zdocker 前缀，GET 或 JSON POST，业务码是 200 或 "200"。
//
// 路径与参数来自三处已公开的社区实现（均为 MIT，这里按协议重新实现）：
//   - github.com/skyzhao1223/zspace-cli   文件、存储、回收站、百度网盘、分片上传
//   - github.com/huhuanming/zspace-cli    夸克网盘、下载器、相册
//   - 极空间网页版 Docker 分块 async/1682  容器、镜像、Compose
package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultSliceSize 是分片上传的单片大小，与桌面客户端一致。
	DefaultSliceSize = 2 * 1024 * 1024
	// DefaultSliceThreshold 超过这个大小走分片上传。
	DefaultSliceThreshold = 8 * 1024 * 1024
	// apiVersion 是桌面客户端网页层上报的接口版本。存储池等接口会校验它，
	// 过旧的值返回 N300477。与社区客户端保持同一串。
	apiVersion = "2.3.2026042401"
)

// Error 是极空间返回的业务错误。Code 原样保留，调用方据此区分鉴权失败和参数错误。
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string {
	if e.Msg == "" {
		return "zspace: code " + e.Code
	}
	return "zspace: " + e.Code + " " + e.Msg
}

// Envelope 是所有接口共用的外层包装。
type Envelope struct {
	Code json.RawMessage `json:"code"`
	Msg  string          `json:"msg"`
	TS   int64           `json:"ts"`
	Data json.RawMessage `json:"data"`
	Raw  json.RawMessage `json:"-"`
}

// CodeString 把 code 统一成字符串。文件接口是 "200"，Docker 接口有时是数字 200。
func (e Envelope) CodeString() string {
	s := strings.TrimSpace(string(e.Code))
	s = strings.Trim(s, `"`)
	return s
}

// OK 判断业务是否成功。
func (e Envelope) OK() bool { return e.CodeString() == "200" }

// DecodeData 把 data 解到 out。data 为 null 时 out 保持零值。
func (e Envelope) DecodeData(out any) error {
	if len(e.Data) == 0 || string(e.Data) == "null" {
		return nil
	}
	return json.Unmarshal(e.Data, out)
}

// Client 持有登录态和本地代理地址。并发安全，可被多个命令复用。
type Client struct {
	BaseURL   string
	Creds     Credentials
	HTTP      *http.Client
	SliceSize int
}

// New 用登录态构造客户端。base 为空时按 ZS_BASE_URL、13579、13580 的顺序探测。
func New(creds Credentials, base string) (*Client, error) {
	c := &Client{
		Creds:     creds,
		SliceSize: DefaultSliceSize,
		HTTP: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				// 不读环境代理。用户在用 Surge 一类工具时，系统代理会把
				// 127.0.0.1:13579 转走，返回空的 502。
				Proxy: nil,
				DialContext: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).DialContext,
			},
		},
	}
	if base == "" {
		base = os.Getenv("ZS_BASE_URL")
	}
	if base != "" {
		c.BaseURL = strings.TrimRight(base, "/")
		return c, nil
	}
	found, err := c.detectBase()
	if err != nil {
		return nil, err
	}
	c.BaseURL = found
	return c, nil
}

func (c *Client) detectBase() (string, error) {
	for _, port := range []int{13579, 13580} {
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/home/", nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := c.HTTP.Do(req)
		cancel()
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 500 {
			return base, nil
		}
	}
	return "", fmt.Errorf("本地代理 13579/13580 没有响应，请确认极空间客户端已打开并登录")
}

func (c *Client) cookieHeader() string {
	return strings.Join([]string{
		"token=" + c.Creds.Token,
		"zenithtoken=" + c.Creds.Token,
		"nas_id=" + c.Creds.NasID,
		"nasid=" + c.Creds.NasID,
		"device_id=" + c.Creds.DeviceID,
	}, "; ")
}

func rnd() string {
	return fmt.Sprintf("%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%100000)
}

// Form 发送表单 POST。extra 为业务参数，公共鉴权参数由客户端补上。
// paths 非空时按 paths[]= 重复键编码，用于移动、复制、删除。
func (c *Client) Form(ctx context.Context, endpoint string, extra map[string]string, paths []string) (Envelope, error) {
	vals := url.Values{}
	vals.Set("token", c.Creds.Token)
	vals.Set("nasid", c.Creds.NasID)
	vals.Set("plat", "web")
	vals.Set("version", apiVersion)
	vals.Set("device_id", c.Creds.DeviceID)
	vals.Set("_l", "zh_cn")
	for k, v := range extra {
		vals.Set(k, v)
	}
	body := vals.Encode()
	for _, p := range paths {
		body += "&paths%5B%5D=" + url.QueryEscape(p)
	}
	u := c.BaseURL + endpoint + "?&rnd=" + rnd() + "&webagent=v2"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(body))
	if err != nil {
		return Envelope{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", c.cookieHeader())
	return c.do(req)
}

// Get 发送 GET。query 会与登录字段、rnd、webagent 合并。
func (c *Client) Get(ctx context.Context, endpoint string, query map[string]string) (Envelope, error) {
	vals := url.Values{}
	vals.Set("rnd", rnd())
	vals.Set("webagent", "v2")
	vals.Set("token", c.Creds.Token)
	vals.Set("nasid", c.Creds.NasID)
	vals.Set("plat", "web")
	vals.Set("version", apiVersion)
	vals.Set("device_id", c.Creds.DeviceID)
	vals.Set("_l", "zh_cn")
	for k, v := range query {
		if v != "" {
			vals.Set(k, v)
		}
	}
	u := c.BaseURL + endpoint + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Envelope{}, err
	}
	req.Header.Set("Cookie", c.cookieHeader())
	req.Header.Set("Accept", "application/json")
	return c.do(req)
}

// PostJSON 发送 JSON POST。body 为 nil 时发送 {}。
func (c *Client) PostJSON(ctx context.Context, endpoint string, query map[string]string, body any) (Envelope, error) {
	if body == nil {
		body = map[string]any{}
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Envelope{}, err
	}
	vals := url.Values{}
	vals.Set("rnd", rnd())
	vals.Set("webagent", "v2")
	for k, v := range query {
		if v != "" {
			vals.Set(k, v)
		}
	}
	u := c.BaseURL + endpoint + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return Envelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", c.cookieHeader())
	return c.do(req)
}

func (c *Client) do(req *http.Request) (Envelope, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Envelope{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Envelope{}, err
	}
	if resp.StatusCode == http.StatusForbidden {
		return Envelope{}, fmt.Errorf("%s 被拒绝（HTTP 403），登录态可能失效", req.URL.Path)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, fmt.Errorf("%s 返回非 JSON（HTTP %d）: %s", req.URL.Path, resp.StatusCode, truncate(raw, 200))
	}
	env.Raw = raw
	if !env.OK() {
		return env, &Error{Code: env.CodeString(), Msg: env.Msg}
	}
	return env, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// FlexInt 兼容接口把同一个字段返回成数字或字符串的情况。
// /v2/file/list 的 is_dir、size 在不同版本里两种都出现过。
type FlexInt int64

// MarshalJSON 输出数字，保证 --json 可被其他程序解析。
func (n FlexInt) MarshalJSON() ([]byte, error) {
	return strconv.AppendInt(nil, int64(n), 10), nil
}

// UnmarshalJSON 接受 1、"1" 和空值。
func (n *FlexInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || string(b) == `""` {
		return nil
	}
	var i int64
	if err := json.Unmarshal(b, &i); err == nil {
		*n = FlexInt(i)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*n = FlexInt(i)
	return nil
}

// FileEntry 是 /v2/file/list 的一条记录，只保留命令和 MCP 用得到的字段。
type FileEntry struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	IsDir   FlexInt `json:"is_dir"`
	Size    FlexInt `json:"size"`
	ModTime FlexInt `json:"modify_time"`
}

// Dir 判断是否目录。接口把 is_dir 返回成 0/1，数字或字符串。
func (f FileEntry) Dir() bool { return f.IsDir == 1 }

// List 列出目录，按页取完。接口单页最多 50 条。
func (c *Client) List(ctx context.Context, dir string, hidden bool) ([]FileEntry, error) {
	show := "0"
	if hidden {
		show = "1"
	}
	var all []FileEntry
	for start := 0; ; {
		env, err := c.Form(ctx, "/v2/file/list", map[string]string{
			"path": dir, "show_hidden": show,
			"start": strconv.Itoa(start), "limit": "50",
		}, nil)
		if err != nil {
			return nil, err
		}
		var data struct {
			List  []FileEntry `json:"list"`
			Total FlexInt     `json:"total"`
		}
		if err := env.DecodeData(&data); err != nil {
			return nil, err
		}
		all = append(all, data.List...)
		start += len(data.List)
		if len(data.List) < 50 || (data.Total > 0 && start >= int(data.Total)) {
			return all, nil
		}
	}
}

// Info 取单个路径的详细信息，原样返回 data。
func (c *Client) Info(ctx context.Context, p string) (map[string]any, error) {
	env, err := c.Form(ctx, "/v2/file/info", map[string]string{"path": p}, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// Rename 重命名，newName 只是新文件名，不含目录。
func (c *Client) Rename(ctx context.Context, p, newName string) (map[string]any, error) {
	env, err := c.Form(ctx, "/v2/file/modify", map[string]string{"path": p, "newname": newName}, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// Mkdir 在 parent 下新建目录。
func (c *Client) Mkdir(ctx context.Context, parent, name string) (map[string]any, error) {
	env, err := c.Form(ctx, "/v2/file/newdir", map[string]string{
		"parent": parent, "name": name, "rename": "0",
	}, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// Move 把若干路径移动到目标目录。
func (c *Client) Move(ctx context.Context, paths []string, to string) (Envelope, error) {
	return c.Form(ctx, "/v2/file/move", map[string]string{"to": to}, paths)
}

// Copy 把若干路径复制到目标目录。
func (c *Client) Copy(ctx context.Context, paths []string, to string) (Envelope, error) {
	return c.Form(ctx, "/v2/file/copy", map[string]string{"to": to}, paths)
}

// Remove 把路径移入个人回收站，不释放空间。
func (c *Client) Remove(ctx context.Context, paths []string) (Envelope, error) {
	return c.Form(ctx, "/v2/file/remove", nil, paths)
}

// Search 走全文索引。接口本身不按目录过滤，目录过滤由调用方处理。
func (c *Client) Search(ctx context.Context, keyword string) ([]FileEntry, error) {
	env, err := c.Form(ctx, "/file_search/file_search", map[string]string{"keyword": keyword}, nil)
	if err != nil {
		return nil, err
	}
	var data struct {
		List []FileEntry `json:"list"`
	}
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data.List, nil
}

// PoolInfo 返回存储池信息。
func (c *Client) PoolInfo(ctx context.Context) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/zspool/info", nil, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// DiskStats 返回磁盘统计。
func (c *Client) DiskStats(ctx context.Context) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/disk/statics", nil, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// DiskUsage 返回 /system/diskusage3 的物理用量快照。
func (c *Client) DiskUsage(ctx context.Context) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/system/diskusage3", nil, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// Hardware 返回硬盘硬件信息。
func (c *Client) Hardware(ctx context.Context) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/zspool/hardware/info", nil, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// Smart 读取指定序列号的 SMART 报告。
func (c *Client) Smart(ctx context.Context, sn string) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/zspool/smart/report2", map[string]string{"sn": sn}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// RecycleList 列出回收站。path 为空时用个人回收站虚拟根 /.recycle/my。
func (c *Client) RecycleList(ctx context.Context, dir string) (json.RawMessage, error) {
	if dir == "" {
		dir = "/.recycle/my"
	}
	env, err := c.Form(ctx, "/v2/file/nb/list", map[string]string{
		"path": dir, "start": "0", "num": "200",
	}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// RecycleEmpty 清空回收站。public 为真时清公共回收站，否则清个人回收站。
func (c *Client) RecycleEmpty(ctx context.Context, public bool) (json.RawMessage, error) {
	endpoint := "/v2/file/rclean"
	if public {
		endpoint = "/v2/public/recycle/clean"
	}
	env, err := c.Form(ctx, endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// RecycleRestore 从个人回收站还原一个路径。路径必须是回收站内的路径。
func (c *Client) RecycleRestore(ctx context.Context, p string) error {
	_, err := c.Form(ctx, "/v2/file/restore", map[string]string{"path": p}, nil)
	return err
}

// RecycleConfig 读取回收站保留天数。-1 表示永不自动清除。
func (c *Client) RecycleConfig(ctx context.Context) (map[string]any, error) {
	env, err := c.Form(ctx, "/v2/file/recycle/config/get", nil, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// CloudProvider 是已接通的网盘。百度与夸克的路径前缀不同，参数也不同。
type CloudProvider string

const (
	CloudBaidu CloudProvider = "baidu"
	CloudQuark CloudProvider = "quark"
)

func (p CloudProvider) base() (string, error) {
	switch p {
	case CloudBaidu:
		return "/znetdisk", nil
	case CloudQuark:
		return "/zdrive/kuake", nil
	default:
		return "", fmt.Errorf("网盘只支持 baidu 或 quark")
	}
}

// CloudStatus 检查网盘是否已登录。
func (c *Client) CloudStatus(ctx context.Context, p CloudProvider) (map[string]any, error) {
	base, err := p.base()
	if err != nil {
		return nil, err
	}
	env, err := c.Form(ctx, base+"/auth/check", map[string]string{"rd": c.BaseURL + "/home/newNetdisk"}, nil)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// CloudList 列出网盘目录。夸克用 parent_file_id，百度用 path 和页码。
func (c *Client) CloudList(ctx context.Context, p CloudProvider, dir, parentID string, page, limit int) (json.RawMessage, error) {
	base, err := p.base()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	if parentID == "" {
		parentID = "0"
	}
	var endpoint string
	extra := map[string]string{}
	switch p {
	case CloudBaidu:
		endpoint = base + "/file/list"
		extra["path"] = dir
		extra["page"] = strconv.Itoa(page)
		extra["limit"] = strconv.Itoa(limit)
	default:
		endpoint = base + "/file/filelist"
		extra["parent_file_id"] = parentID
		extra["show_dir"] = "1"
		extra["limit"] = strconv.Itoa(limit)
		extra["sort"] = ""
	}
	env, err := c.Form(ctx, endpoint, extra, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CloudTasks 列出网盘传输任务。
func (c *Client) CloudTasks(ctx context.Context, p CloudProvider) (json.RawMessage, error) {
	base, err := p.base()
	if err != nil {
		return nil, err
	}
	env, err := c.Form(ctx, base+"/task/list", map[string]string{"page": "1", "limit": "50", "state": ""}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// AddDownload 向 NAS 下载器添加一条链接。
func (c *Client) AddDownload(ctx context.Context, uri, dir string) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/downloader/add/link", map[string]string{"uri": uri, "dir": dir}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// ListDownloads 列出下载任务。kind 取 loading 或 complete。
func (c *Client) ListDownloads(ctx context.Context, kind, status string, start, size int) (json.RawMessage, error) {
	if kind == "" {
		kind = "loading"
	}
	if status == "" {
		status = "all"
	}
	if size <= 0 {
		size = 50
	}
	env, err := c.Form(ctx, "/downloader/list", map[string]string{
		"type": kind, "status": status,
		"start": strconv.Itoa(start), "size": strconv.Itoa(size),
	}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// PhotoStatus 返回相册 AI 开关和索引进度。
func (c *Client) PhotoStatus(ctx context.Context) (map[string]any, error) {
	conf, err := c.Form(ctx, "/v2/album/conf/load", nil, nil)
	if err != nil {
		return nil, err
	}
	state, err := c.Form(ctx, "/v2/album/feed/search/ai_state", nil, nil)
	if err != nil {
		return nil, err
	}
	var cdata, sdata map[string]any
	_ = conf.DecodeData(&cdata)
	_ = state.DecodeData(&sdata)
	return map[string]any{"config": cdata, "state": sdata}, nil
}

// PhotoSearch 按文字、OCR 或相似图搜索。mode 取 ai、ocr、similar。
// similar 时 query 是 NAS 上的图片路径。
func (c *Client) PhotoSearch(ctx context.Context, mode, query string, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	modal := "1"
	content := query
	if mode == "similar" {
		modal = "0"
	}
	params := map[string]string{
		"input_modal": modal, "query_content": content, "top_limit": strconv.Itoa(limit),
	}
	if mode == "ocr" {
		params["start"] = "0"
		params["num"] = strconv.Itoa(limit)
		env, err := c.Form(ctx, "/v2/album/ai/ocr/search", params, nil)
		if err != nil {
			return nil, err
		}
		return env.Data, nil
	}
	if _, err := c.Form(ctx, "/v2/album/feed/search/create", params, nil); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		env, err := c.Form(ctx, "/v2/album/feed/search/query", params, nil)
		if err != nil {
			return nil, err
		}
		var data struct {
			Status any `json:"status"`
		}
		_ = env.DecodeData(&data)
		if fmt.Sprint(data.Status) != "1" || time.Now().After(deadline) {
			return env.Data, nil
		}
		time.Sleep(time.Second)
	}
}

// Download 把 NAS 文件下载到本地路径。
func (c *Client) Download(ctx context.Context, remote, local string) error {
	vals := url.Values{}
	vals.Set("rnd", rnd())
	vals.Set("webagent", "v2")
	vals.Set("path", remote)
	vals.Set("remote_port", "8050")
	u := c.BaseURL + "/v2/file/download?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", c.cookieHeader())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	ctype := resp.Header.Get("Content-Type")
	if strings.Contains(ctype, "json") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var env Envelope
		if json.Unmarshal(raw, &env) == nil && !env.OK() {
			return &Error{Code: env.CodeString(), Msg: env.Msg}
		}
		return fmt.Errorf("下载失败：%s", truncate(raw, 200))
	}
	if err := os.MkdirAll(path.Dir(local), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(local, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// Upload 上传本地文件。小于阈值时整文件一次提交，否则按桌面客户端的分片协议上传。
func (c *Client) Upload(ctx context.Context, localPath, remoteDir, name string) (map[string]any, error) {
	st, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = path.Base(localPath)
	}
	target := path.Clean(remoteDir + "/" + name)
	if st.Size() <= DefaultSliceThreshold {
		return c.uploadWhole(ctx, localPath, target, st)
	}
	return c.uploadSliced(ctx, localPath, target, st)
}

func (c *Client) uploadWhole(ctx context.Context, localPath, target string, st os.FileInfo) (map[string]any, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	env, err := c.postUpload(ctx, target, st, f, st.Size(), 0, false)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	data["path"] = target
	return data, nil
}

func (c *Client) uploadSliced(ctx context.Context, localPath, target string, st os.FileInfo) (map[string]any, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	total := st.Size()
	var sent int64
	for sent < total {
		n := int64(c.SliceSize)
		if remain := total - sent; remain < n {
			n = remain
		}
		if _, err := c.postUpload(ctx, target, st, io.LimitReader(f, n), n, sent, true); err != nil {
			return nil, err
		}
		sent += n
	}
	return map[string]any{"path": target, "name": path.Base(target), "size": total}, nil
}

func (c *Client) postUpload(ctx context.Context, target string, st os.FileInfo, body io.Reader, length, seek int64, split bool) (Envelope, error) {
	mtimeMs := int64(math.Ceil(float64(st.ModTime().UnixNano()) / 1e6))
	sum := md5.Sum([]byte(fmt.Sprintf("%d%s", mtimeMs+st.Size(), target)))
	uuid := hex.EncodeToString(sum[:])
	params := map[string]string{
		"app": "file", "path": url.QueryEscape(target),
		"size": strconv.FormatInt(st.Size(), 10), "uuid": uuid,
		"seek": strconv.FormatInt(seek, 10), "crtime": "",
		"modify_time": strconv.FormatInt(int64(math.Ceil(float64(mtimeMs)/1000)), 10),
		"rename":      "0", "token": url.QueryEscape(c.Creds.Token),
		"plat": "pc", "nasid": c.Creds.NasID,
		"version": c.Creds.AppVersion, "device_id": c.Creds.DeviceID,
		"device":          url.QueryEscape(c.Creds.Device),
		"Content-Length":  strconv.FormatInt(length, 10),
		"request-purpose": "4", "remote-port": "8050",
	}
	if split {
		params["split"] = "1"
	}
	var cookie []string
	for k, v := range params {
		key := k
		if key == "nasid" {
			key = "nas_id"
		}
		cookie = append(cookie, key+"="+url.QueryEscape(v))
	}
	u := fmt.Sprintf("%s/v2/file/upload?remote_port=8050&drnd=%d&uuid=%s", c.BaseURL, time.Now().UnixMilli(), uuid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return Envelope{}, err
	}
	req.ContentLength = length
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Cookie", strings.Join(cookie, "; "))
	for k, v := range params {
		req.Header.Set(k, v)
	}
	return c.do(req)
}
