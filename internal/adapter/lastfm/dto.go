package lastfm

import (
	"encoding/json"
	"fmt"
)

type apiError struct {
	Code    int    `json:"error"`
	Message string `json:"message"`
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API error %d", e.Code)
}

type profileResponse struct {
	User struct {
		Name string `json:"name"`
	} `json:"user"`
}

type recentTracksResponse struct {
	RecentTracks struct {
		Tracks []json.RawMessage `json:"track"`
	} `json:"recenttracks"`
}
