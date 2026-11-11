package collage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"math"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/emount4/lastfm/internal/application/playing"
)

type Renderer struct {
	regular *opentype.Font
	bold    *opentype.Font
}

func NewRenderer() (*Renderer, error) {
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse regular collage font: %w", err)
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse bold collage font: %w", err)
	}
	return &Renderer{regular: regular, bold: bold}, nil
}

func (r *Renderer) Render(cards []playing.Card) ([]byte, error) {
	if len(cards) == 0 {
		return nil, errors.New("render collage: no cards")
	}
	columns := columnCount(len(cards))
	rows := (len(cards) + columns - 1) / columns
	tile := min(480, 1920/max(columns, rows))
	if tile < 96 {
		return nil, errors.New("render collage: too many cards")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, columns*tile, rows*tile))
	stddraw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{R: 15, G: 16, B: 21, A: 255}), image.Point{}, stddraw.Src)

	regular, err := opentype.NewFace(r.regular, &opentype.FaceOptions{Size: float64(tile) * 0.054, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("create regular collage face: %w", err)
	}
	defer regular.Close()
	bold, err := opentype.NewFace(r.bold, &opentype.FaceOptions{Size: float64(tile) * 0.065, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("create bold collage face: %w", err)
	}
	defer bold.Close()

	for i, card := range cards {
		row, column := i/columns, i%columns
		countInRow := min(columns, len(cards)-row*columns)
		left := (columns-countInRow)*tile/2 + column*tile
		top := row * tile
		drawCard(canvas, image.Rect(left, top, left+tile, top+tile), card, bold, regular)
	}

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, canvas, &jpeg.Options{Quality: 88}); err != nil {
		return nil, fmt.Errorf("encode collage: %w", err)
	}
	return encoded.Bytes(), nil
}

func columnCount(count int) int {
	switch count {
	case 1, 2, 3:
		return count
	case 4:
		return 2
	case 5, 6, 9:
		return 3
	case 7, 8, 10:
		return 4
	default:
		return int(math.Ceil(math.Sqrt(float64(count))))
	}
}

func drawCard(canvas *image.RGBA, rect image.Rectangle, card playing.Card, bold, regular font.Face) {
	tile := rect.Dx()
	stddraw.Draw(canvas, rect, image.NewUniform(color.RGBA{R: 62, G: 65, B: 79, A: 255}), image.Point{}, stddraw.Src)
	if card.Cover != nil {
		fillSquare(canvas, rect, card.Cover)
	}
	gradientStart := rect.Min.Y + tile*3/5
	for y := gradientStart; y < rect.Max.Y; y++ {
		alpha := uint8(243 * (y - gradientStart) / (rect.Max.Y - gradientStart))
		line := image.Rect(rect.Min.X, y, rect.Max.X, y+1)
		stddraw.Draw(canvas, line, image.NewUniform(color.RGBA{A: alpha}), image.Point{}, stddraw.Over)
	}

	padding := tile / 22
	if card.Avatar != nil {
		drawAvatar(canvas, rect.Min.Add(image.Pt(padding, padding)), tile/7, card.Avatar)
	}
	lineGap := tile / 14
	baseline := rect.Max.Y - padding
	x, width := rect.Min.X+padding, tile-2*padding
	drawLine(canvas, regular, card.Artist, x, baseline, width)
	drawLine(canvas, bold, card.Track, x, baseline-lineGap, width)
	drawLine(canvas, regular, card.Name, x, baseline-2*lineGap, width)
}

func fillSquare(dst stddraw.Image, rect image.Rectangle, src image.Image) {
	bounds := src.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	if side <= 0 {
		return
	}
	start := bounds.Min.Add(image.Pt((bounds.Dx()-side)/2, (bounds.Dy()-side)/2))
	crop := image.Rectangle{Min: start, Max: start.Add(image.Pt(side, side))}
	draw.CatmullRom.Scale(dst, rect, src, crop, stddraw.Over, nil)
}

func drawAvatar(canvas *image.RGBA, position image.Point, size int, source image.Image) {
	bounds := image.Rect(0, 0, size, size)
	avatar := image.NewRGBA(bounds)
	fillSquare(avatar, bounds, source)
	mask := image.NewAlpha(bounds)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			distance := math.Hypot(float64(2*x+1-size), float64(2*y+1-size))
			mask.SetAlpha(x, y, color.Alpha{A: uint8(255 * max(0, min(1, (float64(size)-distance+1)/2)))})
		}
	}
	stddraw.DrawMask(canvas, bounds.Add(position), avatar, image.Point{}, mask, image.Point{}, stddraw.Over)
}

func drawLine(dst stddraw.Image, face font.Face, text string, x, y, width int) {
	text = strings.Join(strings.Fields(text), " ")
	limit := fixed.I(width)
	if font.MeasureString(face, text) > limit {
		runes := []rune(text)
		for n := min(len(runes), 160); n >= 0; n-- {
			text = string(runes[:n]) + "…"
			if font.MeasureString(face, text) <= limit {
				break
			}
		}
	}
	drawer := font.Drawer{Dst: dst, Src: image.White, Face: face, Dot: fixed.P(x, y)}
	drawer.DrawString(text)
}
