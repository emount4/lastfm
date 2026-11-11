package lastfm

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// LookupProfile возвращает канонический ник и признак существования профиля.
// Если Last.fm сообщает, что профиль не найден, метод возвращает пустой ник, false и nil.
func (c *Client) LookupProfile(ctx context.Context, username string) (name string, found bool, err error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", false, errors.New("lastfm user.getInfo: username is required")
	}

	var profile profileResponse
	if err := c.get(ctx, "user.getInfo", url.Values{"user": {username}}, &profile); err != nil {
		var failure *apiError
		if errors.As(err, &failure) && failure.Code == 6 &&
			strings.EqualFold(strings.TrimSpace(failure.Message), "User not found") {
			return "", false, nil
		}
		return "", false, err
	}

	if name := strings.TrimSpace(profile.User.Name); name != "" {
		return name, true, nil
	}
	return "", false, errors.New("lastfm user.getInfo: response did not include a username")
}
