package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestApplyPublishAndVolumeKeepsDefaults(t *testing.T) {
	body := containerBody("demo:latest", nil)
	err := ApplyContainerSpec(body, ContainerSpec{
		Publish: []string{"8787:8787"},
		Volumes: []string{"/tmp/zfsv3/sata1/18501646420/data/docker_vol/ipad-frame:/data"},
		Env:     []string{"TZ=Asia/Shanghai"},
		Restart: "unless-stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	merged := mergeContainer(containerBody("demo:latest", nil), body)
	host := merged["HostConfig"].(map[string]any)
	if host["NetworkMode"] != "bridge" {
		t.Fatalf("network %v", host["NetworkMode"])
	}
	if host["Privileged"] != false {
		t.Fatalf("privileged %v", host["Privileged"])
	}
	bindings := host["PortBindings"].(map[string]any)
	got := bindings["8787/tcp"].([]any)[0].(map[string]any)["HostPort"]
	if got != "8787" {
		t.Fatalf("port %v", bindings)
	}
	exposed := merged["ExposedPorts"].(map[string]any)
	if _, ok := exposed["8787/tcp"]; !ok {
		t.Fatalf("exposed %v", exposed)
	}
	binds := host["Binds"].([]any)
	if len(binds) != 1 || !strings.Contains(binds[0].(string), ":/data") {
		t.Fatalf("binds %v", binds)
	}
	env := merged["Env"].([]any)
	if env[0] != "TZ=Asia/Shanghai" {
		t.Fatalf("env %v", env)
	}
	policy := host["RestartPolicy"].(map[string]any)
	if policy["Name"] != "unless-stopped" {
		t.Fatalf("restart %v", policy)
	}
}

func TestWaitDockerReadySeesRestart(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case strings.HasPrefix(r.URL.Path, "/zdocker/system_status"):
			status := "off"
			if calls > 4 {
				status = "on"
			}
			fmt.Fprintf(w, `{"code":"200","data":{"status":%q}}`, status)
		case strings.HasPrefix(r.URL.Path, "/zdocker/system_info"):
			proxy := ""
			if calls > 4 {
				proxy = "http://192.168.0.173:6152"
			}
			fmt.Fprintf(w, `{"code":"200","data":{"HttpProxy":%q,"HttpsProxy":%q}}`, proxy, proxy)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T"}, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	info, err := c.WaitDockerReady(ctx, "http://192.168.0.173:6152", "http://192.168.0.173:6152")
	if err != nil {
		t.Fatal(err)
	}
	if info["HttpProxy"] != "http://192.168.0.173:6152" {
		t.Fatalf("info %+v", info)
	}
}

func TestApplyRejectsBadSpec(t *testing.T) {
	body := containerBody("demo", nil)
	if err := ApplyContainerSpec(body, ContainerSpec{Publish: []string{"bad:port:extra"}}); err == nil {
		t.Fatal("坏端口应被拒绝")
	}
	if err := ApplyContainerSpec(body, ContainerSpec{Volumes: []string{"/only"}}); err == nil {
		t.Fatal("坏挂载应被拒绝")
	}
	if err := ApplyContainerSpec(body, ContainerSpec{Env: []string{"NOVALUE"}}); err == nil {
		t.Fatal("坏环境变量应被拒绝")
	}
}
