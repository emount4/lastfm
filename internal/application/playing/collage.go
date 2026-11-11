package playing

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type ListenerReader interface {
	ActiveListeners(context.Context) ([]Listener, error)
}

type AlbumCoverReader interface {
	AlbumCover(ctx context.Context, artist, album string) (string, error)
}

type AvatarReader interface {
	AvatarURL(ctx context.Context, userID int64) (string, error)
}

type ImageLoader interface {
	Load(context.Context, string) (image.Image, error)
}

type Renderer interface {
	Render([]Card) ([]byte, error)
}

type Card struct {
	Name   string
	Artist string
	Track  string
	Cover  image.Image
	Avatar image.Image
}

type Collage struct {
	listeners ListenerReader
	albums    AlbumCoverReader
	avatars   AvatarReader
	images    ImageLoader
	renderer  Renderer
}

func NewCollage(listeners ListenerReader, albums AlbumCoverReader, avatars AvatarReader, images ImageLoader, renderer Renderer) *Collage {
	return &Collage{listeners: listeners, albums: albums, avatars: avatars, images: images, renderer: renderer}
}

func (c *Collage) Build(ctx context.Context) ([]byte, error) {
	listeners, err := c.listeners.ActiveListeners(ctx)
	if err != nil {
		return nil, fmt.Errorf("build collage: get active listeners: %w", err)
	}
	if len(listeners) == 0 {
		return nil, nil
	}

	mediaCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cards := make([]Card, len(listeners))
	slots := make(chan struct{}, 6)
	var downloads sync.WaitGroup
	load := func(task func()) {
		downloads.Go(func() {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-mediaCtx.Done():
				return
			}
			task()
		})
	}
	for index, listener := range listeners {
		name := listener.Participant.TelegramDisplayName
		if listener.Participant.TelegramUsername != "" {
			name = "@" + listener.Participant.TelegramUsername
		}
		cards[index] = Card{Name: name, Artist: listener.Track.Artist, Track: listener.Track.Title}
		load(func() {
			cards[index].Cover = c.loadCover(mediaCtx, listener.Track)
		})
		load(func() {
			if avatar, err := c.avatars.AvatarURL(mediaCtx, listener.Participant.TelegramUserID); err == nil && avatar != "" {
				cards[index].Avatar, _ = c.images.Load(mediaCtx, avatar)
			}
		})
	}
	downloads.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("build collage: %w", err)
	}
	photo, err := c.renderer.Render(cards)
	if err != nil {
		return nil, fmt.Errorf("build collage: render cards: %w", err)
	}
	return photo, nil
}

func (c *Collage) loadCover(ctx context.Context, track domain.NowPlayingTrack) image.Image {
	if track.CoverURL != "" {
		if cover, err := c.images.Load(ctx, track.CoverURL); err == nil && cover != nil {
			return cover
		}
	}
	if track.Album == "" || ctx.Err() != nil {
		return nil
	}
	url, err := c.albums.AlbumCover(ctx, track.Artist, track.Album)
	if err != nil || url == "" || url == track.CoverURL {
		return nil
	}
	cover, _ := c.images.Load(ctx, url)
	return cover
}
