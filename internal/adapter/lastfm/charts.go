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

type chartItem struct {
	Name      string      `json:"name"`
	PlayCount json.Number `json:"playcount"`
	Artist    struct {
		Name string `json:"name"`
	} `json:"artist"`
}

type chartResponse struct {
	Artists    []chartItem `json:"artist"`
	Tracks     []chartItem `json:"track"`
	Albums     []chartItem `json:"album"`
	Pagination pagination  `json:"@attr"`
}

func (c *Client) ChartPage(ctx context.Context, username string, kind domain.MusicKind, page int) ([]domain.ChartEntry, bool, error) {
	var method, root string
	switch kind {
	case domain.Artist:
		method, root = "user.getTopArtists", "topartists"
	case domain.Track:
		method, root = "user.getTopTracks", "toptracks"
	case domain.Album:
		method, root = "user.getTopAlbums", "topalbums"
	default:
		return nil, false, fmt.Errorf("unsupported chart kind %q", kind)
	}
	params := url.Values{
		"user": {username}, "period": {"overall"}, "page": {strconv.Itoa(page)}, "limit": {"500"},
	}
	var response map[string]chartResponse
	if err := c.get(ctx, method, params, &response); err != nil {
		return nil, false, fmt.Errorf("read %s chart page %d: %w", kind, page, err)
	}

	chart := response[root]
	items := chart.Artists
	switch kind {
	case domain.Track:
		items = chart.Tracks
	case domain.Album:
		items = chart.Albums
	}
	more, err := chart.Pagination.hasMore(page, len(items))
	if err != nil {
		return nil, false, fmt.Errorf("read %s chart pagination: %w", kind, err)
	}

	entries := make([]domain.ChartEntry, 0, len(items))
	for _, item := range items {
		count, err := item.PlayCount.Int64()
		if err != nil {
			return nil, false, fmt.Errorf("read %s play count on page %d: %w", kind, page, err)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" || count <= 0 {
			continue
		}
		entry := domain.ChartEntry{Artist: name, Scrobbles: count}
		if kind != domain.Artist {
			entry.Artist, entry.Title = strings.TrimSpace(item.Artist.Name), name
			if entry.Artist == "" {
				continue
			}
		}
		entries = append(entries, entry)
	}
	return entries, more, nil
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
