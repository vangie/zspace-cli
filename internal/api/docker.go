package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ContainerActions 是 /zdocker/container_op 接受的 action。
// 来自 NAS 网页版容器右键菜单（async/25400）。
var ContainerActions = []string{
	"start", "stop", "restart", "pause", "resume",
	"force_stop", "remove", "force_remove",
}

// SystemStatus 返回 Docker 服务状态，data.status 为 on 或 off。
func (c *Client) SystemStatus(ctx context.Context) (map[string]any, error) {
	return c.dockerGetMap(ctx, "/zdocker/system_status", nil)
}

// DockerInfo 返回 engine、compose、containerd、runc 版本。
func (c *Client) DockerInfo(ctx context.Context) (map[string]any, error) {
	return c.dockerGetMap(ctx, "/zdocker/docker_info", nil)
}

// SystemInfo 返回 Docker Engine /info 的原样结构。
func (c *Client) SystemInfo(ctx context.Context) (map[string]any, error) {
	return c.dockerGetMap(ctx, "/zdocker/system_info", nil)
}

// ProxyEndpoint 是引擎 HTTP 或 HTTPS 代理的主机和端口。
// 网页把地址和端口分开存，url 不含协议。
type ProxyEndpoint struct {
	URL  string `json:"url"`
	Port string `json:"port"`
}

// ProxyConfig 是 /zdocker/system_get_proxy_conf 的 data。
// status 为 "1" 表示开启。username、password 只在保存时提交，读取结果不含密码。
type ProxyConfig struct {
	Status   string        `json:"status"`
	HTTP     ProxyEndpoint `json:"http"`
	HTTPS    ProxyEndpoint `json:"https"`
	Username string        `json:"username,omitempty"`
	Password string        `json:"password,omitempty"`
}

// ProxyConfig 读取 Docker 引擎代理。返回里没有密码。
func (c *Client) ProxyConfig(ctx context.Context) (ProxyConfig, error) {
	env, err := c.Get(ctx, "/zdocker/system_get_proxy_conf", nil)
	if err != nil {
		return ProxyConfig{}, err
	}
	var cfg ProxyConfig
	if err := env.DecodeData(&cfg); err != nil {
		return ProxyConfig{}, err
	}
	cfg.Password = ""
	return cfg, nil
}

// SaveProxyConfig 保存引擎代理。请求体与网页确认按钮一致。
// 极空间保存后会重启 Docker，这个方法只提交配置，不等待重启。
func (c *Client) SaveProxyConfig(ctx context.Context, cfg ProxyConfig) error {
	if cfg.Status != "0" && cfg.Status != "1" {
		return fmt.Errorf("代理 status 只能是 0 或 1，收到 %q", cfg.Status)
	}
	_, err := c.PostJSON(ctx, "/zdocker/system_save_proxy_conf", nil, cfg)
	return err
}

// WaitDockerReady 等到引擎 /info 里的代理与期望值一致，且 system_status 为 on。
// 保存请求返回时重启可能已经结束，所以不再要求先观察到一次掉线。
func (c *Client) WaitDockerReady(ctx context.Context, wantHTTP, wantHTTPS string) (map[string]any, error) {
	var last error
	for {
		status := ""
		st, err := c.SystemStatus(ctx)
		if err != nil {
			last = err
		} else {
			status = fmt.Sprint(st["status"])
		}
		info, infoErr := c.SystemInfo(ctx)
		if infoErr != nil {
			last = infoErr
		}
		// 保存请求返回时重启可能已经结束，状态会一直是 on。
		// 引擎地址已经是目标值就不用再等一次掉线。
		if err == nil && status == "on" && infoErr == nil && proxyMatches(info, wantHTTP, wantHTTPS) {
			return info, nil
		}
		select {
		case <-ctx.Done():
			if last == nil {
				last = ctx.Err()
			}
			return nil, fmt.Errorf("等待 Docker 重启完成：%w", last)
		case <-time.After(2 * time.Second):
		}
	}
}

func proxyMatches(info map[string]any, wantHTTP, wantHTTPS string) bool {
	if wantHTTP == "" && wantHTTPS == "" {
		return fmt.Sprint(info["HttpProxy"]) == "" && fmt.Sprint(info["HttpsProxy"]) == ""
	}
	return fmt.Sprint(info["HttpProxy"]) == wantHTTP && fmt.Sprint(info["HttpsProxy"]) == wantHTTPS
}

// ProxyURL 把网页分开存的主机和端口拼成引擎 /info 里的地址。
func ProxyURL(ep ProxyEndpoint) string {
	if ep.URL == "" && ep.Port == "" {
		return ""
	}
	return "http://" + ep.URL + ":" + ep.Port
}

// ImportImage 从 NAS 上的 tar 导入镜像。imageFile 用文件接口路径，例如 /sata1/my/data/a.tar。
func (c *Client) ImportImage(ctx context.Context, imageFile string) error {
	_, err := c.Get(ctx, "/zdocker/image_import", map[string]string{"image_file_name": imageFile})
	return err
}

// Containers 列出容器。走 v2 接口，请求体为 {}。
func (c *Client) Containers(ctx context.Context) ([]map[string]any, error) {
	env, err := c.PostJSON(ctx, "/zdocker/v2/container_list", nil, map[string]any{})
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := env.DecodeData(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// ContainerInfo 取单个容器的 inspect。
func (c *Client) ContainerInfo(ctx context.Context, id string) (map[string]any, error) {
	return c.dockerGetMap(ctx, "/zdocker/container_info", map[string]string{"container_id": id})
}

// Images 列出本机镜像。
func (c *Client) Images(ctx context.Context) ([]map[string]any, error) {
	env, err := c.PostJSON(ctx, "/zdocker/v2/local_image_list", nil, map[string]any{})
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := env.DecodeData(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// Networks 列出 Docker 网络。
func (c *Client) Networks(ctx context.Context) (json.RawMessage, error) {
	env, err := c.Get(ctx, "/zdocker/network_list", nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// Projects 列出 Compose 项目。没有项目时 data 为 null。
func (c *Client) Projects(ctx context.Context) (json.RawMessage, error) {
	env, err := c.PostJSON(ctx, "/zdocker/v2/compose/list_projects", nil, map[string]any{})
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// PullImage 开始拉取镜像，返回任务描述。进度用 WaitPull 轮询。
func (c *Client) PullImage(ctx context.Context, ref string) (map[string]any, error) {
	return c.dockerGetMap(ctx, "/zdocker/image_pull", map[string]string{"image_name": ref})
}

// PullTasks 返回当前拉镜像任务列表。
func (c *Client) PullTasks(ctx context.Context) ([]map[string]any, error) {
	env, err := c.Get(ctx, "/zdocker/image_pull_task_list", nil)
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := env.DecodeData(&list); err != nil {
		return nil, err
	}
	return list, nil
}

// WaitPull 轮询直到任务进入终态。终态为 ===Pull Completed===、===Pull Cancelled===、Error。
func (c *Client) WaitPull(ctx context.Context, taskID string) (map[string]any, error) {
	var last map[string]any
	for {
		tasks, err := c.PullTasks(ctx)
		if err != nil {
			return nil, err
		}
		var found map[string]any
		for _, t := range tasks {
			if fmt.Sprint(t["id"]) == taskID {
				found = t
			}
		}
		if found == nil && last != nil {
			last["status"] = "===Pull Completed==="
			return last, nil
		}
		if found != nil {
			last = found
			switch fmt.Sprint(found["status"]) {
			case "===Pull Completed===", "===Pull Cancelled===", "Error":
				return found, nil
			}
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// ContainerSpec 是命令行能表达的创建参数。空字段沿用网页默认骨架。
type ContainerSpec struct {
	Publish    []string
	Volumes    []string
	Env        []string
	Network    string
	Restart    string
	Cmd        []string
	Entrypoint []string
}

// ApplyContainerSpec 把端口、挂载、环境变量写进创建请求体。
// 只改给出的字段，其余保持网页默认骨架。
// 端口写成 PortBindings 和 ExposedPorts。挂载写成 HostConfig.Binds。
func ApplyContainerSpec(body map[string]any, spec ContainerSpec) error {
	host, _ := body["HostConfig"].(map[string]any)
	if host == nil {
		host = map[string]any{}
		body["HostConfig"] = host
	}
	if len(spec.Publish) > 0 {
		bindings := map[string]any{}
		exposed := map[string]any{}
		for _, item := range spec.Publish {
			hostPort, containerPort, proto, err := parsePublish(item)
			if err != nil {
				return err
			}
			key := containerPort + "/" + proto
			bindings[key] = []any{map[string]any{"HostPort": hostPort}}
			exposed[key] = map[string]any{}
		}
		host["PortBindings"] = bindings
		body["ExposedPorts"] = exposed
	}
	if len(spec.Volumes) > 0 {
		binds := make([]any, 0, len(spec.Volumes))
		for _, item := range spec.Volumes {
			bind, err := parseBind(item)
			if err != nil {
				return err
			}
			binds = append(binds, bind)
		}
		host["Binds"] = binds
	}
	if len(spec.Env) > 0 {
		env := make([]any, 0, len(spec.Env))
		for _, item := range spec.Env {
			if !strings.Contains(item, "=") {
				return fmt.Errorf("环境变量 %q 需要 NAME=VALUE", item)
			}
			env = append(env, item)
		}
		body["Env"] = env
	}
	if spec.Network != "" {
		host["NetworkMode"] = spec.Network
		body["NetworkingConfig"] = map[string]any{
			"EndpointsConfig": map[string]any{
				spec.Network: map[string]any{"IPAMConfig": map[string]any{"IPv4Address": "", "IPv6Address": ""}},
			},
		}
	}
	if spec.Restart != "" {
		name := spec.Restart
		switch name {
		case "no", "always", "unless-stopped", "on-failure":
		default:
			return fmt.Errorf("未知重启策略 %q", name)
		}
		host["RestartPolicy"] = map[string]any{"Name": name, "MaximumRetryCount": 0}
	}
	if spec.Cmd != nil {
		body["Cmd"] = stringSliceAny(spec.Cmd)
	}
	if spec.Entrypoint != nil {
		body["Entrypoint"] = stringSliceAny(spec.Entrypoint)
	}
	return nil
}

func stringSliceAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func parsePublish(item string) (hostPort, containerPort, proto string, err error) {
	proto = "tcp"
	parts := strings.Split(item, ":")
	switch len(parts) {
	case 1:
		containerPort = parts[0]
		hostPort = parts[0]
	case 2:
		hostPort, containerPort = parts[0], parts[1]
	default:
		return "", "", "", fmt.Errorf("端口 %q 需要 宿主机:容器 或 端口", item)
	}
	if i := strings.IndexByte(containerPort, '/'); i >= 0 {
		proto = containerPort[i+1:]
		containerPort = containerPort[:i]
	}
	if hostPort == "" || containerPort == "" || (proto != "tcp" && proto != "udp") {
		return "", "", "", fmt.Errorf("端口 %q 无效", item)
	}
	return hostPort, containerPort, proto, nil
}

func parseBind(item string) (string, error) {
	parts := strings.Split(item, ":")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("挂载 %q 需要 宿主机路径:容器路径[:ro]", item)
	}
	if len(parts) == 3 && parts[2] != "ro" && parts[2] != "rw" {
		return "", fmt.Errorf("挂载 %q 的模式只能是 ro 或 rw", item)
	}
	return item, nil
}

// containerBody 是创建容器的默认骨架，字段与网页版 transformToApiParams 一致。
func containerBody(image string, extra map[string]any) map[string]any {
	body := map[string]any{
		"Image": image, "Hostname": "", "Domainname": "",
		"Cmd": nil, "Entrypoint": nil, "Env": []any{},
		"ExposedPorts": map[string]any{}, "Labels": map[string]any{},
		"MacAddress": "", "OpenStdin": false, "Tty": false,
		"Volumes": map[string]any{},
		"HostConfig": map[string]any{
			"AutoRemove": false, "Binds": []any{}, "CapAdd": []any{}, "CapDrop": []any{},
			"Devices": []any{}, "Dns": []any{}, "ExtraHosts": []any{}, "Init": false,
			"Memory": 0, "MemoryReservation": 0, "NanoCpus": 0,
			"NetworkMode": "bridge", "PortBindings": map[string]any{},
			"Privileged": false, "PublishAllPorts": false,
			"RestartPolicy": map[string]any{"Name": "no", "MaximumRetryCount": 0},
			"Runtime":       nil, "Sysctls": map[string]any{},
		},
		"NetworkingConfig": map[string]any{
			"EndpointsConfig": map[string]any{
				"bridge": map[string]any{"IPAMConfig": map[string]any{"IPv4Address": "", "IPv6Address": ""}},
			},
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

// CreateContainer 创建容器。name 走查询串，定义走 JSON 体。
// extra 的顶层字段和 HostConfig 字段分别合并进网页默认骨架。
func (c *Client) CreateContainer(ctx context.Context, name, image string, extra map[string]any) (map[string]any, error) {
	env, err := c.PostJSON(ctx, "/zdocker/container_create",
		map[string]string{"name": name}, mergeContainer(containerBody(image, nil), extra))
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// ContainerOp 对容器执行 action。action 必须在 ContainerActions 里。
func (c *Client) ContainerOp(ctx context.Context, id, action string) error {
	if !validAction(action) {
		return fmt.Errorf("未知 action %q，可选：%s", action, joinActions())
	}
	_, err := c.Get(ctx, "/zdocker/container_op", map[string]string{
		"container_id": id, "action": action,
	})
	return err
}

func validAction(a string) bool {
	for _, x := range ContainerActions {
		if x == a {
			return true
		}
	}
	return false
}

func joinActions() string {
	out := ""
	for i, a := range ContainerActions {
		if i > 0 {
			out += ", "
		}
		out += a
	}
	return out
}

// RemoveImage 删除镜像。imageID 用镜像列表里的 Id。force 走强制删除。
func (c *Client) RemoveImage(ctx context.Context, imageID string, force bool) error {
	endpoint := "/zdocker/image_remove"
	if force {
		endpoint = "/zdocker/image_force_remove"
	}
	_, err := c.Get(ctx, endpoint, map[string]string{"image_id": imageID})
	return err
}

// ComposeProject 是创建 Compose 项目的请求体，字段与网页版 doCreateCompose 一致。
type ComposeProject struct {
	Name   string `json:"project_name"`
	Config string `json:"config"`
	Path   string `json:"path"`
	Start  bool   `json:"start"`
	Env    string `json:"env,omitempty"`
	RW     bool   `json:"rw"`
}

// CreateProject 创建 Compose 项目，返回 data（含 task_id）。
func (c *Client) CreateProject(ctx context.Context, p ComposeProject) (map[string]any, error) {
	env, err := c.PostJSON(ctx, "/zdocker/v2/compose/create_project", nil, p)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// ComposeOp 对 Compose 项目执行 start、stop、restart、delete、rebuild。
func (c *Client) ComposeOp(ctx context.Context, projectID, action string) error {
	paths := map[string]string{
		"start":   "/zdocker/compose_start_project",
		"stop":    "/zdocker/compose_stop_project",
		"restart": "/zdocker/compose_restart_project",
		"delete":  "/zdocker/v2/compose/delete_project",
		"rebuild": "/zdocker/v2/compose/rebuild_project",
	}
	endpoint, ok := paths[action]
	if !ok {
		return fmt.Errorf("未知 compose action %q", action)
	}
	_, err := c.Get(ctx, endpoint, map[string]string{"project_id": projectID})
	return err
}

func mergeContainer(base, extra map[string]any) map[string]any {
	if extra == nil {
		return base
	}
	extraHost, _ := extra["HostConfig"].(map[string]any)
	for k, v := range extra {
		if k == "HostConfig" {
			continue
		}
		base[k] = v
	}
	if extraHost == nil {
		return base
	}
	host, _ := base["HostConfig"].(map[string]any)
	if host == nil {
		base["HostConfig"] = extraHost
		return base
	}
	for k, v := range extraHost {
		host[k] = v
	}
	return base
}

func (c *Client) dockerGetMap(ctx context.Context, endpoint string, query map[string]string) (map[string]any, error) {
	env, err := c.Get(ctx, endpoint, query)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := env.DecodeData(&data); err != nil {
		return nil, err
	}
	return data, nil
}
