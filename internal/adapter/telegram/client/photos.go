package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/url"
	"strconv"
)

func (c *Client) SendStatus(ctx context.Context, chatID, threadID int64, text string) (int64, error) {
	var message struct {
		ID int64 `json:"message_id"`
	}
	if err := c.sendText(ctx, chatID, threadID, text, "", &message); err != nil {
		return 0, fmt.Errorf("send Telegram status: %w", err)
	}
	return message.ID, nil
}

func (c *Client) SendPhoto(ctx context.Context, chatID, threadID int64, photo []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("encode photo chat ID: %w", err)
	}
	if threadID != 0 {
		if err := writer.WriteField("message_thread_id", strconv.FormatInt(threadID, 10)); err != nil {
			return fmt.Errorf("encode photo thread ID: %w", err)
		}
	}
	file, err := writer.CreateFormFile("photo", "playing.jpg")
	if err != nil {
		return fmt.Errorf("create photo form file: %w", err)
	}
	if _, err := file.Write(photo); err != nil {
		return fmt.Errorf("encode photo data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish photo form: %w", err)
	}
	return c.request(ctx, "sendPhoto", writer.FormDataContentType(), &body, nil)
}

func (c *Client) AvatarURL(ctx context.Context, userID int64) (string, error) {
	params := struct {
		UserID int64 `json:"user_id"`
		Limit  int   `json:"limit"`
	}{UserID: userID, Limit: 1}
	var profile struct {
		Photos [][]struct {
			FileID string `json:"file_id"`
			Width  int    `json:"width"`
		} `json:"photos"`
	}
	if err := c.call(ctx, "getUserProfilePhotos", params, &profile); err != nil {
		return "", fmt.Errorf("get avatar: %w", err)
	}
	if len(profile.Photos) == 0 || len(profile.Photos[0]) == 0 {
		return "", nil
	}
	photo := profile.Photos[0][0]
	for _, option := range profile.Photos[0] {
		if option.Width >= 128 {
			photo = option
			break
		}
		if option.Width > photo.Width {
			photo = option
		}
	}
	var file struct {
		Path string `json:"file_path"`
	}
	if err := c.call(ctx, "getFile", map[string]string{"file_id": photo.FileID}, &file); err != nil {
		return "", fmt.Errorf("get avatar file: %w", err)
	}
	if !fs.ValidPath(file.Path) || file.Path == "." {
		return "", errors.New("get avatar file: invalid path")
	}
	asset := url.URL{Scheme: "https", Host: "api.telegram.org", Path: "/file/bot" + c.token + "/" + file.Path}
	return asset.String(), nil
}
