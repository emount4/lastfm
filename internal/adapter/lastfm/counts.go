package lastfm

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/emount4/lastfm/internal/domain"
)

func (c *Client) PlayCount(ctx context.Context, username string, target domain.MusicTarget) (int64, error) {
	var response playCountResponse
	params := url.Values{"username": {username}, "artist": {target.Artist}, "autocorrect": {"1"}}
	var personal *personalPlayCount
	var artist, title *string
	switch target.Kind {
	case domain.Artist:
		personal = &response.Artist.Stats
		artist = &response.Artist.Name
	case domain.Track:
		params.Set("track", target.Title)
		personal = &response.Track.personalPlayCount
		artist, title = &response.Track.Artist.Name, &response.Track.Name
	case domain.Album:
		params.Set("album", target.Title)
		personal = &response.Album.personalPlayCount
		artist, title = &response.Album.Artist, &response.Album.Name
	default:
		return 0, fmt.Errorf("read play count: unknown music kind %q", target.Kind)
	}
	if err := c.get(ctx, string(target.Kind)+".getInfo", params, &response); err != nil {
		return 0, fmt.Errorf("read play count: %w", err)
	}
	if (strings.ContainsAny(target.Artist, "ёЁ") && !matchesSpelling(target.Artist, strings.TrimSpace(*artist))) ||
		(title != nil && strings.ContainsAny(target.Title, "ёЁ") && !matchesSpelling(target.Title, strings.TrimSpace(*title))) {
		return 0, nil
	}
	value := personal.UserPlayCount
	if value == "" {
		return 0, nil
	}
	count, err := value.Int64()
	if err != nil {
		return 0, fmt.Errorf("parse %s play count: %w", target.Kind, err)
	}
	return count, nil
}
