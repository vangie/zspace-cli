package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// 极影视走 /zvideo，极音乐走 /zmusic/api/v2。
// 网页客户端里，影视请求大多是表单 POST，音乐列表大多是带登录字段的 GET。
// 歌曲列表是例外，它用 POST。

func (c *Client) video(ctx context.Context, endpoint string, extra map[string]string) (json.RawMessage, error) {
	env, err := c.Form(ctx, endpoint, extra, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

func (c *Client) mediaGet(ctx context.Context, endpoint string, extra map[string]string) (json.RawMessage, error) {
	env, err := c.Get(ctx, endpoint, extra)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

func pageFields(start, limit int) map[string]string {
	if start <= 0 {
		start = 1
	}
	if limit <= 0 {
		limit = 30
	}
	return map[string]string{"start": strconv.Itoa(start), "limit": strconv.Itoa(limit)}
}

// VideoStatus 返回极影视服务状态。这个接口是 GET，POST 会 404。
func (c *Client) VideoStatus(ctx context.Context) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zvideo/services/status", nil)
}

// VideoClassifications 返回片库分类。网页侧边栏用这个列表。
func (c *Client) VideoClassifications(ctx context.Context) (json.RawMessage, error) {
	return c.video(ctx, "/zvideo/classification/list", nil)
}

// VideoLatest 返回首页最近添加，默认 30 条。
func (c *Client) VideoLatest(ctx context.Context, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 30
	}
	return c.video(ctx, "/zvideo/home/collection/latest", map[string]string{"limit": strconv.Itoa(limit)})
}

// VideoSuggested 返回首页推荐。backdrop=1 与网页首页一致。
func (c *Client) VideoSuggested(ctx context.Context, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 31
	}
	return c.video(ctx, "/zvideo/home/collection/suggested", map[string]string{
		"limit": strconv.Itoa(limit), "backdrop": "1",
	})
}

// VideoList 列出一个分类下的影视。classificationID 为空时列出最近添加。
// start 从 1 开始，与网页分页一致。
func (c *Client) VideoList(ctx context.Context, classificationID string, start, limit int) (json.RawMessage, error) {
	extra := pageFields(start, limit)
	if classificationID == "" {
		extra["system"] = "0"
		return c.video(ctx, "/zvideo/video/v2/lately", extra)
	}
	extra["classification_id"] = classificationID
	extra["system"] = "0"
	extra["field"] = "release_year"
	extra["sort"] = "desc"
	extra["seen"] = "0"
	extra["collection_type"] = "0"
	return c.video(ctx, "/zvideo/video/v2/list", extra)
}

// VideoInfo 取一部影视的详情。
func (c *Client) VideoInfo(ctx context.Context, collectionID string) (json.RawMessage, error) {
	return c.video(ctx, "/zvideo/collection/info", map[string]string{"collection_id": collectionID})
}

// VideoSearch 在本地片库搜索。key_word 是网页用的字段名。
func (c *Client) VideoSearch(ctx context.Context, keyword, classificationID string) (json.RawMessage, error) {
	extra := map[string]string{"key_word": keyword}
	if classificationID != "" {
		extra["classification_id"] = classificationID
	}
	return c.video(ctx, "/zvideo/search/local", extra)
}

// VideoRecords 返回播放记录。网页侧边栏走 /zvideo/video/v2/playlist。
func (c *Client) VideoRecords(ctx context.Context, start, limit int) (json.RawMessage, error) {
	return c.video(ctx, "/zvideo/video/v2/playlist", pageFields(start, limit))
}

// VideoFavorites 返回收藏。
func (c *Client) VideoFavorites(ctx context.Context, start, limit int) (json.RawMessage, error) {
	return c.video(ctx, "/zvideo/favorite/list", pageFields(start, limit))
}

// VideoSeries 返回一个分类下的合集。classificationID 必填。
func (c *Client) VideoSeries(ctx context.Context, classificationID string) (json.RawMessage, error) {
	if classificationID == "" {
		return nil, fmt.Errorf("合集需要分类 ID")
	}
	extra := pageFields(1, 50)
	extra["classification_id"] = classificationID
	return c.video(ctx, "/zvideo/series/list", extra)
}

// VideoSettings 返回首页推荐开关。网页读的是 suggested/load。
func (c *Client) VideoSettings(ctx context.Context) (json.RawMessage, error) {
	return c.video(ctx, "/zvideo/settings/suggested/load", nil)
}

// MusicStatus 返回极音乐服务状态。
func (c *Client) MusicStatus(ctx context.Context) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/services/status", nil)
}

// MusicSongs 列出歌曲。网页对这个接口用 POST。
func (c *Client) MusicSongs(ctx context.Context, start, limit int) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/zmusic/api/v2/song/list", pageFields(start, limit), nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// MusicAlbums 列出专辑。
func (c *Client) MusicAlbums(ctx context.Context, start, limit int) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/album/list", pageFields(start, limit))
}

// MusicArtists 列出歌手。
func (c *Client) MusicArtists(ctx context.Context, start, limit int) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/artist/list", pageFields(start, limit))
}

// MusicPlaylists 列出歌单。
func (c *Client) MusicPlaylists(ctx context.Context) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/playlist/list", nil)
}

// MusicLatest 返回最近添加的歌曲。
func (c *Client) MusicLatest(ctx context.Context) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/song/latest", nil)
}

// MusicHistory 返回最近播放的歌曲。record/latest 在这台 NAS 上是空对象，
// 歌曲历史走 history/song。
func (c *Client) MusicHistory(ctx context.Context, start, limit int) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/history/song", pageFields(start, limit))
}

// MusicFavorites 返回收藏歌曲。
func (c *Client) MusicFavorites(ctx context.Context, start, limit int) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/favorite/song/list", pageFields(start, limit))
}

// MusicPicks 返回首页精选概览。
func (c *Client) MusicPicks(ctx context.Context) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/picks/overview", nil)
}

// MusicSong 返回一首歌的详情。
func (c *Client) MusicSong(ctx context.Context, songID string) (json.RawMessage, error) {
	return c.mediaGet(ctx, "/zmusic/api/v2/song/info", map[string]string{"song_id": songID})
}

// MusicLyrics 返回歌词。网页用 POST，字段是 song_id。
func (c *Client) MusicLyrics(ctx context.Context, songID string) (json.RawMessage, error) {
	env, err := c.Form(ctx, "/zmusic/api/v2/file/info/lyrics", map[string]string{"song_id": songID}, nil)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// MusicSearch 按歌名过滤曲库。歌曲列表接口不接受搜索词，这里分页取回后在本地匹配。
func (c *Client) MusicSearch(ctx context.Context, keyword string, limit int) (json.RawMessage, error) {
	if keyword == "" {
		return nil, fmt.Errorf("搜索词不能为空")
	}
	if limit <= 0 {
		limit = 30
	}
	needle := strings.ToLower(keyword)
	var matched []json.RawMessage
	start := 1
	for page := 0; page < 40 && len(matched) < limit; page++ {
		env, err := c.Form(ctx, "/zmusic/api/v2/song/list", map[string]string{
			"start": strconv.Itoa(start), "limit": "100",
		}, nil)
		if err != nil {
			return nil, err
		}
		var data struct {
			Count int               `json:"count"`
			List  []json.RawMessage `json:"list"`
		}
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, err
		}
		for _, row := range data.List {
			if songMatches(row, needle) {
				matched = append(matched, row)
				if len(matched) >= limit {
					break
				}
			}
		}
		start += len(data.List)
		if len(data.List) == 0 || start > data.Count {
			break
		}
	}
	out, err := json.Marshal(map[string]any{"count": len(matched), "list": matched})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func songMatches(row json.RawMessage, needle string) bool {
	var song struct {
		Name     string `json:"song_name"`
		Title    string `json:"title"`
		Artist   string `json:"artist"`
		Album    string `json:"album"`
		SongInfo struct {
			Name   string `json:"song_name"`
			Artist string `json:"artist"`
		} `json:"song_info"`
	}
	if err := json.Unmarshal(row, &song); err != nil {
		return false
	}
	for _, v := range []string{song.Name, song.Title, song.Artist, song.Album, song.SongInfo.Name, song.SongInfo.Artist} {
		if v != "" && strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	return false
}
