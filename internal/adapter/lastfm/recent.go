package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/emount4/lastfm/internal/domain"
)

// CheckScrobbleAccess проверяет, доступна ли история прослушиваний. Пустая история допустима.
func (c *Client) CheckScrobbleAccess(ctx context.Context, username string) error {
	if _, err := c.recentTracks(ctx, username); err != nil {
		return fmt.Errorf("check scrobble access: %w", err)
	}
	return nil
}

// NowPlaying возвращает трек, который Last.fm пометил как текущий.
func (c *Client) NowPlaying(ctx context.Context, username string) (domain.NowPlayingTrack, bool, error) {
	tracks, err := c.recentTracks(ctx, username)
	if err != nil {
		return domain.NowPlayingTrack{}, false, fmt.Errorf("get currently playing track: %w", err)
	}
	for _, raw := range tracks {
		var track recentTrack
		if err := json.Unmarshal(raw, &track); err != nil {
			return domain.NowPlayingTrack{}, false, fmt.Errorf("decode recent track: %w", err)
		}
		if track.Attrs.NowPlaying != "true" {
			continue
		}
		artist, title := strings.TrimSpace(track.Artist.Name), strings.TrimSpace(track.Name)
		if artist == "" || title == "" {
			return domain.NowPlayingTrack{}, false, errors.New("currently playing track lacks an artist or title")
		}
		return domain.NowPlayingTrack{
			Artist:   artist,
			Title:    title,
			Album:    strings.TrimSpace(track.Album.Name),
			CoverURL: largestCover(track.Images),
		}, true, nil
	}
	return domain.NowPlayingTrack{}, false, nil
}

func (c *Client) recentTracks(ctx context.Context, username string) ([]json.RawMessage, error) {
	const method = "user.getRecentTracks"

	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("lastfm user.getRecentTracks: username is required")
	}

	var response recentTracksResponse
	if err := c.get(ctx, method, url.Values{"user": {username}, "limit": {"1"}}, &response); err != nil {
		return nil, err
	}
	if response.RecentTracks.Tracks == nil {
		return nil, errors.New("lastfm user.getRecentTracks: response did not include a track list")
	}
	return response.RecentTracks.Tracks, nil
}

func largestCover(images []trackImage) string {
	for _, size := range [...]string{"mega", "extralarge", "large", "medium", "small"} {
		for _, image := range images {
			if image.Size == size && image.URL != "" {
				return image.URL
			}
		}
	}
	return ""
}
