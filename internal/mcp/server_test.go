package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInitializeAndToolGating(t *testing.T) {
	read := toolsFor(false, false)
	if len(read) == 0 {
		t.Fatal("默认应暴露读工具")
	}
	for _, tool := range read {
		if tool.mode != "read" {
			t.Fatalf("默认暴露了 %s（%s）", tool.name, tool.mode)
		}
	}
	write := toolsFor(true, false)
	delete := toolsFor(true, true)
	if len(write) <= len(read) || len(delete) <= len(write) {
		t.Fatalf("read %d write %d delete %d", len(read), len(write), len(delete))
	}

	resp := handle(request{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize",
	}, read, "", "")
	buf, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(buf), "zspace-cli") {
		t.Fatalf("initialize result %s", buf)
	}

	resp = handle(request{
		JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/list",
	}, read, "", "")
	buf, _ = json.Marshal(resp.Result)
	if !strings.Contains(string(buf), "zspace_docker_ps") || strings.Contains(string(buf), "zspace_remove") {
		t.Fatalf("tools/list 未按开关过滤：%s", buf)
	}
}
