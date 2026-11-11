package lastfm

import (
	"context"
	"fmt"
	"net/url"

	"github.com/emount4/lastfm/internal/domain"
)

func (c *Client) PlayCount(ctx context.Context, username string, target domain.MusicTarget) (int64, error) {
	var response playCountResponse
	params := url.Values{"username": {username}, "artist": {target.Artist}, "autocorrect": {"1"}}
	var personal *personalPlayCount
	switch target.Kind {
	case domain.Artist:
		personal = &response.Artist.Stats
	case domain.Track:
		params.Set("track", target.Title)
		personal = &response.Track
	case domain.Album:
		params.Set("album", target.Title)
		personal = &response.Album
	default:
		return 0, fmt.Errorf("read play count: unknown music kind %q", target.Kind)
	}
	if err := c.get(ctx, string(target.Kind)+".getInfo", params, &response); err != nil {
		return 0, fmt.Errorf("read play count: %w", err)
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
