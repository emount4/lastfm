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

const Limit = 25

type ParticipantReader interface {
	List(context.Context) ([]domain.Participant, error)
}

type ChartReader interface {
	ChartPage(ctx context.Context, username string, kind domain.MusicKind, page int) (entries []domain.ChartEntry, more bool, err error)
}

type ScrobbleReader interface {
	ScrobblesPage(ctx context.Context, username string, interval domain.ScrobbleRange, page int) (entries []domain.Scrobble, more bool, err error)
}

type Ranking struct {
	Entries          []domain.ChartEntry
	ProfilesIncluded int
	ProfilesTotal    int
}

type Service struct {
	participants ParticipantReader
	charts       ChartReader
	scrobbles    ScrobbleReader
	now          func() time.Time
}

func NewService(participants ParticipantReader, charts ChartReader, scrobbles ScrobbleReader, now func() time.Time) *Service {
	return &Service{participants: participants, charts: charts, scrobbles: scrobbles, now: now}
}

// Rank суммирует скробблы участников за выбранный период и возвращает до 25 лидеров.
func (s *Service) Rank(ctx context.Context, kind domain.MusicKind, period domain.Period) (Ranking, error) {
	var readPage pageReader
	if period.AllTime() {
		readPage = func(ctx context.Context, username string, page int) ([]domain.ChartEntry, bool, error) {
			return s.charts.ChartPage(ctx, username, kind, page)
		}
	} else {
		interval, err := period.Range(s.now())
		if err != nil {
			return Ranking{}, fmt.Errorf("calculate ranking period: %w", err)
		}
		readPage = func(ctx context.Context, username string, page int) ([]domain.ChartEntry, bool, error) {
			scrobbles, more, err := s.scrobbles.ScrobblesPage(ctx, username, interval, page)
			if err != nil {
				return nil, false, fmt.Errorf("read scrobbles: %w", err)
			}
			entries := make([]domain.ChartEntry, 0, len(scrobbles))
			for _, scrobble := range scrobbles {
				entry := domain.ChartEntry{Artist: scrobble.Artist, Scrobbles: 1}
				switch kind {
				case domain.Track:
					entry.Title = scrobble.Track
				case domain.Album:
					entry.Title = scrobble.Album
				}
				if kind == domain.Artist || entry.Title != "" {
					entries = append(entries, entry)
				}
			}
			return entries, more, nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	participants, err := s.participants.List(ctx)
	if err != nil {
		return Ranking{}, fmt.Errorf("list participants for %s chart: %w", kind, err)
	}

	type profileChart struct {
		entries map[chartKey]domain.ChartEntry
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
			profiles[index].entries, profiles[index].err = readChartPages(ctx, participant.LastFMUsername, readPage)
		})
	}
	requests.Wait()

	ranking := Ranking{ProfilesTotal: len(participants)}
	totals := make(map[chartKey]domain.ChartEntry)
	var profileErrors []error
	for _, profile := range profiles {
		if profile.err != nil {
			profileErrors = append(profileErrors, profile.err)
			continue
		}
		ranking.ProfilesIncluded++
		for _, entry := range profile.entries {
			addScrobbles(totals, entry)
		}
	}
	if len(profileErrors) > 0 && ranking.ProfilesIncluded == 0 {
		return Ranking{}, fmt.Errorf("load %s charts: %w", kind, errors.Join(profileErrors...))
	}

	for _, total := range totals {
		ranking.Entries = append(ranking.Entries, total)
	}
	slices.SortFunc(ranking.Entries, func(a, b domain.ChartEntry) int {
		if byCount := cmp.Compare(b.Scrobbles, a.Scrobbles); byCount != 0 {
			return byCount
		}
		if byArtist := strings.Compare(strings.ToLower(a.Artist), strings.ToLower(b.Artist)); byArtist != 0 {
			return byArtist
		}
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})
	ranking.Entries = ranking.Entries[:min(Limit, len(ranking.Entries))]
	return ranking, nil
}

type pageReader func(context.Context, string, int) ([]domain.ChartEntry, bool, error)

type chartKey struct {
	artist string
	title  string
}

func readChartPages(ctx context.Context, username string, readPage pageReader) (map[chartKey]domain.ChartEntry, error) {
	chart := make(map[chartKey]domain.ChartEntry)
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("read chart: %w", err)
		}
		entries, more, err := readPage(ctx, username, page)
		if err != nil {
			return nil, fmt.Errorf("load chart page %d: %w", page, err)
		}
		for _, entry := range entries {
			addScrobbles(chart, entry)
		}
		if !more {
			return chart, nil
		}
	}
}

func addScrobbles(totals map[chartKey]domain.ChartEntry, entry domain.ChartEntry) {
	key := chartKey{artist: strings.ToLower(entry.Artist), title: strings.ToLower(entry.Title)}
	total, exists := totals[key]
	if !exists {
		totals[key] = entry
		return
	}
	total.Scrobbles += entry.Scrobbles
	totals[key] = total
}
