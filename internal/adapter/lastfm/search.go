package lastfm

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/emount4/lastfm/internal/domain"
)

func (c *Client) Search(ctx context.Context, kind domain.MusicKind, title string) ([]domain.MusicTarget, error) {
	if kind != domain.Track && kind != domain.Album {
		return nil, fmt.Errorf("search Last.fm: unsupported music kind %q", kind)
	}
	results, err := c.searchMatches(ctx, kind, title, "")
	if err != nil {
		return nil, fmt.Errorf("search Last.fm: %w", err)
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

func (c *Client) searchMatches(ctx context.Context, kind domain.MusicKind, title, artist string) ([]musicSearchMatch, error) {
	var response musicSearchResponse
	params := url.Values{string(kind): {title}, "limit": {"10"}}
	if artist != "" {
		params.Set("artist", artist)
	}
	if err := c.get(ctx, string(kind)+".search", params, &response); err != nil {
		return nil, fmt.Errorf("read search matches: %w", err)
	}
	if kind == domain.Album {
		return response.Results.AlbumMatches.Albums, nil
	}
	return response.Results.TrackMatches.Tracks, nil
}

func (c *Client) FindTitleVariants(ctx context.Context, target domain.MusicTarget) ([]domain.MusicTarget, error) {
	base := titleWithoutTrailingPunctuation(target.Title)
	if base == "" {
		return nil, nil
	}
	var results []musicSearchMatch
	switch target.Kind {
	case domain.Track:
		matches, err := c.searchMatches(ctx, domain.Track, base, target.Artist)
		if err != nil {
			return nil, fmt.Errorf("find track title variants: %w", err)
		}
		results = matches
	case domain.Album:
		var response artistTopAlbumsResponse
		params := url.Values{"artist": {target.Artist}, "autocorrect": {"1"}, "limit": {"50"}}
		if err := c.get(ctx, "artist.getTopAlbums", params, &response); err != nil {
			return nil, fmt.Errorf("find album title variants: %w", err)
		}
		for _, album := range response.TopAlbums.Albums {
			results = append(results, musicSearchMatch{Name: album.Name, Artist: album.Artist.Name})
		}
	}

	var variants []domain.MusicTarget
	seen := make(map[string]struct{})
	for _, result := range results {
		name, artist := strings.TrimSpace(result.Name), strings.TrimSpace(result.Artist)
		if !strings.EqualFold(artist, target.Artist) || strings.EqualFold(name, target.Title) ||
			!strings.EqualFold(titleWithoutTrailingPunctuation(name), base) {
			continue
		}
		key := strings.ToLower(name)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		variants = append(variants, domain.MusicTarget{Kind: target.Kind, Artist: artist, Title: name})
		if len(variants) == 5 {
			break
		}
	}
	return variants, nil
}

func titleWithoutTrailingPunctuation(title string) string {
	return strings.TrimRightFunc(title, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
}
