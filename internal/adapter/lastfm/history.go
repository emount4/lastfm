package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/emount4/lastfm/internal/domain"
)

func (c *Client) ScrobblesPage(ctx context.Context, username string, interval domain.ScrobbleRange, page int) ([]domain.Scrobble, bool, error) {
	// Last.fm описывает from как исключающую границу. Сдвигаем её на секунду и проверяем диапазон после чтения.
	params := url.Values{
		"user": {username}, "page": {strconv.Itoa(page)}, "limit": {"200"},
		"from": {strconv.FormatInt(max(0, interval.From-1), 10)}, "to": {strconv.FormatInt(interval.To, 10)},
	}
	var response recentTracksResponse
	if err := c.get(ctx, "user.getRecentTracks", params, &response); err != nil {
		return nil, false, fmt.Errorf("read scrobbles page %d: %w", page, err)
	}
	history := response.RecentTracks
	more, err := history.Pagination.hasMore(page, len(history.Tracks))
	if err != nil {
		return nil, false, fmt.Errorf("read scrobbles pagination: %w", err)
	}

	scrobbles := make([]domain.Scrobble, 0, len(history.Tracks))
	for _, raw := range history.Tracks {
		var track recentTrack
		if err := json.Unmarshal(raw, &track); err != nil {
			return nil, false, fmt.Errorf("decode scrobble on page %d: %w", page, err)
		}
		if track.Attrs.NowPlaying == "true" || track.Date.Timestamp == "" {
			continue
		}
		timestamp, err := track.Date.Timestamp.Int64()
		if err != nil {
			return nil, false, fmt.Errorf("read scrobble timestamp on page %d: %w", page, err)
		}
		artist := strings.TrimSpace(track.Artist.Name)
		if artist == "" || timestamp < interval.From || timestamp >= interval.To {
			continue
		}
		scrobbles = append(scrobbles, domain.Scrobble{
			Artist: artist, Track: strings.TrimSpace(track.Name), Album: strings.TrimSpace(track.Album.Name),
		})
	}
	return scrobbles, more, nil
}
