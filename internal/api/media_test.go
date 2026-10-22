package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVideoAndMusicRequests(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.Path+" "+string(b)+" "+r.URL.RawQuery)
		payload := `{"code":"200","msg":"200","data":{"count":2,"list":[{"song_name":"一首歌","artist":"甲"},{"song_name":"其他","artist":"乙"}]}}`
		if !strings.Contains(r.URL.Path, "/zmusic/api/v2/song/list") {
			payload = `{"code":"200","msg":"200","data":{"count":0}}`
		}
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Creds: Credentials{Token: "T", NasID: "N", DeviceID: "D"}, HTTP: srv.Client()}
	ctx := context.Background()

	if _, err := c.VideoList(ctx, "cls", 1, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := c.VideoSearch(ctx, "电影", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.VideoLatest(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.MusicAlbums(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := c.MusicSongs(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	found, err := c.MusicSearch(ctx, "歌", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(found), `"count":1`) {
		t.Fatalf("搜索应只留下歌名匹配的一首：%s", found)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"POST /zvideo/video/v2/list", "classification_id=cls", "start=1", "limit=20",
		"POST /zvideo/search/local", "key_word=",
		"POST /zvideo/home/collection/latest", "limit=30",
		"GET /zmusic/api/v2/album/list", "token=T", "nasid=N",
		"POST /zmusic/api/v2/song/list", "limit=100",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("缺少 %s\n%s", want, joined)
		}
	}
}
