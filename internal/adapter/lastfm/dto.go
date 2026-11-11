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
		Name      string      `json:"name"`
		PlayCount json.Number `json:"playcount"`
	} `json:"user"`
}

type recentTracksResponse struct {
	RecentTracks struct {
		Tracks     []json.RawMessage `json:"track"`
		Pagination pagination        `json:"@attr"`
	} `json:"recenttracks"`
}

type pagination struct {
	Page       json.Number `json:"page"`
	TotalPages json.Number `json:"totalPages"`
	Total      json.Number `json:"total"`
}

type recentTrack struct {
	Name   string `json:"name"`
	Artist struct {
		Name string `json:"#text"`
	} `json:"artist"`
	Album struct {
		Name string `json:"#text"`
	} `json:"album"`
	Images []trackImage `json:"image"`
	Attrs  struct {
		NowPlaying string `json:"nowplaying"`
	} `json:"@attr"`
	Date struct {
		Timestamp json.Number `json:"uts"`
	} `json:"date"`
}

type trackImage struct {
	Size string `json:"size"`
	URL  string `json:"#text"`
}

type personalPlayCount struct {
	UserPlayCount json.Number `json:"userplaycount"`
}

type playCountResponse struct {
	Artist struct {
		Name  string            `json:"name"`
		Stats personalPlayCount `json:"stats"`
	} `json:"artist"`
	Track struct {
		personalPlayCount
		Name   string `json:"name"`
		Artist struct {
			Name string `json:"name"`
		} `json:"artist"`
	} `json:"track"`
	Album struct {
		personalPlayCount
		Name   string `json:"name"`
		Artist string `json:"artist"`
	} `json:"album"`
}

type musicSearchMatch struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
}

type musicSearchResponse struct {
	Results struct {
		TrackMatches struct {
			Tracks []musicSearchMatch `json:"track"`
		} `json:"trackmatches"`
		AlbumMatches struct {
			Albums []musicSearchMatch `json:"album"`
		} `json:"albummatches"`
	} `json:"results"`
}

type artistTopAlbumsResponse struct {
	TopAlbums struct {
		Albums []struct {
			Name   string `json:"name"`
			Artist struct {
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"album"`
	} `json:"topalbums"`
}
