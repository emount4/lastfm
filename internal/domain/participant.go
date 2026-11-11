package domain

import "errors"

var ErrLastFMAccountTaken = errors.New("lastfm account is already linked to another participant")

type Participant struct {
	TelegramUserID      int64
	TelegramUsername    string
	TelegramDisplayName string
	LastFMUsername      string
}
