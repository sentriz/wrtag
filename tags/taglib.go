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
	"sync"
	"time"

	"github.com/araddon/dateparse"
	"github.com/sentriz/audiotags"
)

var ErrWrite = errors.New("error writing tags")

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

//go:generate go run gen_taglist.go -- $GOFILE taglist.gen.go
const (
	Album              = "album"
	AlbumArtist        = "albumartist"         //tag: alts "album_artist"
	AlbumArtists       = "albumartists"        //tag: alts "album_artists"
	AlbumArtistCredit  = "albumartist_credit"  //tag: alts "album_artist_credit"
	AlbumArtistsCredit = "albumartists_credit" //tag: alts "album_artists_credit"
	Date               = "date"                //tag: alts "year"
	OriginalDate       = "originaldate"        //tag: alts "original_year"
	MediaFormat        = "media"
	Label              = "label"
	CatalogueNum       = "catalognumber" //tag: alts "catalognum"
	UPC                = "upc"           //tag: alts "mcn"

	MBReleaseID      = "musicbrainz_albumid"
	MBReleaseGroupID = "musicbrainz_releasegroupid"
	MBAlbumArtistID  = "musicbrainz_albumartistid"
	MBAlbumComment   = "musicbrainz_albumcomment"

	Title         = "title"
	Artist        = "artist"
	Artists       = "artists"
	ArtistCredit  = "artist_credit"  //tag: alts "artistcredit"
	ArtistsCredit = "artists_credit" //tag: alts "artistscredit"
	Genre         = "genre"
	Genres        = "genres"
	TrackNumber   = "tracknumber" //tag: alts "track" "trackc"
	DiscNumber    = "discnumber"

	MBRecordingID = "musicbrainz_trackid"
	MBArtistID    = "musicbrainz_artistid"

	ReplayGainTrackGain = "replaygain_track_gain"
	ReplayGainTrackPeak = "replaygain_track_peak"
	ReplayGainAlbumGain = "replaygain_album_gain"
	ReplayGainAlbumPeak = "replaygain_album_peak"

	Lyrics = "lyrics" //tag: alts "lyrics:description" "uslt:description" "©lyr"
)

func CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

type File struct {
	raw            map[string][]string
	properties     *audiotags.AudioProperties
	propertiesOnce sync.Once
	file           *audiotags.File
	path           string
}

func Read(path string) (*File, error) {
	f, err := audiotags.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	raw := f.ReadTags()
	normalise(raw, alternatives) // tag replacements, case normalisation, etc

	return &File{raw: raw, file: f, path: path}, nil
}

func (f *File) initProperties() {
	f.propertiesOnce.Do(func() {
		f.properties = f.file.ReadAudioProperties()
	})
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
func (f *File) WriteNum(t string, v int)       { f.Write(t, intStr(v)) }
func (f *File) WriteFloat(t string, v float64) { f.Write(t, floatStr(v, 6)) }
func (f *File) WritedB(t string, v float64)    { f.Write(t, floatStr(v, 2)+" dB") }

func (f *File) Clear(t string) { delete(f.raw, t) }
func (f *File) ClearAll()      { clear(f.raw) }
func (f *File) ClearUnknown() {
	for k := range f.raw {
		if _, ok := knownTags[k]; !ok {
			delete(f.raw, k)
		}
	}
}

func (f *File) Length() time.Duration {
	f.initProperties()
	return time.Duration(f.properties.LengthMs) * time.Millisecond
}
func (f *File) Bitrate() int     { f.initProperties(); return f.properties.Bitrate }
func (f *File) SampleRate() int  { f.initProperties(); return f.properties.Samplerate }
func (f *File) NumChannels() int { f.initProperties(); return f.properties.Channels }

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

	before := maps.Clone(f.raw)
	if err := fn(f); err != nil {
		return err
	}

	// try avoid filesystem writes if we can
	if maps.EqualFunc(before, f.raw, slices.Equal) {
		return nil
	}

	if l := slog.Default(); l.Enabled(context.Background(), slog.LevelDebug) {
		pathBase := filepath.Base(path)
		for k := range f.raw {
			if before, after := before[k], f.raw[k]; !slices.Equal(before, after) {
				l.Debug("tag change", "file", pathBase, "key", k, "from", before, "to", after)
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

func floatStr(v float64, p int) string {
	return strconv.FormatFloat(v, 'f', p, 64)
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

func normalise(raw map[string][]string, alternatives map[string]string) {
	for kbad, kgood := range alternatives {
		if _, ok := raw[kgood]; ok {
			continue
		}
		if v, ok := raw[kbad]; ok {
			raw[kgood] = v
			delete(raw, kbad)
			continue
		}
	}
}

func deleteZero[T comparable](elms []T) []T {
	var zero T
	return slices.DeleteFunc(elms, func(t T) bool { return t == zero })
}
