// tags wraps go-taglib to normalise known tag variants
package tags

import (
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
	Date               = "DATE"                //tag: alts "YEAR" "RELEASEDATE"
	OriginalDate       = "ORIGINALDATE"        //tag: alts "ORIGINAL_YEAR"
	MediaFormat        = "MEDIA"
	Label              = "LABEL"
	CatalogueNum       = "CATALOGNUMBER" //tag: alts "CATALOGNUM" "CAT#" "CATALOGID" "CATNUM"
	UPC                = "UPC"           //tag: alts "MCN" "BARCODE"
	Compilation        = "COMPILATION"
	ReleaseType        = "RELEASETYPE"

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
	MBTrackID     = "MUSICBRAINZ_RELEASETRACKID"
	MBArtistID    = "MUSICBRAINZ_ARTISTID"

	ReplayGainTrackGain = "REPLAYGAIN_TRACK_GAIN"
	ReplayGainTrackPeak = "REPLAYGAIN_TRACK_PEAK"
	ReplayGainAlbumGain = "REPLAYGAIN_ALBUM_GAIN"
	ReplayGainAlbumPeak = "REPLAYGAIN_ALBUM_PEAK"

	Lyrics = "LYRICS" //tag: alts "LYRICS:DESCRIPTION" "USLT:DESCRIPTION" "©LYR"

	Encoder = "ENCODER"
	Comment = "COMMENT"
)

type WriteOption = taglib.WriteOption

const (
	Clear = taglib.Clear
)

func CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".opus", ".aac", ".aiff", ".ape", ".m4a", ".m4b", ".mp2", ".mpc", ".oga", ".ogg", ".spx", ".tak", ".wav", ".wma", ".wv":
		return true
	}
	return false
}

func ReadTags(path string) (Tags, error) {
	rt, err := taglib.ReadTags(path)
	if err != nil {
		return Tags{}, err
	}

	// the internal state of t should be always be normalised so that later users of
	// Get and Set, potentially with non-normalised keys will find a match
	var t = make(Tags, len(rt))
	for k, vs := range rt {
		t.Set(k, vs...)
	}
	return t, nil
}

func WriteTags(path string, tags Tags, opts WriteOption) error {
	return taglib.WriteTags(path, tags, opts)
}

func ReadProperties(path string) (taglib.Properties, error) {
	return taglib.ReadProperties(path)
}

type Tags map[string][]string

func NewTags(vs ...string) Tags {
	if len(vs)%2 != 0 {
		panic("vs should be kv pairs")
	}
	var t = Tags{}
	for i := 0; i < len(vs)-1; i += 2 {
		t.Set(vs[i], vs[i+1])
	}
	return t
}

func (t Tags) Set(key string, values ...string) {
	t[NormKey(key)] = values
}

func (t Tags) Get(key string) string {
	if vs := t[NormKey(key)]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func (t Tags) Values(key string) []string {
	return t[NormKey(key)]
}

func Equal(a, b Tags) bool {
	// not using maps.EqualFunc(x, slices.Equal) here since we don't care
	// about a difference in not present vs present but 0 len
	for k, vs := range a {
		if ovs := b[k]; !slices.Equal(vs, ovs) {
			return false
		}
	}
	for k, vs := range b {
		if ovs := a[k]; !slices.Equal(vs, ovs) {
			return false
		}
	}
	return true
}

func NormKey(k string) string {
	k = strings.ToUpper(k)
	if nk, ok := alternatives[k]; ok {
		return nk
	}
	return k
}

func KnownTags() map[string]struct{} {
	return maps.Clone(knownTags)
}
