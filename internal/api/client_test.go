package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnvelopeCode(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{`{"code":"200","msg":"200"}`, "200", true},
		{`{"code":200,"msg":"200"}`, "200", true},
		{`{"code":"N001411","msg":"无权限"}`, "N001411", false},
	}
	for _, tc := range cases {
		var env Envelope
		if err := json.Unmarshal([]byte(tc.raw), &env); err != nil {
			t.Fatal(err)
		}
		if env.CodeString() != tc.want || env.OK() != tc.ok {
			t.Fatalf("%s -> code %q ok %v", tc.raw, env.CodeString(), env.OK())
		}
	}
}

func TestFormCarriesAuthAndPaths(t *testing.T) {
	var got struct {
		body   string
		cookie string
		path   string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.body = string(b)
		got.cookie = r.Header.Get("Cookie")
		got.path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"200","msg":"200","data":{"list":[]}}`))
	}))
	defer srv.Close()

	c := &Client{
		BaseURL: srv.URL,
		Creds:   Credentials{Token: "TOK", NasID: "NAS", DeviceID: "DEV"},
		HTTP:    srv.Client(),
	}
	if _, err := c.Move(context.Background(), []string{"/a", "/b"}, "/dest"); err != nil {
		t.Fatal(err)
	}
	if got.path != "/v2/file/move" {
		t.Fatalf("path %s", got.path)
	}
	for _, want := range []string{"token=TOK", "nasid=NAS", "plat=web", "to=%2Fdest", "paths%5B%5D=%2Fa", "paths%5B%5D=%2Fb"} {
		if !strings.Contains(got.body, want) {
			t.Fatalf("body 缺少 %s：%s", want, got.body)
		}
	}
	if !strings.Contains(got.cookie, "zenithtoken=TOK") || !strings.Contains(got.cookie, "nas_id=NAS") {
		t.Fatalf("cookie %s", got.cookie)
	}
}

func TestDockerCreateUsesJSONAndQueryName(t *testing.T) {
	var got struct {
		name        string
		contentType string
		body        map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.name = r.URL.Query().Get("name")
		got.contentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		_, _ = w.Write([]byte(`{"code":"200","msg":"200","data":{"Id":"abc","Warnings":[]}}`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N"}, HTTP: srv.Client()}
	data, err := c.CreateContainer(context.Background(), "demo", "hello-world", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.name != "demo" || !strings.Contains(got.contentType, "application/json") {
		t.Fatalf("name %q type %q", got.name, got.contentType)
	}
	if got.body["Image"] != "hello-world" {
		t.Fatalf("image %v", got.body["Image"])
	}
	host, _ := got.body["HostConfig"].(map[string]any)
	if host["NetworkMode"] != "bridge" {
		t.Fatalf("host %+v", host)
	}
	if data["Id"] != "abc" {
		t.Fatalf("data %+v", data)
	}
}

func TestBusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":"N001212","msg":"参数有误"}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T"}, HTTP: srv.Client()}
	_, err := c.PoolInfo(context.Background())
	ze, ok := err.(*Error)
	if !ok || ze.Code != "N001212" {
		t.Fatalf("err %#v", err)
	}
}

func TestContainerActionGuard(t *testing.T) {
	c := &Client{}
	if err := c.ContainerOp(context.Background(), "id", "explode"); err == nil {
		t.Fatal("非法 action 应被拒绝")
	}
}
