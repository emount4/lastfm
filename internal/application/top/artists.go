package top

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type ParticipantReader interface {
	List(context.Context) ([]domain.Participant, error)
}

type ArtistChartReader interface {
	TopArtistsPage(ctx context.Context, username string, page int) (entries []domain.ArtistScrobbles, more bool, err error)
}

type ScrobbleReader interface {
	ScrobblesPage(ctx context.Context, username string, interval domain.ScrobbleRange, page int) (entries []domain.Scrobble, more bool, err error)
}

type ArtistRanking struct {
	Entries          []domain.ArtistScrobbles
	ProfilesIncluded int
	ProfilesTotal    int
}

type Service struct {
	participants ParticipantReader
	charts       ArtistChartReader
	scrobbles    ScrobbleReader
	now          func() time.Time
}

func NewService(participants ParticipantReader, charts ArtistChartReader, scrobbles ScrobbleReader, now func() time.Time) *Service {
	return &Service{participants: participants, charts: charts, scrobbles: scrobbles, now: now}
}

// Artists суммирует скробблы участников за выбранный период и возвращает десять лидеров.
func (s *Service) Artists(ctx context.Context, period domain.Period) (ArtistRanking, error) {
	var readPage func(context.Context, string, int) ([]domain.ArtistScrobbles, bool, error)
	if period.AllTime() {
		readPage = s.charts.TopArtistsPage
	} else {
		interval, err := period.Range(s.now())
		if err != nil {
			return ArtistRanking{}, fmt.Errorf("calculate artist ranking period: %w", err)
		}
		readPage = func(ctx context.Context, username string, page int) ([]domain.ArtistScrobbles, bool, error) {
			scrobbles, more, err := s.scrobbles.ScrobblesPage(ctx, username, interval, page)
			if err != nil {
				return nil, false, fmt.Errorf("read artist scrobbles: %w", err)
			}
			entries := make([]domain.ArtistScrobbles, 0, len(scrobbles))
			for _, scrobble := range scrobbles {
				entries = append(entries, domain.ArtistScrobbles{Name: scrobble.Artist, Scrobbles: 1})
			}
			return entries, more, nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	participants, err := s.participants.List(ctx)
	if err != nil {
		return ArtistRanking{}, fmt.Errorf("list participants for artist chart: %w", err)
	}

	type profileChart struct {
		entries map[string]domain.ArtistScrobbles
		err     error
	}
	profiles := make([]profileChart, len(participants))
	slots := make(chan struct{}, 3)
	var requests sync.WaitGroup
	for index, participant := range participants {
		requests.Go(func() {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				profiles[index].err = ctx.Err()
				return
			}
			profiles[index].entries, profiles[index].err = readArtistPages(ctx, participant.LastFMUsername, readPage)
		})
	}
	requests.Wait()

	ranking := ArtistRanking{ProfilesTotal: len(participants)}
	totals := make(map[string]domain.ArtistScrobbles)
	var profileErrors []error
	for _, profile := range profiles {
		if profile.err != nil {
			profileErrors = append(profileErrors, profile.err)
			continue
		}
		ranking.ProfilesIncluded++
		for _, entry := range profile.entries {
			addArtistScrobbles(totals, entry)
		}
	}
	if len(profileErrors) > 0 && ranking.ProfilesIncluded == 0 {
		return ArtistRanking{}, fmt.Errorf("load artist charts: %w", errors.Join(profileErrors...))
	}

	for _, total := range totals {
		ranking.Entries = append(ranking.Entries, total)
	}
	slices.SortFunc(ranking.Entries, func(a, b domain.ArtistScrobbles) int {
		if byCount := cmp.Compare(b.Scrobbles, a.Scrobbles); byCount != 0 {
			return byCount
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	ranking.Entries = ranking.Entries[:min(10, len(ranking.Entries))]
	return ranking, nil
}

func readArtistPages(ctx context.Context, username string, readPage func(context.Context, string, int) ([]domain.ArtistScrobbles, bool, error)) (map[string]domain.ArtistScrobbles, error) {
	chart := make(map[string]domain.ArtistScrobbles)
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("read artist chart: %w", err)
		}
		entries, more, err := readPage(ctx, username, page)
		if err != nil {
			return nil, fmt.Errorf("load artist chart page %d: %w", page, err)
		}
		for _, entry := range entries {
			addArtistScrobbles(chart, entry)
		}
		if !more {
			return chart, nil
		}
	}
}

func addArtistScrobbles(totals map[string]domain.ArtistScrobbles, entry domain.ArtistScrobbles) {
	key := strings.ToLower(entry.Name)
	total, exists := totals[key]
	if !exists {
		total.Name = entry.Name
	}
	total.Scrobbles += entry.Scrobbles
	totals[key] = total
}
