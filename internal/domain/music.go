package domain

type MusicKind string

const (
	Artist MusicKind = "artist"
	Track  MusicKind = "track"
	Album  MusicKind = "album"
)

type MusicTarget struct {
	Kind   MusicKind
	Artist string
	Title  string
}

type NowPlayingTrack struct {
	Artist   string
	Title    string
	Album    string
	CoverURL string
}

type ArtistScrobbles struct {
	Name      string
	Scrobbles int64
}

type Scrobble struct {
	Artist string
	Track  string
	Album  string
}
