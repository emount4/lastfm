package lastfm

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) ArtistPlayCount(ctx context.Context, username, artist string) (int64, error) {
	var response artistInfoResponse
	params := url.Values{"username": {username}, "artist": {artist}, "autocorrect": {"1"}}
	if err := c.get(ctx, "artist.getInfo", params, &response); err != nil {
		return 0, fmt.Errorf("read artist play count: %w", err)
	}
	value := response.Artist.Stats.UserPlayCount
	if value == "" {
		return 0, nil
	}
	count, err := value.Int64()
	if err != nil {
		return 0, fmt.Errorf("parse artist play count: %w", err)
	}
	return count, nil
}
