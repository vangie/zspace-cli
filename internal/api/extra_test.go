package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGlobRegexp(t *testing.T) {
	re, err := globRegexp("/sata1/my/data/*.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("/sata1/my/data/a.mkv") {
		t.Fatal("应匹配单层")
	}
	if re.MatchString("/sata1/my/data/sub/a.mkv") {
		t.Fatal("* 不应跨目录")
	}
	deep, err := globRegexp("/sata1/my/data/**/*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if !deep.MatchString("/sata1/my/data/sub/a.mp4") {
		t.Fatal("** 应跨目录")
	}
}

func TestGlobWalksFromWildcardDir(t *testing.T) {
	var listed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		listed = append(listed, body)
		if strings.Contains(body, "path=%2Fsata1%2Fmy%2Fdata") && !strings.Contains(body, "video") {
			_, _ = w.Write([]byte(`{"code":"200","data":{"list":[{"name":"a.mkv","path":"/sata1/my/data/a.mkv","is_dir":"0","size":"3"},{"name":"video","path":"/sata1/my/data/video","is_dir":"1","size":"0"}],"total":"2"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"200","data":{"list":[{"name":"b.mkv","path":"/sata1/my/data/video/b.mkv","is_dir":"0","size":"4"}],"total":"1"}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	got, err := c.Glob(context.Background(), "/sata1/my/data/*.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/sata1/my/data/a.mkv" {
		t.Fatalf("matches %+v", got)
	}
	if len(listed) != 1 {
		t.Fatalf("单层 * 不应进入子目录，实际请求 %d 次", len(listed))
	}
}

func TestCloudAndRecycleForms(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.URL.Path+" "+string(b))
		_, _ = w.Write([]byte(`{"code":"200","msg":"200","data":{"my_cycle":-1,"public_cycle":30,"spwd":"S","is_login":true}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	ctx := context.Background()

	days := 7
	if _, err := c.RecycleSetConfig(ctx, &days, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CloudDownload(ctx, CloudQuark, "/sata1/my/data", "1", "2"); err != nil {
		t.Fatal(err)
	}
	if err := c.CloudTaskAction(ctx, CloudBaidu, "pause", ""); err == nil {
		t.Fatal("单任务操作缺少 task_id 应拒绝")
	}
	if err := c.CloudTaskAction(ctx, CloudBaidu, "pause_all", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CloudList(ctx, CloudQuark, "/", "", 1, 20); err != nil {
		t.Fatal(err)
	}
	spwd, err := c.CloudShareVerify(ctx, "abc", "1234")
	if err != nil || spwd != "S" {
		t.Fatalf("spwd %q err %v", spwd, err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"/v2/file/recycle/config/save", "my_cycle=7", "public_cycle=30",
		"/zdrive/kuake/task/download", "folder_ids=2",
		"/zdrive/kuake/file/filelist", "parent_file_id=0", "show_dir=1",
		"/znetdisk/task/action", "method=pause_all",
		"/znetdisk/share/verify", "short_url=abc",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("缺少 %s\n%s", want, joined)
		}
	}
}
