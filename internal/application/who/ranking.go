package who

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type ParticipantReader interface {
	List(context.Context) ([]domain.Participant, error)
}

type PlayCountReader interface {
	PlayCount(ctx context.Context, username string, target domain.MusicTarget) (int64, error)
}

type Entry struct {
	Participant domain.Participant
	Scrobbles   int64
}

type Ranking struct {
	Entries []Entry
	Total   int64
}

type Service struct {
	participants ParticipantReader
	counts       PlayCountReader
}

func NewService(participants ParticipantReader, counts PlayCountReader) *Service {
	return &Service{participants: participants, counts: counts}
}

func (s *Service) Rank(ctx context.Context, target domain.MusicTarget) (Ranking, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	participants, err := s.participants.List(ctx)
	if err != nil {
		return Ranking{}, fmt.Errorf("rank %s: list participants: %w", target.Kind, err)
	}

	counts := make([]int64, len(participants))
	slots := make(chan struct{}, 4)
	var requests sync.WaitGroup
	for index, participant := range participants {
		requests.Go(func() {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				return
			}
			count, err := s.counts.PlayCount(ctx, participant.LastFMUsername, target)
			if err == nil {
				counts[index] = count
			}
		})
	}
	requests.Wait()

	var ranking Ranking
	for index, count := range counts {
		if count <= 0 {
			continue
		}
		ranking.Entries = append(ranking.Entries, Entry{Participant: participants[index], Scrobbles: count})
		ranking.Total += count
	}
	slices.SortStableFunc(ranking.Entries, func(a, b Entry) int {
		return cmp.Compare(b.Scrobbles, a.Scrobbles)
	})
	return ranking, nil
}
