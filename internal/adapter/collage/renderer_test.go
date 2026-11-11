package collage

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/application/playing"
)

func TestRenderAdaptiveGrid(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	cover := image.NewRGBA(image.Rect(0, 0, 96, 80))
	draw.Draw(cover, cover.Bounds(), image.NewUniform(color.RGBA{R: 156, G: 103, B: 70, A: 255}), image.Point{}, draw.Src)
	for _, tt := range []struct {
		count  int
		width  int
		height int
	}{
		{count: 1, width: 480, height: 480},
		{count: 4, width: 960, height: 960},
		{count: 7, width: 1920, height: 960},
		{count: 10, width: 1920, height: 1440},
	} {
		cards := make([]playing.Card, tt.count)
		for i := range cards {
			cards[i] = playing.Card{
				Name: "@слушатель", Artist: "Артист", Track: strings.Repeat("Длинное название ", 12),
				Cover: cover,
			}
		}
		cards[0].Cover = nil
		photo, err := renderer.Render(cards)
		if err != nil {
			t.Fatalf("render %d cards: %v", tt.count, err)
		}
		img, err := jpeg.Decode(bytes.NewReader(photo))
		if err != nil {
			t.Fatalf("decode %d cards: %v", tt.count, err)
		}
		if got := img.Bounds().Size(); got != image.Pt(tt.width, tt.height) {
			t.Fatalf("%d cards: size = %v, want %dx%d", tt.count, got, tt.width, tt.height)
		}
		above, _, _, _ := img.At(450, 280).RGBA()
		below, _, _, _ := img.At(450, 474).RGBA()
		if below >= above {
			t.Fatal("caption gradient did not darken the card")
		}
		if tt.count == 7 {
			empty, _, _, _ := img.At(100, 520).RGBA()
			card, _, _, _ := img.At(350, 520).RGBA()
			if empty >= card {
				t.Fatal("incomplete row was not centered")
			}
		}
	}
}
