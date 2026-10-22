// Package mcp 实现 MCP stdio 服务。
//
// 协议只覆盖 tools/list 和 tools/call，足够接入 Claude、Cursor 等客户端。
// 读工具默认开启。写工具需要启动参数 --write，删除类工具还需要 --allow-delete。
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vangie/zspace-cli/internal/api"
)

// version 由 -ldflags 覆盖。
var version = "dev"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// tool 描述一个 MCP 工具。mode 决定它在什么开关下可见。
type tool struct {
	name        string
	description string
	mode        string // read, write, delete
	schema      map[string]any
	call        func(ctx context.Context, c *api.Client, args map[string]any) (any, error)
}

// Run 从 stdin 读 JSON-RPC，向 stdout 写响应。日志走 stderr。
func Run(args []string) int {
	fs := flag.NewFlagSet("zspace-mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	write := fs.Bool("write", false, "")
	allowDelete := fs.Bool("allow-delete", false, "")
	configDir := fs.String("config-dir", "", "")
	baseURL := fs.String("base-url", "", "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	tools := toolsFor(*write, *allowDelete)
	in := bufio.NewReader(os.Stdin)
	for {
		body, err := readMessage(in)
		if errors.Is(err, io.EOF) {
			return 0
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "读取消息失败：", err)
			return 1
		}
		var req request
		if err := json.Unmarshal(body, &req); err != nil {
			writeMessage(errorResponse(nil, -32700, "parse error"))
			continue
		}
		if len(req.ID) == 0 {
			continue // notification
		}
		writeMessage(handle(req, tools, *configDir, *baseURL))
	}
}

func handle(req request, tools []tool, configDir, baseURL string) response {
	switch req.Method {
	case "initialize":
		return response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "zspace-cli", "version": version},
		}}
	case "ping":
		return response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		list := make([]any, 0, len(tools))
		for _, t := range tools {
			list = append(list, map[string]any{
				"name": "zspace_" + t.name, "description": t.description, "inputSchema": t.schema,
			})
		}
		return response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": list}}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, -32602, "invalid params")
		}
		name := strings.TrimPrefix(params.Name, "zspace_")
		var t *tool
		for i := range tools {
			if tools[i].name == name {
				t = &tools[i]
				break
			}
		}
		if t == nil {
			return errorResponse(req.ID, -32602, "unknown or disabled tool")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		creds, err := api.LoadCredentials(configDir)
		if err != nil {
			return toolError(req.ID, err)
		}
		c, err := api.New(creds, baseURL)
		if err != nil {
			return toolError(req.ID, err)
		}
		out, err := t.call(ctx, c, params.Arguments)
		if err != nil {
			return toolError(req.ID, err)
		}
		buf, _ := json.Marshal(out)
		return response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"content": []any{map[string]any{"type": "text", "text": string(buf)}},
		}}
	default:
		return errorResponse(req.ID, -32601, "method not found")
	}
}

func toolError(id json.RawMessage, err error) response {
	return response{JSONRPC: "2.0", ID: id, Result: map[string]any{
		"content": []any{map[string]any{"type": "text", "text": err.Error()}},
		"isError": true,
	}}
}

func errorResponse(id json.RawMessage, code int, msg string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

func readMessage(in *bufio.Reader) ([]byte, error) {
	var length int
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		var n int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &n); err == nil {
			length = n
		}
	}
	if length <= 0 {
		return nil, errors.New("missing Content-Length")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(in, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeMessage(resp response) {
	buf, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(buf), buf)
}

func numArg(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return def
		}
		return n
	default:
		return def
	}
}

func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func schema(required []string, props map[string]any) map[string]any {
	return map[string]any{
		"type": "object", "properties": props, "required": required, "additionalProperties": false,
	}
}

func textProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func toolsFor(write, allowDelete bool) []tool {
	all := []tool{
		{"check", "检查极空间客户端代理是否在线，并返回存储池信息", "read",
			schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) {
				pools, err := c.PoolInfo(ctx)
				if err != nil {
					return nil, err
				}
				return map[string]any{"connected": true, "base_url": c.BaseURL, "pools": pools}, nil
			}},
		{"ls", "列出 NAS 目录", "read",
			schema([]string{"path"}, map[string]any{"path": textProp("NAS 绝对路径")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.List(ctx, str(a, "path"), false)
			}},
		{"info", "文件或目录详情", "read",
			schema([]string{"path"}, map[string]any{"path": textProp("NAS 绝对路径")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Info(ctx, str(a, "path"))
			}},
		{"find", "全文搜索文件", "read",
			schema([]string{"keyword"}, map[string]any{"keyword": textProp("关键词")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Search(ctx, str(a, "keyword"))
			}},
		{"pools", "存储池信息", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.PoolInfo(ctx) }},
		{"disks", "磁盘统计", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.DiskStats(ctx) }},
		{"usage", "磁盘物理用量快照", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.DiskUsage(ctx) }},
		{"docker_ps", "列出 NAS 上的 Docker 容器", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.Containers(ctx) }},
		{"docker_images", "列出 NAS 上的本地镜像", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.Images(ctx) }},
		{"docker_projects", "列出 Compose 项目", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.Projects(ctx) }},
		{"docker_info", "Docker 引擎版本与状态", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) {
				info, err := c.DockerInfo(ctx)
				if err != nil {
					return nil, err
				}
				status, err := c.SystemStatus(ctx)
				if err != nil {
					return nil, err
				}
				return map[string]any{"versions": info, "status": status}, nil
			}},
		{"photo_status", "相册 AI 开关和索引进度", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.PhotoStatus(ctx) }},
		{"photo_home", "相册首页：我的相册、系统目录、喜欢", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.AlbumHome(ctx) }},
		{"photo_albums", "相册列表", "read",
			schema(nil, map[string]any{"start": map[string]any{"type": "number"}, "num": map[string]any{"type": "number"}}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Albums(ctx, numArg(a, "start", 0), numArg(a, "num", 50))
			}},
		{"photo_list", "全部照片和视频", "read",
			schema(nil, map[string]any{"start": map[string]any{"type": "number"}, "num": map[string]any{"type": "number"}}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Photos(ctx, numArg(a, "start", 0), numArg(a, "num", 50), false)
			}},
		{"photo_timeline", "照片时间线分组", "read", schema(nil, map[string]any{}),
			func(ctx context.Context, c *api.Client, _ map[string]any) (any, error) { return c.PhotoTimeline(ctx) }},
		{"cloud_status", "网盘登录状态", "read",
			schema([]string{"provider"}, map[string]any{"provider": textProp("baidu 或 quark")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.CloudStatus(ctx, api.CloudProvider(str(a, "provider")))
			}},

		{"rename", "重命名 NAS 文件", "write",
			schema([]string{"path", "new_name"}, map[string]any{"path": textProp("原路径"), "new_name": textProp("新文件名")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Rename(ctx, str(a, "path"), str(a, "new_name"))
			}},
		{"mkdir", "新建目录", "write",
			schema([]string{"parent", "name"}, map[string]any{"parent": textProp("父目录"), "name": textProp("目录名")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Mkdir(ctx, str(a, "parent"), str(a, "name"))
			}},
		{"move", "移动文件", "write",
			schema([]string{"path", "to"}, map[string]any{"path": textProp("原路径"), "to": textProp("目标目录")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				env, err := c.Move(ctx, []string{str(a, "path")}, str(a, "to"))
				return env.Data, err
			}},
		{"copy", "复制文件", "write",
			schema([]string{"path", "to"}, map[string]any{"path": textProp("原路径"), "to": textProp("目标目录")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				env, err := c.Copy(ctx, []string{str(a, "path")}, str(a, "to"))
				return env.Data, err
			}},
		{"upload", "上传本地文件到 NAS", "write",
			schema([]string{"local_path", "remote_dir"}, map[string]any{
				"local_path": textProp("本地文件"), "remote_dir": textProp("NAS 目录"),
			}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				return c.Upload(ctx, str(a, "local_path"), str(a, "remote_dir"), "")
			}},
		{"docker_op", "对容器执行 start/stop/restart/pause/resume", "write",
			schema([]string{"container_id", "action"}, map[string]any{
				"container_id": textProp("容器 ID"), "action": textProp("start、stop、restart、pause、resume"),
			}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				action := str(a, "action")
				if action == "remove" || action == "force_remove" || action == "force_stop" {
					return nil, errors.New("删除和强制停止需要 --allow-delete")
				}
				if err := c.ContainerOp(ctx, str(a, "container_id"), action); err != nil {
					return nil, err
				}
				return map[string]any{"ok": true}, nil
			}},

		{"remove", "把文件移入个人回收站", "delete",
			schema([]string{"path"}, map[string]any{"path": textProp("NAS 路径")}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				env, err := c.Remove(ctx, []string{str(a, "path")})
				return env.Data, err
			}},
		{"docker_remove", "删除容器", "delete",
			schema([]string{"container_id"}, map[string]any{
				"container_id": textProp("容器 ID"), "force": map[string]any{"type": "boolean"},
			}),
			func(ctx context.Context, c *api.Client, a map[string]any) (any, error) {
				action := "remove"
				if b, _ := a["force"].(bool); b {
					action = "force_remove"
				}
				if err := c.ContainerOp(ctx, str(a, "container_id"), action); err != nil {
					return nil, err
				}
				return map[string]any{"ok": true}, nil
			}},
	}
	var out []tool
	for _, t := range all {
		switch t.mode {
		case "read":
			out = append(out, t)
		case "write":
			if write {
				out = append(out, t)
			}
		case "delete":
			if allowDelete {
				out = append(out, t)
			}
		}
	}
	return out
}
