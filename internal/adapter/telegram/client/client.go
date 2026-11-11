package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	httpClient HTTPDoer
	token      string
}

func New(httpClient HTTPDoer, token string) *Client {
	return &Client{httpClient: httpClient, token: token}
}

func (c *Client) SendText(ctx context.Context, chatID, threadID int64, text string) error {
	payload := struct {
		ChatID   int64  `json:"chat_id"`
		ThreadID int64  `json:"message_thread_id,omitempty"`
		Text     string `json:"text"`
	}{ChatID: chatID, ThreadID: threadID, Text: text}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram sendMessage: encode request: %w", err)
	}

	endpoint := "https://api.telegram.org/bot" + url.PathEscape(c.token) + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram sendMessage: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		// Ошибка транспорта может содержать адрес с токеном бота.
		switch {
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("telegram sendMessage: %w", context.Canceled)
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("telegram sendMessage: %w", context.DeadlineExceeded)
		default:
			return errors.New("telegram sendMessage: HTTP request failed")
		}
	}
	defer res.Body.Close()

	var result struct {
		OK        bool `json:"ok"`
		ErrorCode int  `json:"error_code"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage: HTTP status %d (API code %d)", res.StatusCode, result.ErrorCode)
	}
	if err != nil {
		return fmt.Errorf("telegram sendMessage: decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram sendMessage: API error %d", result.ErrorCode)
	}
	return nil
}
