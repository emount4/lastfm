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
