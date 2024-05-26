package tags

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/sentriz/audiotags"
)

var ErrWrite = errors.New("error writing tags")

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html
const (
	Album              = "album"
	AlbumArtist        = "albumartist"
	AlbumArtists       = "albumartists"
	AlbumArtistCredit  = "albumartist_credit"
	AlbumArtistsCredit = "albumartists_credit"
	Date               = "date"
	OriginalDate       = "originaldate"
	MediaFormat        = "media"
	Label              = "label"
	CatalogueNum       = "catalognumber"

	MBReleaseID      = "musicbrainz_albumid"
	MBReleaseGroupID = "musicbrainz_releasegroupid"
	MBAlbumArtistID  = "musicbrainz_albumartistid"
	MBAlbumComment   = "musicbrainz_albumcomment" // seems like beets uses this for disambiguations

	Title         = "title"
	Artist        = "artist"
	Artists       = "artists"
	ArtistCredit  = "artist_credit"
	ArtistsCredit = "artists_credit"
	Genre         = "genre"
	Genres        = "genres"
	TrackNumber   = "tracknumber"
	DiscNumber    = "discnumber"

	MBRecordingID = "musicbrainz_trackid"
	MBArtistID    = "musicbrainz_artistid"

	Lyrics = "lyrics"
)

func CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

type File struct {
	raw   map[string][]string
	props *audiotags.AudioProperties
	file  *audiotags.File
	path  string
}

func Read(path string) (*File, error) {
	f, err := audiotags.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	raw := f.ReadTags()
	normalise(raw, replacements) // tag replacements, case normalisation, etc

	props := f.ReadAudioProperties()
	return &File{raw: raw, props: props, file: f, path: path}, nil
}

func (f *File) Read(t string) string        { return first(f.raw[t]) }
func (f *File) ReadMulti(t string) []string { return f.raw[t] }
func (f *File) ReadNum(t string) int        { return anyNum(first(f.raw[t])) }
func (f *File) ReadTime(t string) time.Time { return anyTime(first(f.raw[t])) }

func (f *File) ReadAll(fn func(k string, vs []string) bool) {
	keys := make([]string, 0, len(f.raw))
	for k := range f.raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !fn(k, f.raw[k]) {
			break
		}
	}
}

func (f *File) Write(t string, v ...string) {
	v = deleteZero(v)
	if len(v) == 0 {
		delete(f.raw, t)
		return
	}
	f.raw[t] = v
}
func (f *File) WriteNum(t string, v int) { f.Write(t, intStr(v)) }

func (f *File) Clear(t string) { delete(f.raw, t) }
func (f *File) ClearAll()      { clear(f.raw) }

func (f *File) Length() time.Duration { return time.Duration(f.props.LengthMs) * time.Millisecond }
func (f *File) Bitrate() int          { return f.props.Bitrate }
func (f *File) SampleRate() int       { return f.props.Samplerate }
func (f *File) NumChannels() int      { return f.props.Channels }

func (f *File) CloneRaw() map[string][]string {
	return maps.Clone(f.raw)
}

func (f *File) Save() error {
	if !f.file.WriteTags(f.raw) {
		return ErrWrite
	}
	return nil
}

func (f *File) Close() {
	f.file.Close()
}

func (f *File) Path() string {
	return f.path
}

func Write(path string, fn func(f *File) error) error {
	f, err := Read(path)
	if err != nil {
		return fmt.Errorf("read tag file: %w", err)
	}
	defer f.Close()

	before := f.CloneRaw()
	if err := fn(f); err != nil {
		return err
	}
	after := f.CloneRaw()

	// try avoid filesystem writes if we can
	if maps.EqualFunc(before, after, slices.Equal) {
		return nil
	}

	if l := slog.Default(); l.Enabled(context.Background(), slog.LevelDebug) {
		for k := range after {
			if b, a := before[k], after[k]; !slices.Equal(b, a) {
				l.Debug("tag change", "key", k, "from", b, "to", a)
			}
		}
	}

	if err := f.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func first(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func intStr(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

var numExpr = regexp.MustCompile(`\d+`)

func anyNum(in string) int {
	match := numExpr.FindString(in)
	i, _ := strconv.Atoi(match)
	return i
}

func anyTime(str string) time.Time {
	t, _ := dateparse.ParseAny(str)
	return t
}

// tags we can use instead if we dont have the ones we're expecting.
// it maps [bad] -> [good]
var replacements = map[string]string{
	"year":                Date,
	"original_year":       OriginalDate,
	"track":               TrackNumber,
	"trackc":              TrackNumber,
	"catalognum":          CatalogueNum,
	"album_artists":       AlbumArtists,
	"album artist credit": AlbumArtistCredit,
	"artist credit":       ArtistCredit,
}

func normalise(raw map[string][]string, fallbacks map[string]string) {
	for kbad, kgood := range fallbacks {
		if _, ok := raw[kgood]; ok {
			continue
		}
		if v, ok := raw[kbad]; ok {
			raw[kgood] = v
			delete(raw, kbad)
			continue
		}
	}

	for k, vs := range raw {
		kNew := k
		kNew = strings.ToLower(kNew)
		kNew = strings.ReplaceAll(kNew, " ", "_")
		if k == kNew {
			continue
		}
		delete(raw, k)
		raw[kNew] = vs
	}
}

func deleteZero[T comparable](elms []T) []T {
	var zero T
	return slices.DeleteFunc(elms, func(t T) bool { return t == zero })
}
