package vibe

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

const MaxDays = 30

var (
	ErrNotRegistered = errors.New("listener is not registered")
	ErrNoScrobbles   = errors.New("listener has no scrobbles in the period")
	ErrRateLimited   = errors.New("vibe generator rate limited")
)

type ParticipantReader interface {
	List(context.Context) ([]domain.Participant, error)
}

type ScrobbleReader interface {
	ScrobblesPage(context.Context, string, domain.ScrobbleRange, int) ([]domain.Scrobble, bool, error)
}

type Generator interface {
	Describe(context.Context, Summary) (string, error)
}

type TrackCount struct {
	Artist string
	Track  string
	Count  int
}

type Summary struct {
	Days      int
	Scrobbles int
	TopTracks []TrackCount
}

type Service struct {
	participants ParticipantReader
	scrobbles    ScrobbleReader
	generator    Generator
	now          func() time.Time
}

func NewService(participants ParticipantReader, scrobbles ScrobbleReader, generator Generator, now func() time.Time) *Service {
	return &Service{participants: participants, scrobbles: scrobbles, generator: generator, now: now}
}

func ParseDays(args string) (int, error) {
	args = strings.ToLower(strings.TrimSpace(args))
	if args == "" {
		return 1, nil
	}
	number, ok := strings.CutSuffix(args, "d")
	if !ok {
		return 0, domain.ErrInvalidPeriod
	}
	days, err := strconv.ParseUint(number, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", domain.ErrInvalidPeriod, err)
	}
	if days == 0 || days > MaxDays {
		return 0, domain.ErrInvalidPeriod
	}
	return int(days), nil
}

func (s *Service) Describe(ctx context.Context, telegramUserID int64, days int) (string, error) {
	if days < 1 || days > MaxDays {
		return "", domain.ErrInvalidPeriod
	}
	now := s.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	interval := domain.ScrobbleRange{From: today.AddDate(0, 0, 1-days).Unix(), To: now.Unix()}
	historyCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	participants, err := s.participants.List(historyCtx)
	if err != nil {
		return "", fmt.Errorf("list participants for scrobble vibe: %w", err)
	}
	index := slices.IndexFunc(participants, func(participant domain.Participant) bool {
		return participant.TelegramUserID == telegramUserID
	})
	if index == -1 {
		return "", ErrNotRegistered
	}
	if interval.From >= interval.To {
		return "", ErrNoScrobbles
	}
	summary, err := s.summarize(historyCtx, participants[index].LastFMUsername, interval, days)
	if err != nil {
		return "", fmt.Errorf("summarize listening for scrobble vibe: %w", err)
	}
	if len(summary.TopTracks) == 0 {
		return "", ErrNoScrobbles
	}

	reply, err := s.generator.Describe(ctx, summary)
	if err != nil {
		return "", fmt.Errorf("generate scrobble vibe: %w", err)
	}
	return reply, nil
}

func (s *Service) summarize(ctx context.Context, username string, interval domain.ScrobbleRange, days int) (Summary, error) {
	var summary = Summary{Days: days}
	counts := make(map[[2]string]TrackCount)
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return Summary{}, fmt.Errorf("read vibe history: %w", err)
		}
		rows, more, err := s.scrobbles.ScrobblesPage(ctx, username, interval, page)
		if err != nil {
			return Summary{}, fmt.Errorf("load vibe history page %d: %w", page, err)
		}
		summary.Scrobbles += len(rows)
		for _, row := range rows {
			if row.Track == "" {
				continue
			}
			key := [2]string{strings.ToLower(row.Artist), strings.ToLower(row.Track)}
			entry := counts[key]
			if entry.Count == 0 {
				entry = TrackCount{Artist: row.Artist, Track: row.Track}
			}
			entry.Count++
			counts[key] = entry
		}
		if !more {
			break
		}
	}
	for _, entry := range counts {
		summary.TopTracks = append(summary.TopTracks, entry)
	}
	slices.SortFunc(summary.TopTracks, func(a, b TrackCount) int {
		if byCount := cmp.Compare(b.Count, a.Count); byCount != 0 {
			return byCount
		}
		if byArtist := cmp.Compare(a.Artist, b.Artist); byArtist != 0 {
			return byArtist
		}
		return cmp.Compare(a.Track, b.Track)
	})
	summary.TopTracks = summary.TopTracks[:min(10, len(summary.TopTracks))]
	return summary, nil
}
