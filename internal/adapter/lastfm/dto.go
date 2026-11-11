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
		Stats personalPlayCount `json:"stats"`
	} `json:"artist"`
	Track personalPlayCount `json:"track"`
	Album personalPlayCount `json:"album"`
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
