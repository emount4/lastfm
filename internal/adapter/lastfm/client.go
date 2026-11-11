package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	apiEndpoint     = "https://ws.audioscrobbler.com/2.0/"
	maxResponseSize = 2 << 20
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	httpClient HTTPDoer
	apiKey     string
}

func NewClient(httpClient HTTPDoer, apiKey string) *Client {
	return &Client{httpClient: httpClient, apiKey: apiKey}
}

func (c *Client) get(ctx context.Context, method string, params url.Values, result any) error {
	params.Set("method", method)
	params.Set("api_key", c.apiKey)
	params.Set("format", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("lastfm %s: create request: %w", method, err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		// Ошибка транспорта может содержать полный адрес запроса с ключом API.
		switch {
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("lastfm %s: %w", method, context.Canceled)
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("lastfm %s: %w", method, context.DeadlineExceeded)
		default:
			return fmt.Errorf("lastfm %s: HTTP request failed", method)
		}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseSize+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("lastfm %s: read response: %w", method, ctxErr)
		}
		return fmt.Errorf("lastfm %s: read response: %w", method, err)
	}
	if len(body) > maxResponseSize {
		return fmt.Errorf("lastfm %s: response exceeds size limit", method)
	}

	var failure apiError
	parseErr := json.Unmarshal(body, &failure)
	if parseErr == nil && failure.Code != 0 {
		return fmt.Errorf("lastfm %s: %w", method, &failure)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("lastfm %s: HTTP status %d", method, res.StatusCode)
	}
	if parseErr != nil {
		return fmt.Errorf("lastfm %s: invalid JSON response: %w", method, parseErr)
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("lastfm %s: invalid JSON response: %w", method, err)
	}
	return nil
}
