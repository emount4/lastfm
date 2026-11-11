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

type ArtistCountReader interface {
	ArtistPlayCount(ctx context.Context, username, artist string) (int64, error)
}

type Entry struct {
	Participant domain.Participant
	Scrobbles   int64
}

type Ranking struct {
	Entries []Entry
	Total   int64
}

type Artists struct {
	participants ParticipantReader
	counts       ArtistCountReader
}

func NewArtists(participants ParticipantReader, counts ArtistCountReader) *Artists {
	return &Artists{participants: participants, counts: counts}
}

func (s *Artists) RankArtist(ctx context.Context, artist string) (Ranking, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	participants, err := s.participants.List(ctx)
	if err != nil {
		return Ranking{}, fmt.Errorf("rank artist: list participants: %w", err)
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
			count, err := s.counts.ArtistPlayCount(ctx, participant.LastFMUsername, artist)
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
