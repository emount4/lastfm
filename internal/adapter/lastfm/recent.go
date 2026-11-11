package lastfm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// CheckScrobbleAccess проверяет, доступна ли история прослушиваний. Пустая история допустима.
func (c *Client) CheckScrobbleAccess(ctx context.Context, username string) error {
	const method = "user.getRecentTracks"

	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("lastfm user.getRecentTracks: username is required")
	}

	var response recentTracksResponse
	if err := c.get(ctx, method, url.Values{"user": {username}, "limit": {"1"}}, &response); err != nil {
		return fmt.Errorf("check scrobble access: %w", err)
	}
	if response.RecentTracks.Tracks == nil {
		return errors.New("lastfm user.getRecentTracks: response did not include a track list")
	}
	return nil
}
