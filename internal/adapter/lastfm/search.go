package lastfm

import (
	"context"
	"errors"
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
		if matchesSpelling(title, name) {
			exact = append(exact, target)
		}
	}
	if len(exact) != 0 {
		matches = exact
	} else if strings.ContainsAny(title, "ёЁ") {
		return nil, nil
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

func (c *Client) FindVariants(ctx context.Context, target domain.MusicTarget) ([]domain.MusicTarget, error) {
	if target.Kind == domain.Artist {
		return c.findArtistVariants(ctx, target.Artist)
	}
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
			var failure *apiError
			if !strings.ContainsAny(target.Artist+target.Title, "еЕ") ||
				!errors.As(err, &failure) || (failure.Code != 6 && failure.Code != 7) {
				return nil, fmt.Errorf("find album title variants: %w", err)
			}
		}
		for _, album := range response.TopAlbums.Albums {
			results = append(results, musicSearchMatch{Name: album.Name, Artist: album.Artist.Name})
		}
	}

	variants := titleVariants(results, target, base)
	if len(variants) != 0 || !strings.ContainsAny(target.Artist+target.Title, "еЕ") {
		return variants, nil
	}
	matches, err := c.searchMatches(ctx, target.Kind, base, "")
	if err != nil {
		return nil, fmt.Errorf("find %s spelling variants: %w", target.Kind, err)
	}
	return titleVariants(matches, target, base), nil
}

func titleVariants(results []musicSearchMatch, target domain.MusicTarget, base string) []domain.MusicTarget {
	var variants []domain.MusicTarget
	seen := make(map[[2]string]struct{})
	for _, result := range results {
		name, artist := strings.TrimSpace(result.Name), strings.TrimSpace(result.Artist)
		if !matchesSpelling(target.Artist, artist) ||
			(strings.EqualFold(artist, target.Artist) && strings.EqualFold(name, target.Title)) ||
			!matchesSpelling(base, titleWithoutTrailingPunctuation(name)) {
			continue
		}
		key := [2]string{strings.ToLower(artist), strings.ToLower(name)}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		variants = append(variants, domain.MusicTarget{Kind: target.Kind, Artist: artist, Title: name})
		if len(variants) == 5 {
			break
		}
	}
	return variants
}

func (c *Client) findArtistVariants(ctx context.Context, artist string) ([]domain.MusicTarget, error) {
	var response struct {
		Results struct {
			Matches struct {
				Artists []struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"artistmatches"`
		} `json:"results"`
	}
	if err := c.get(ctx, "artist.search", url.Values{"artist": {artist}, "limit": {"10"}}, &response); err != nil {
		return nil, fmt.Errorf("find artist spelling variants: %w", err)
	}
	var variants []domain.MusicTarget
	seen := make(map[string]struct{})
	for _, candidate := range response.Results.Matches.Artists {
		name := strings.TrimSpace(candidate.Name)
		if !matchesSpelling(artist, name) || strings.EqualFold(artist, name) {
			continue
		}
		key := strings.ToLower(name)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		variants = append(variants, domain.MusicTarget{Kind: domain.Artist, Artist: name})
		if len(variants) == 5 {
			break
		}
	}
	return variants, nil
}

func matchesSpelling(query, name string) bool {
	if strings.EqualFold(query, name) {
		return true
	}
	queryLetters := []rune(strings.ToLower(query))
	nameLetters := []rune(strings.ToLower(name))
	if len(queryLetters) != len(nameLetters) {
		return false
	}
	for index, letter := range queryLetters {
		if letter != nameLetters[index] && !(letter == 'е' && nameLetters[index] == 'ё') {
			return false
		}
	}
	return true
}

func titleWithoutTrailingPunctuation(title string) string {
	return strings.TrimRightFunc(title, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
}
