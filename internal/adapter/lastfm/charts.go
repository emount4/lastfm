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

type artistChartResponse struct {
	TopArtists struct {
		Artists []struct {
			Name      string      `json:"name"`
			PlayCount json.Number `json:"playcount"`
		} `json:"artist"`
		Pagination pagination `json:"@attr"`
	} `json:"topartists"`
}

func (c *Client) TopArtistsPage(ctx context.Context, username string, page int) ([]domain.ArtistScrobbles, bool, error) {
	params := url.Values{
		"user": {username}, "period": {"overall"}, "page": {strconv.Itoa(page)}, "limit": {"500"},
	}
	var response artistChartResponse
	if err := c.get(ctx, "user.getTopArtists", params, &response); err != nil {
		return nil, false, fmt.Errorf("read artist chart page %d: %w", page, err)
	}

	chart := response.TopArtists
	more, err := chart.Pagination.hasMore(page, len(chart.Artists))
	if err != nil {
		return nil, false, fmt.Errorf("read artist chart pagination: %w", err)
	}

	artists := make([]domain.ArtistScrobbles, 0, len(chart.Artists))
	for _, artist := range chart.Artists {
		count, err := artist.PlayCount.Int64()
		if err != nil {
			return nil, false, fmt.Errorf("read artist play count on page %d: %w", page, err)
		}
		if name := strings.TrimSpace(artist.Name); name != "" && count > 0 {
			artists = append(artists, domain.ArtistScrobbles{Name: name, Scrobbles: count})
		}
	}
	return artists, more, nil
}

func (p pagination) hasMore(page, entries int) (bool, error) {
	current, err := p.Page.Int64()
	if err != nil {
		return false, fmt.Errorf("read page number: %w", err)
	}
	total, err := p.TotalPages.Int64()
	if err != nil {
		return false, fmt.Errorf("read total pages: %w", err)
	}
	if current != int64(page) || total < 0 || (current < total && entries == 0) {
		return false, fmt.Errorf("invalid pagination for page %d", page)
	}
	return current < total, nil
}
