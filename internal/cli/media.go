package cli

import (
	"context"
	"errors"

	"github.com/vangie/zspace-cli/internal/api"
)

func pageStart(o opts) int {
	if o.startAt > 0 {
		return o.startAt
	}
	if o.page > 1 {
		return o.page
	}
	return 1
}

func cmdVideo(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	limit := o.limit
	if limit <= 0 {
		limit = 30
	}
	start := pageStart(o)
	switch sub {
	case "status":
		return call(c.VideoStatus(ctx))
	case "classes":
		return call(c.VideoClassifications(ctx))
	case "home":
		latest, err := c.VideoLatest(ctx, limit)
		if err != nil {
			return nil, 1, err
		}
		suggested, err := c.VideoSuggested(ctx, limit)
		if err != nil {
			return nil, 1, err
		}
		return map[string]any{"latest": latest, "suggested": suggested}, 0, nil
	case "list":
		classID := o.classID
		if classID == "" && len(args) > 0 {
			classID = args[0]
		}
		return call(c.VideoList(ctx, classID, start, limit))
	case "info":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace video info <影视ID>")
		}
		return call(c.VideoInfo(ctx, args[0]))
	case "search":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace video search <关键词>")
		}
		return call(c.VideoSearch(ctx, args[0], o.classID))
	case "records":
		return call(c.VideoRecords(ctx, start, limit))
	case "favorites":
		return call(c.VideoFavorites(ctx, start, limit))
	case "series":
		classID := o.classID
		if classID == "" && len(args) > 0 {
			classID = args[0]
		}
		return call(c.VideoSeries(ctx, classID))
	case "settings":
		return call(c.VideoSettings(ctx))
	default:
		return nil, 2, errors.New("用法：zspace video <status|classes|home|list|info|search|records|favorites|series|settings>")
	}
}

func cmdMusic(ctx context.Context, c *api.Client, args []string, o opts) (any, int, error) {
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	limit := o.limit
	if limit <= 0 {
		limit = 30
	}
	start := pageStart(o)
	switch sub {
	case "status":
		return call(c.MusicStatus(ctx))
	case "songs":
		return call(c.MusicSongs(ctx, start, limit))
	case "albums":
		return call(c.MusicAlbums(ctx, start, limit))
	case "artists":
		return call(c.MusicArtists(ctx, start, limit))
	case "playlists":
		return call(c.MusicPlaylists(ctx))
	case "latest":
		return call(c.MusicLatest(ctx))
	case "history":
		return call(c.MusicHistory(ctx, start, limit))
	case "favorites":
		return call(c.MusicFavorites(ctx, start, limit))
	case "picks":
		return call(c.MusicPicks(ctx))
	case "info":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace music info <歌曲ID>")
		}
		return call(c.MusicSong(ctx, args[0]))
	case "lyrics":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace music lyrics <歌曲ID>")
		}
		return call(c.MusicLyrics(ctx, args[0]))
	case "search":
		if len(args) < 1 {
			return nil, 2, errors.New("用法：zspace music search <歌名>")
		}
		return call(c.MusicSearch(ctx, args[0], limit))
	default:
		return nil, 2, errors.New("用法：zspace music <status|songs|albums|artists|playlists|latest|history|favorites|picks|info|lyrics|search>")
	}
}
