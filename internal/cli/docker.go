package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vangie/zspace-cli/internal/api"
)

func cmdDocker(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	if len(args) == 0 {
		return nil, 2, errors.New("用法：zspace docker <info|ps|images|networks|projects|proxy|proxy-set|pull|load|create|op|rmi|compose|compose-create>")
	}
	sub := args[0]
	args = args[1:]
	switch sub {
	case "info":
		status, err := c.SystemStatus(ctx)
		if err != nil {
			return nil, 1, err
		}
		info, err := c.DockerInfo(ctx)
		if err != nil {
			return nil, 1, err
		}
		sys, err := c.SystemInfo(ctx)
		if err != nil {
			return nil, 1, err
		}
		return map[string]any{"status": status, "versions": info, "engine": sys}, 0, nil
	case "proxy":
		cfg, err := c.ProxyConfig(ctx)
		if err != nil {
			return nil, 1, err
		}
		return cfg, 0, nil
	case "proxy-set":
		if err := confirm(o, "修改 Docker 引擎代理"); err != nil {
			return nil, 2, err
		}
		cfg, err := proxyConfigFromFlags(o)
		if err != nil {
			return nil, 2, err
		}
		if err := c.SaveProxyConfig(ctx, cfg); err != nil {
			return nil, 1, err
		}
		waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		info, err := c.WaitDockerReady(waitCtx, api.ProxyURL(cfg.HTTP), api.ProxyURL(cfg.HTTPS))
		if err != nil {
			return map[string]any{"saved": true, "restart": "timeout"}, 1, err
		}
		got, err := c.ProxyConfig(ctx)
		if err != nil {
			return map[string]any{
				"saved": true, "restart": "done",
				"http_proxy": info["HttpProxy"], "https_proxy": info["HttpsProxy"],
				"read_error": err.Error(),
			}, 1, nil
		}
		return map[string]any{
			"saved": true, "restart": "done", "proxy": got,
			"http_proxy": info["HttpProxy"], "https_proxy": info["HttpsProxy"],
		}, 0, nil
	case "ps":
		return call(c.Containers(ctx))
	case "images":
		return call(c.Images(ctx))
	case "networks":
		return call(c.Networks(ctx))
	case "projects":
		return call(c.Projects(ctx))
	case "pull":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace docker pull <镜像>")
		}
		if err := confirm(o, "拉取镜像 "+args[0]); err != nil {
			return nil, 2, err
		}
		task, err := c.PullImage(ctx, args[0])
		if err != nil {
			return nil, 1, err
		}
		id := fmt.Sprint(task["id"])
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		final, err := c.WaitPull(waitCtx, id)
		if err != nil {
			return map[string]any{"task": task, "error": err.Error()}, 1, nil
		}
		return map[string]any{"task": task, "final": final}, 0, nil
	case "load":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace docker load <NAS上的tar路径> --yes")
		}
		if err := confirm(o, "导入镜像 "+args[0]); err != nil {
			return nil, 2, err
		}
		if err := c.ImportImage(ctx, args[0]); err != nil {
			return nil, 1, err
		}
		return map[string]any{"image_file": args[0], "imported": true}, 0, nil
	case "create":
		if len(args) < 2 {
			return nil, 2, errors.New("用法：zspace docker create <镜像> <名字> --yes [--publish 宿主机:容器] [--volume 宿主机:容器[:ro]] [--env NAME=VALUE] [--network bridge] [--restart no|always|unless-stopped|on-failure]")
		}
		if err := confirm(o, "创建容器 "+args[1]); err != nil {
			return nil, 2, err
		}
		body := map[string]any{}
		if err := api.ApplyContainerSpec(body, api.ContainerSpec{
			Publish: o.publish, Volumes: o.volumes, Env: o.env,
			Network: o.network, Restart: o.restart,
		}); err != nil {
			return nil, 2, err
		}
		return call(c.CreateContainer(ctx, args[1], args[0], body))
	case "op":
		if len(args) < 2 {
			return nil, 2, fmt.Errorf("用法：zspace docker op <action> <容器ID> --yes\naction：%s", strings.Join(api.ContainerActions, ", "))
		}
		if err := confirm(o, args[0]+" 容器 "+args[1]); err != nil {
			return nil, 2, err
		}
		if err := c.ContainerOp(ctx, args[1], args[0]); err != nil {
			return nil, 1, err
		}
		return map[string]any{"container_id": args[1], "action": args[0], "ok": true}, 0, nil
	case "rmi":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace docker rmi <镜像ID> --yes")
		}
		if err := confirm(o, "删除镜像 "+args[0]); err != nil {
			return nil, 2, err
		}
		if err := c.RemoveImage(ctx, args[0], o.force); err != nil {
			return nil, 1, err
		}
		return map[string]any{"image_id": args[0], "removed": true}, 0, nil
	case "compose-create":
		if len(args) < 3 {
			return nil, 2, errors.New("用法：zspace docker compose-create <项目名> <yaml文件> <NAS目录> --yes")
		}
		if err := confirm(o, "创建 Compose 项目 "+args[0]); err != nil {
			return nil, 2, err
		}
		raw, err := os.ReadFile(args[1])
		if err != nil {
			return nil, 1, err
		}
		return call(c.CreateProject(ctx, api.ComposeProject{
			Name: args[0], Config: string(raw), Path: args[2], Start: o.start,
		}))
	case "compose":
		if len(args) < 2 {
			return nil, 2, errors.New("用法：zspace docker compose <start|stop|restart|delete|rebuild> <项目ID> --yes")
		}
		if err := confirm(o, args[0]+" Compose 项目 "+args[1]); err != nil {
			return nil, 2, err
		}
		if err := c.ComposeOp(ctx, args[1], args[0]); err != nil {
			return nil, 1, err
		}
		return map[string]any{"project_id": args[1], "action": args[0], "ok": true}, 0, nil
	default:
		return nil, 2, fmt.Errorf("未知子命令 docker %s", sub)
	}
}

func proxyConfigFromFlags(o opts) (api.ProxyConfig, error) {
	if o.proxyOff && (o.httpProxy != "" || o.httpsProxy != "") {
		return api.ProxyConfig{}, errors.New("--off 与 --http-proxy、--https-proxy 不能同时用")
	}
	if o.proxyOff {
		return api.ProxyConfig{Status: "0"}, nil
	}
	if o.httpProxy == "" && o.httpsProxy == "" {
		return api.ProxyConfig{}, errors.New("用法：zspace docker proxy-set --http-proxy 主机:端口 --yes")
	}
	httpURL, httpPort, err := splitProxy(o.httpProxy)
	if err != nil {
		return api.ProxyConfig{}, err
	}
	httpsProxy := o.httpsProxy
	if httpsProxy == "" {
		httpsProxy = o.httpProxy
	}
	httpsURL, httpsPort, err := splitProxy(httpsProxy)
	if err != nil {
		return api.ProxyConfig{}, err
	}
	return api.ProxyConfig{
		Status:   "1",
		HTTP:     api.ProxyEndpoint{URL: httpURL, Port: httpPort},
		HTTPS:    api.ProxyEndpoint{URL: httpsURL, Port: httpsPort},
		Username: o.proxyUser,
		Password: o.proxyPassword,
	}, nil
}

func splitProxy(raw string) (host, port string, err error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	host, port, ok := strings.Cut(raw, ":")
	if !ok || host == "" || port == "" || strings.Contains(port, ":") {
		return "", "", fmt.Errorf("代理 %q 需要 主机:端口", raw)
	}
	return host, port, nil
}

// 让 json.RawMessage 在人工输出时也能被编码。printHuman 对 docker 走 printJSON。
var _ = json.Marshal
