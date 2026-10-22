package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

// albumForm 给相册接口补上 app=album。网页版每个相册请求都带这个字段。
func albumForm(extra map[string]string) map[string]string {
	out := map[string]string{"app": "album"}
	for k, v := range extra {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func (c *Client) album(ctx context.Context, endpoint string, extra map[string]string) (json.RawMessage, error) {
	env, err := c.Form(ctx, endpoint, albumForm(extra), nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// AlbumHome 是相册首页：我的相册、系统目录、喜欢、其他人的相册。
// 没有照片时各分组的 list 为空、total 为 0。
func (c *Client) AlbumHome(ctx context.Context) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/home", nil)
}

// Albums 列出相册。start、num 控制分页。
func (c *Client) Albums(ctx context.Context, start, num int) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/albums", map[string]string{
		"start": strconv.Itoa(start), "num": strconv.Itoa(num),
	})
}

// AlbumInfo 取一个相册的详情。
func (c *Client) AlbumInfo(ctx context.Context, albumID string) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/album/info", map[string]string{"album_id": albumID})
}

// AlbumFeeds 列出一个相册里的照片。
func (c *Client) AlbumFeeds(ctx context.Context, albumID string, start, num int) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/album/feeds", map[string]string{
		"album_id": albumID, "start": strconv.Itoa(start), "num": strconv.Itoa(num),
	})
}

// AlbumDirs 列出相册扫描的目录。
func (c *Client) AlbumDirs(ctx context.Context) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/dirs", nil)
}

// PhotoTimeline 返回时间线分组。data 含 total、list。
func (c *Client) PhotoTimeline(ctx context.Context) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/feed/timeline", nil)
}

// Photos 列出全部照片和视频。liked 为真时只返回喜欢的。
func (c *Client) Photos(ctx context.Context, start, num int, liked bool) (json.RawMessage, error) {
	extra := map[string]string{"start": strconv.Itoa(start), "num": strconv.Itoa(num)}
	if liked {
		extra["liked"] = "1"
	}
	return c.album(ctx, "/v2/album/feed/list", extra)
}

// PhotoLike 设置一张照片的喜欢状态。feedID 是照片 id。
func (c *Client) PhotoLike(ctx context.Context, feedID string, liked bool) error {
	flag := "0"
	if liked {
		flag = "1"
	}
	_, err := c.album(ctx, "/v2/album/feed/like", map[string]string{
		"feed_id": feedID, "liked": flag,
	})
	return err
}

// PhotoPickTypes 返回 AI 挑选分类。每项含 type 和 name。
func (c *Client) PhotoPickTypes(ctx context.Context) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/ai/picking/menu_bar", nil)
}

// PhotoPicks 返回一个挑选分类下的结果。
// type 来自 PhotoPickTypes，社区客户端观察到的取值是 9、1、10、11。
func (c *Client) PhotoPicks(ctx context.Context, pickType, start, num int, order string) (json.RawMessage, error) {
	if order == "" {
		order = "desc"
	}
	return c.album(ctx, "/v2/album/ai/picking/result", map[string]string{
		"selection_type": strconv.Itoa(pickType), "threshold_score": "0",
		"start": strconv.Itoa(start), "num": strconv.Itoa(num), "order": order,
	})
}

// AIState 返回人脸、场景、视频的识别进度。
func (c *Client) AIState(ctx context.Context) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/ai/state", nil)
}

// CreateAlbum 新建相册。name 必填，desc 可空。返回 data。
func (c *Client) CreateAlbum(ctx context.Context, name, desc string) (json.RawMessage, error) {
	return c.album(ctx, "/v2/album/album/create", map[string]string{
		"name": name, "desc": desc,
	})
}

// DeleteAlbum 删除相册。
func (c *Client) DeleteAlbum(ctx context.Context, albumID string) error {
	_, err := c.album(ctx, "/v2/album/album/delete", map[string]string{"album_id": albumID})
	return err
}

// PhotoThumb 把一张照片的缩略图保存到本地。
// size 取 small 或 large，与网页版 dest_fmt 一致。
func (c *Client) PhotoThumb(ctx context.Context, remotePath, localPath, size string) error {
	if size == "" {
		size = "small"
	}
	vals := url.Values{}
	vals.Set("file_path", remotePath)
	vals.Set("dest_fmt", size)
	vals.Set("request_purpose", "5")
	vals.Set("rnd", rnd())
	vals.Set("webagent", "v2")
	u := c.BaseURL + "/transcode/thumb?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", c.cookieHeader())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("缩略图请求失败：HTTP %d", resp.StatusCode)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var env Envelope
		if json.Unmarshal(raw, &env) == nil && !env.OK() {
			return &Error{Code: env.CodeString(), Msg: env.Msg}
		}
		return fmt.Errorf("缩略图失败：%s", truncate(raw, 200))
	}
	if err := os.MkdirAll(path.Dir(localPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(localPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// AlbumCount 从列表响应里取 total。接口把 total 返回成数字。
func AlbumCount(raw json.RawMessage) (int, error) {
	var data struct {
		Total int `json:"total"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return 0, err
	}
	return data.Total, nil
}
