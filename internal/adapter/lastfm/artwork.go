package lastfm

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) AlbumCover(ctx context.Context, artist, album string) (string, error) {
	var response struct {
		Album struct {
			Images []trackImage `json:"image"`
		} `json:"album"`
	}
	params := url.Values{"artist": {artist}, "album": {album}}
	if err := c.get(ctx, "album.getInfo", params, &response); err != nil {
		return "", fmt.Errorf("get album cover: %w", err)
	}
	return largestCover(response.Album.Images), nil
}
