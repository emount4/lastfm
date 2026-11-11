package lastfm

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/emount4/lastfm/internal/domain"
)

func (c *Client) Search(ctx context.Context, kind domain.MusicKind, title string) ([]domain.MusicTarget, error) {
	if kind != domain.Track && kind != domain.Album {
		return nil, fmt.Errorf("search Last.fm: unsupported music kind %q", kind)
	}
	var response musicSearchResponse
	params := url.Values{string(kind): {title}, "limit": {"10"}}
	if err := c.get(ctx, string(kind)+".search", params, &response); err != nil {
		return nil, fmt.Errorf("search Last.fm: %w", err)
	}

	results := response.Results.TrackMatches.Tracks
	if kind == domain.Album {
		results = response.Results.AlbumMatches.Albums
	}
	var matches, exact []domain.MusicTarget
	seen := make(map[[2]string]struct{}, len(results))
	for _, result := range results {
		artist, name := strings.TrimSpace(result.Artist), strings.TrimSpace(result.Name)
		if artist == "" || name == "" {
			continue
		}
		key := [2]string{strings.ToLower(artist), strings.ToLower(name)}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		target := domain.MusicTarget{Kind: kind, Artist: artist, Title: name}
		matches = append(matches, target)
		if strings.EqualFold(name, title) {
			exact = append(exact, target)
		}
	}
	if len(exact) != 0 {
		matches = exact
	}
	return matches[:min(len(matches), 5)], nil
}
