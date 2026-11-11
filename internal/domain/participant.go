package domain

import (
	"errors"
	"strings"
)

var ErrLastFMAccountTaken = errors.New("lastfm account is already linked to another participant")

type Participant struct {
	TelegramUserID      int64
	TelegramUsername    string
	TelegramDisplayName string
	LastFMUsername      string
}

func (p Participant) TelegramName() string {
	if name := strings.TrimSpace(p.TelegramDisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(p.TelegramUsername)
}
