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
	return c.sendText(ctx, chatID, threadID, text, "")
}

func (c *Client) SendHTML(ctx context.Context, chatID, threadID int64, text string) error {
	return c.sendText(ctx, chatID, threadID, text, "HTML")
}

func (c *Client) sendText(ctx context.Context, chatID, threadID int64, text, parseMode string) error {
	payload := struct {
		ChatID    int64  `json:"chat_id"`
		ThreadID  int64  `json:"message_thread_id,omitempty"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode,omitempty"`
	}{ChatID: chatID, ThreadID: threadID, Text: text, ParseMode: parseMode}
	return c.call(ctx, "sendMessage", payload, nil)
}

func (c *Client) BotUsername(ctx context.Context) (string, error) {
	var bot struct {
		Username string `json:"username"`
	}
	if err := c.call(ctx, "getMe", struct{}{}, &bot); err != nil {
		return "", fmt.Errorf("get Telegram bot username: %w", err)
	}
	if bot.Username == "" {
		return "", errors.New("telegram getMe: response did not include a username")
	}
	return bot.Username, nil
}

func (c *Client) call(ctx context.Context, method string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram %s: encode request: %w", method, err)
	}

	endpoint := "https://api.telegram.org/bot" + url.PathEscape(c.token) + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram %s: create request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		// Ошибка транспорта может содержать адрес с токеном бота.
		switch {
		case errors.Is(err, context.Canceled):
			return fmt.Errorf("telegram %s: %w", method, context.Canceled)
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("telegram %s: %w", method, context.DeadlineExceeded)
		default:
			return fmt.Errorf("telegram %s: HTTP request failed", method)
		}
	}
	defer res.Body.Close()

	var response struct {
		OK        bool            `json:"ok"`
		ErrorCode int             `json:"error_code"`
		Result    json.RawMessage `json:"result"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&response)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("telegram %s: HTTP status %d (API code %d)", method, res.StatusCode, response.ErrorCode)
	}
	if err != nil {
		return fmt.Errorf("telegram %s: decode response: %w", method, err)
	}
	if !response.OK {
		return fmt.Errorf("telegram %s: API error %d", method, response.ErrorCode)
	}
	if result != nil {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("telegram %s: decode result: %w", method, err)
		}
	}
	return nil
}
