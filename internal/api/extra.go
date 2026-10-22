package api

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// RecycleSetConfig 设置回收站保留天数。-1 表示永不自动清除。
// 未给出的字段保持当前值。
func (c *Client) RecycleSetConfig(ctx context.Context, my, public *int) (map[string]any, error) {
	current, err := c.RecycleConfig(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]string{
		"my_cycle":     cycleValue(current, "my_cycle", my),
		"public_cycle": cycleValue(current, "public_cycle", public),
	}
	if _, err := c.Form(ctx, "/v2/file/recycle/config/save", body, nil); err != nil {
		return nil, err
	}
	return map[string]any{
		"my_cycle": body["my_cycle"], "public_cycle": body["public_cycle"],
	}, nil
}

func cycleValue(current map[string]any, key string, override *int) string {
	if override != nil {
		return strconv.Itoa(*override)
	}
	switch v := current[key].(type) {
	case float64:
		return strconv.Itoa(int(v))
	case string:
		if v != "" {
			return v
		}
	}
	return "-1"
}

// Glob 按绝对路径通配符列出 NAS 文件。支持 *、?、[...] 和 **。
// 扫描起点是第一个通配符所在的目录。
func (c *Client) Glob(ctx context.Context, pattern string) ([]FileEntry, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, fmt.Errorf("通配符必须是绝对路径：%s", pattern)
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return nil, fmt.Errorf("路径里没有通配符：%s", pattern)
	}
	re, err := globRegexp(pattern)
	if err != nil {
		return nil, err
	}
	root := "/"
	for i, ch := range pattern {
		if strings.ContainsRune("*?[", ch) {
			root = pattern[:i]
			if cut := strings.LastIndex(root, "/"); cut >= 0 {
				root = root[:cut]
			}
			if root == "" {
				root = "/"
			}
			break
		}
	}
	// 只有 ** 才递归。单层 * 只看它所在的那一层，
	// 否则 /pool/my/data/* 会把整个媒体库都列一遍。
	recursive := strings.Contains(pattern, "**")
	var matches []FileEntry
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := c.List(ctx, dir, false)
		if err != nil {
			return nil
		}
		for _, e := range entries {
			if re.MatchString(e.Path) {
				matches = append(matches, e)
			}
			if recursive && e.Dir() {
				if err := walk(e.Path); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	return matches, nil
}

func globRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString("(?:.*/)?")
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					i++
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '[':
			j := strings.IndexByte(pattern[i+1:], ']')
			if j < 0 {
				b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
				break
			}
			b.WriteString(pattern[i : i+2+j])
			i += 2 + j
			continue
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
		i++
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// Expand 把一个参数展开成路径列表。含通配符时走 Glob，否则原样返回。
func (c *Client) Expand(ctx context.Context, arg string) ([]string, error) {
	if strings.ContainsAny(arg, "*?[") {
		entries, err := c.Glob(ctx, arg)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Path)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("没有匹配：%s", arg)
		}
		return out, nil
	}
	return []string{arg}, nil
}

// CloudUserInfo 返回已登录网盘的账号信息。百度有配额，夸克返回其 auth/userinfo。
func (c *Client) CloudUserInfo(ctx context.Context, p CloudProvider) (json.RawMessage, error) {
	base, err := p.base()
	if err != nil {
		return nil, err
	}
	env, err := c.Form(ctx, base+"/auth/userinfo", nil, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CloudDownload 把网盘文件下载到 NAS 目录。
// 百度用 file_ids，夸克同时接受 file_ids 和 folder_ids，逗号分隔。
func (c *Client) CloudDownload(ctx context.Context, p CloudProvider, savePath, fileIDs, folderIDs string) (json.RawMessage, error) {
	base, err := p.base()
	if err != nil {
		return nil, err
	}
	extra := map[string]string{"save_path": savePath}
	var endpoint string
	switch p {
	case CloudBaidu:
		endpoint = base + "/file/download"
		extra["file_ids"] = fileIDs
	default:
		endpoint = base + "/task/download"
		if fileIDs != "" {
			extra["file_ids"] = fileIDs
		}
		if folderIDs != "" {
			extra["folder_ids"] = folderIDs
		}
	}
	env, err := c.Form(ctx, endpoint, extra, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CloudFails 列出网盘传输失败的文件。taskID 为空时返回全部失败。
func (c *Client) CloudFails(ctx context.Context, p CloudProvider, taskID string) (json.RawMessage, error) {
	base, err := p.base()
	if err != nil {
		return nil, err
	}
	extra := map[string]string{"page": "1", "limit": "50"}
	if taskID != "" {
		extra["task_id"] = taskID
	}
	env, err := c.Form(ctx, base+"/fail/list", extra, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// cloudTaskActions 是网页版任务操作。带 all 的不需要 task_id。
var cloudTaskActions = map[string]bool{
	"resume": true, "pause": true, "clean": true,
	"pause_all": true, "resume_all": true, "clean_all": true,
	"clean_all_done": true, "resume_fail_all": true, "clean_fail_all": true,
}

// CloudTaskAction 操作网盘传输任务。clean 类会删除任务记录。
func (c *Client) CloudTaskAction(ctx context.Context, p CloudProvider, method, taskID string) error {
	if !cloudTaskActions[method] {
		return fmt.Errorf("未知任务操作 %q", method)
	}
	if !strings.Contains(method, "all") && taskID == "" {
		return fmt.Errorf("%s 需要任务 ID", method)
	}
	base, err := p.base()
	if err != nil {
		return err
	}
	extra := map[string]string{"method": method}
	if taskID != "" {
		extra["task_id"] = taskID
	}
	_, err = c.Form(ctx, base+"/task/action", extra, nil)
	return err
}

// CloudShareVerify 校验百度分享链接，返回 spwd，供 CloudShareList 使用。
func (c *Client) CloudShareVerify(ctx context.Context, shortURL, pwd string) (string, error) {
	env, err := c.Form(ctx, "/znetdisk/share/verify", map[string]string{
		"short_url": shortURL, "pwd": pwd,
	}, nil)
	if err != nil {
		return "", err
	}
	var data struct {
		Spwd string `json:"spwd"`
	}
	if err := env.DecodeData(&data); err != nil {
		return "", err
	}
	return data.Spwd, nil
}

// CloudShareList 列出百度分享链接里的文件。spwd 来自 CloudShareVerify。
func (c *Client) CloudShareList(ctx context.Context, shortURL, spwd, dir string) (json.RawMessage, error) {
	if dir == "" {
		dir = "/"
	}
	env, err := c.Form(ctx, "/znetdisk/share/filelist", map[string]string{
		"short_url": shortURL, "spwd": spwd, "path": dir, "page": "1", "limit": "50",
	}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}
