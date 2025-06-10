// tags wraps go-taglib to normalise known tag variants
package tags

import (
	"iter"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"go.senan.xyz/taglib"
)

// https://taglib.org/api/p_propertymapping.html
// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

//go:generate go run gen_taglist.go -- $GOFILE taglist.gen.go
//nolint:gosec
const (
	Album              = "ALBUM"
	AlbumArtist        = "ALBUMARTIST"         //tag: alts "ALBUM_ARTIST"
	AlbumArtists       = "ALBUMARTISTS"        //tag: alts "ALBUM_ARTISTS"
	AlbumArtistCredit  = "ALBUMARTIST_CREDIT"  //tag: alts "ALBUM_ARTIST_CREDIT"
	AlbumArtistsCredit = "ALBUMARTISTS_CREDIT" //tag: alts "ALBUM_ARTISTS_CREDIT"
	Date               = "DATE"                //tag: alts "YEAR"
	OriginalDate       = "ORIGINALDATE"        //tag: alts "ORIGINAL_YEAR"
	MediaFormat        = "MEDIA"
	Label              = "LABEL"
	CatalogueNum       = "CATALOGNUMBER" //tag: alts "CATALOGNUM"
	UPC                = "UPC"           //tag: alts "MCN"
	Compilation        = "COMPILATION"

	MBReleaseID      = "MUSICBRAINZ_ALBUMID"
	MBReleaseGroupID = "MUSICBRAINZ_RELEASEGROUPID"
	MBAlbumArtistID  = "MUSICBRAINZ_ALBUMARTISTID"
	MBAlbumComment   = "MUSICBRAINZ_ALBUMCOMMENT"

	Title         = "TITLE"
	Artist        = "ARTIST"
	Artists       = "ARTISTS"
	ArtistCredit  = "ARTIST_CREDIT"  //tag: alts "ARTISTCREDIT"
	ArtistsCredit = "ARTISTS_CREDIT" //tag: alts "ARTISTSCREDIT"
	Genre         = "GENRE"
	Genres        = "GENRES"
	TrackNumber   = "TRACKNUMBER" //tag: alts "TRACK" "TRACKC"
	DiscNumber    = "DISCNUMBER"

	MBRecordingID = "MUSICBRAINZ_TRACKID"
	MBArtistID    = "MUSICBRAINZ_ARTISTID"

	ReplayGainTrackGain = "REPLAYGAIN_TRACK_GAIN"
	ReplayGainTrackPeak = "REPLAYGAIN_TRACK_PEAK"
	ReplayGainAlbumGain = "REPLAYGAIN_ALBUM_GAIN"
	ReplayGainAlbumPeak = "REPLAYGAIN_ALBUM_PEAK"

	Lyrics = "LYRICS" //tag: alts "LYRICS:DESCRIPTION" "USLT:DESCRIPTION" "©LYR"
)

type WriteOption = taglib.WriteOption

const (
	Clear           = taglib.Clear
	DiffBeforeWrite = taglib.DiffBeforeWrite
)

func CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".opus", ".aac", ".aiff", ".ape", ".m4a", ".m4b", ".mp2", ".mpc", ".oga", ".ogg", ".spx", ".tak", ".wav", ".wma", ".wv":
		return true
	}
	return false
}

func ReadTags(path string) (Tags, error) {
	t, err := taglib.ReadTags(path)
	return Tags{t}, err
}

func WriteTags(path string, tags Tags, opts WriteOption) error {
	return taglib.WriteTags(path, tags.t, opts)
}

func ReadProperties(path string) (taglib.Properties, error) {
	return taglib.ReadProperties(path)
}

type Tags struct {
	t map[string][]string
}

func NewTags(vs ...string) Tags {
	if len(vs)%2 != 0 {
		panic("vs should be kv pairs")
	}
	var t Tags
	for i := 0; i < len(vs)-1; i += 2 {
		t.Set(vs[i], vs[i+1])
	}
	return t
}

func (t Tags) Iter() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		for _, k := range slices.Sorted(maps.Keys(t.t)) {
			if k := NormKey(k); !yield(k, t.t[k]) {
				break
			}
		}
	}
}

func (t *Tags) Set(key string, values ...string) {
	if t.t == nil {
		t.t = map[string][]string{}
	}
	t.t[NormKey(key)] = values
}

func (t Tags) Get(key string) string {
	if vs := t.t[NormKey(key)]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func (t Tags) Values(key string) []string {
	return t.t[NormKey(key)]
}

func Equal(a, b Tags) bool {
	return maps.EqualFunc(a.t, b.t, slices.Equal)
}

func NormKey(k string) string {
	k = strings.ToUpper(k)
	if nk, ok := alternatives[k]; ok {
		return nk
	}
	return k
}
