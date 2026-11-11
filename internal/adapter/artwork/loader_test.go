package artwork

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"testing"
)

type imageHTTPFunc func(*http.Request) (*http.Response, error)

func (f imageHTTPFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLoadLastFMCover(t *testing.T) {
	const address = "https://lastfm-img.freetls.fastly.net/i/u/300x300/example.jpg"
	var cover bytes.Buffer
	if err := jpeg.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 2, 3)), nil); err != nil {
		t.Fatal(err)
	}
	loader := NewLoader(imageHTTPFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != address {
			t.Fatalf("requested URL = %q, want %q", req.URL, address)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(cover.Bytes())),
		}, nil
	}))
	img, err := loader.Load(t.Context(), address)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != image.Pt(2, 3) {
		t.Fatalf("cover size = %v, want 2x3", got)
	}
}

func TestLoadRejectsLookalikeCoverHost(t *testing.T) {
	loader := NewLoader(imageHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("request reached an unsupported host")
		return nil, nil
	}))
	if _, err := loader.Load(t.Context(), "https://lastfm-img.freetls.fastly.net.example.com/cover.jpg"); err == nil {
		t.Fatal("expected an error for an unsupported host")
	}
}
