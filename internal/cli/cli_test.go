package cli

import (
	"strings"
	"testing"
)

func TestHelpListsPhotoCommands(t *testing.T) {
	if code := Run([]string{"help"}); code != 0 {
		t.Fatalf("help exit %d", code)
	}
}

func TestUnknownCommand(t *testing.T) {
	if code := Run([]string{"no-such"}); code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestPhotoFlagsDoNotCollide(t *testing.T) {
	// 不连接 NAS。photo 的分页选项曾与 docker compose 的 --start 撞名，
	// 撞名时 flag 在解析阶段 panic，退出码不是 1。
	code := Run([]string{"photo", "albums", "--offset", "0", "--limit", "1", "--base-url", "http://127.0.0.1:1"})
	if code != 1 {
		t.Fatalf("连不上代理应返回 1，实际 %d。退出码 2 通常是选项解析失败", code)
	}
}

func TestWriteRequiresYes(t *testing.T) {
	code := Run([]string{"photo", "create", "probe", "--base-url", "http://127.0.0.1:1"})
	if code != 2 {
		t.Fatalf("缺少 --yes 应返回 2，实际 %d", code)
	}
}

func TestUsageMentionsDocker(t *testing.T) {
	if !strings.Contains(usage, "docker ps") || !strings.Contains(usage, "photo albums") || !strings.Contains(usage, "video classes") || !strings.Contains(usage, "music songs") || !strings.Contains(usage, "ddns add") || !strings.Contains(usage, "docker proxy-set") || !strings.Contains(usage, "--publish") {
		t.Fatal("帮助文本缺命令")
	}
}

func TestDockerProxySetRequiresYes(t *testing.T) {
	code := Run([]string{"docker", "proxy-set", "--http-proxy", "192.168.0.173:6152", "--base-url", "http://127.0.0.1:1"})
	if code != 2 {
		t.Fatalf("缺少 --yes 应返回 2，实际 %d", code)
	}
}

func TestDockerCreateFlagsParse(t *testing.T) {
	code := Run([]string{
		"docker", "create", "demo:latest", "probe", "--yes",
		"--publish", "8787:8787", "--volume", "/tmp/data:/data",
		"--env", "TZ=Asia/Shanghai", "--restart", "unless-stopped",
		"--base-url", "http://127.0.0.1:1",
	})
	if code != 1 {
		t.Fatalf("选项应能解析，连不上代理返回 1，实际 %d", code)
	}
}
