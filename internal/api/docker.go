package api

import (
	"context"
	"encoding/json"
	"fmt"
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
// extra 覆盖骨架里的同名顶层字段，例如 Env、Cmd、HostConfig。
func (c *Client) CreateContainer(ctx context.Context, name, image string, extra map[string]any) (map[string]any, error) {
	env, err := c.PostJSON(ctx, "/zdocker/container_create",
		map[string]string{"name": name}, containerBody(image, extra))
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
