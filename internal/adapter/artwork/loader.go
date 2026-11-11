package artwork

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"

	_ "golang.org/x/image/webp"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Loader struct {
	httpClient HTTPDoer
}

func NewLoader(httpClient HTTPDoer) *Loader {
	return &Loader{httpClient: httpClient}
}

func (l *Loader) Load(ctx context.Context, address string) (image.Image, error) {
	asset, err := url.Parse(address)
	if err != nil || asset.Scheme != "https" || asset.User != nil || asset.Port() != "" {
		return nil, errors.New("load image: invalid image address")
	}
	switch strings.ToLower(asset.Hostname()) {
	case "lastfm.freetls.fastly.net", "lastfm-img2.akamaized.net", "api.telegram.org":
	default:
		return nil, errors.New("load image: unsupported host")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.String(), nil)
	if err != nil {
		return nil, errors.New("load image: create request failed")
	}
	res, err := l.httpClient.Do(req)
	if err != nil {
		// Адрес фотографии Telegram может содержать токен бота.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("load image: %w", ctxErr)
		}
		return nil, errors.New("load image: request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("load image: HTTP status %d", res.StatusCode)
	}

	const maxBytes = 5 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("load image: read response: %w", err)
	}
	if len(data) > maxBytes {
		return nil, errors.New("load image: response exceeds size limit")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("load image: decode dimensions: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 2048 || config.Height > 2048 {
		return nil, errors.New("load image: unsupported image dimensions")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("load image: decode image: %w", err)
	}
	return img, nil
}
