package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAlbumRequestsCarryApp(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.URL.Path+" "+string(b))
		_, _ = w.Write([]byte(`{"code":"200","msg":"200","data":{"total":0,"list":[]}}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	ctx := context.Background()

	if _, err := c.AlbumHome(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Photos(ctx, 0, 20, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AlbumFeeds(ctx, "42", 0, 10); err != nil {
		t.Fatal(err)
	}
	if err := c.PhotoLike(ctx, "7", true); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"/v2/album/home",
		"app=album",
		"/v2/album/feed/list",
		"liked=1",
		"/v2/album/album/feeds",
		"album_id=42",
		"/v2/album/feed/like",
		"feed_id=7",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("缺少 %s\n%s", want, joined)
		}
	}
}

func TestAlbumCount(t *testing.T) {
	n, err := AlbumCount([]byte(`{"total":3,"list":[]}`))
	if err != nil || n != 3 {
		t.Fatalf("count %d err %v", n, err)
	}
	n, err = AlbumCount([]byte(`null`))
	if err != nil || n != 0 {
		t.Fatalf("null count %d err %v", n, err)
	}
}

func TestPhotoThumbRejectsJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcode/thumb" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("dest_fmt") != "large" {
			t.Errorf("dest_fmt %s", r.URL.Query().Get("dest_fmt"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"N001315","msg":"文件不存在"}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T"}, HTTP: srv.Client()}
	err := c.PhotoThumb(context.Background(), "/sata1/my/data/a.jpg", t.TempDir()+"/a.jpg", "large")
	ze, ok := err.(*Error)
	if !ok || ze.Code != "N001315" {
		t.Fatalf("err %#v", err)
	}
}
