package pathformat

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"

	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/musicbrainz"
)

var ErrInvalidFormat = errors.New("invalid format")
var ErrAmbiguousFormat = errors.New("ambiguous format")
var ErrBadRelease = errors.New("bad release")
var ErrBadData = errors.New("bad data")

const delimL, delimR = "{{", "}}"

type Format struct {
	tt   texttemplate.Template
	root string
}

func (pf *Format) Parse(str string) error {
	str = strings.TrimSpace(str)
	if str == "" {
		return fmt.Errorf("%w: empty format", ErrInvalidFormat)
	}
	if strings.Count(str, string(filepath.Separator)) < 2 {
		return fmt.Errorf("%w: not enough path segments", ErrInvalidFormat)
	}
	tmpl, err := texttemplate.
		New("template").
		Funcs(funcMap).
		Delims(delimL, delimR).
		Parse(str)
	if err != nil {
		return fmt.Errorf("%w: %w", err, ErrInvalidFormat)
	}
	if err := validate(Format{*tmpl, ""}); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	root, _, ok := strings.Cut(str, delimL)
	if !ok {
		return fmt.Errorf("find root: %w", ErrInvalidFormat)
	}
	root = filepath.Clean(root)
	if dir, err := filepath.EvalSymlinks(root); err == nil {
		root = dir
	}
	*pf = Format{*tmpl, root}
	return nil
}

func (pf *Format) Root() string {
	return pf.root
}

func (pf *Format) Execute(release *musicbrainz.Release, index int, ext string) (string, error) {
	if len(pf.tt.Templates()) == 0 {
		return "", errors.New("not initialised yet")
	}

	flatTracks := musicbrainz.FlatTracks(release.Media)
	if len(flatTracks) == 0 {
		return "", errors.New("release has no tracks")
	}

	var anyPos bool
	for _, t := range flatTracks {
		if t.Position > 0 {
			anyPos = true
			break
		}
	}
	if !anyPos {
		return "", fmt.Errorf("%w: all tracks have 0 position", ErrBadRelease)
	}

	var numPregap int
	for _, t := range flatTracks[:index] {
		if t.Position == 0 {
			numPregap++
		}
	}

	var d Data
	d.Release = *release
	d.Ext = ext
	d.Track = flatTracks[index]
	d.Tracks = flatTracks
	if d.Track.Position > 0 {
		d.TrackNum = index + 1 - numPregap // non pregap track, give it a 1-indexed num
	}
	d.IsCompilation = musicbrainz.IsCompilation(release.ReleaseGroup)
	{
		var parts []string
		if release.ReleaseGroup.Disambiguation != "" {
			parts = append(parts, release.ReleaseGroup.Disambiguation)
		}
		if release.Disambiguation != "" {
			parts = append(parts, release.Disambiguation)
		}
		d.ReleaseDisambiguation = strings.Join(parts, ", ")
	}

	// make sure these are not used
	d.Track.Number = ""
	d.Track.Position = -1

	var buff strings.Builder
	if err := pf.tt.Execute(&buff, d); err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	destPath := buff.String()

	if strings.HasSuffix(destPath, string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q: output path has trailing slash", ErrBadData, destPath)
	}
	if strings.Contains(destPath, strings.Repeat(string(filepath.Separator), 2)) {
		return "", fmt.Errorf("%w: %q: output path would contain adjacent filepath seperators", ErrBadData, destPath)
	}
	destPath = filepath.Clean(destPath)
	return destPath, nil
}

type Data struct {
	Release               musicbrainz.Release
	ReleaseDisambiguation string
	Ext                   string
	Track                 musicbrainz.Track
	Tracks                []musicbrainz.Track
	TrackNum              int
	IsCompilation         bool
}

func validate(f Format) error {
	release := func(artist, name string, tracks ...string) *musicbrainz.Release { //nolint:unparam
		var release musicbrainz.Release
		release.Title = name
		release.Artists = append(release.Artists, musicbrainz.ArtistCredit{Name: artist, Artist: musicbrainz.Artist{Name: artist}})

		var media musicbrainz.Media
		for i, t := range tracks {
			media.Tracks = append(media.Tracks, musicbrainz.Track{
				Title:    t,
				Position: i + 1,
			})
			media.TrackCount++
		}
		release.Media = append(release.Media, media)
		return &release
	}
	compare := func(r1 *musicbrainz.Release, i1 int, r2 *musicbrainz.Release, i2 int) (bool, error) {
		path1, err := f.Execute(r1, i1, "")
		if err != nil {
			return false, fmt.Errorf("execute data 1: %w", err)
		}
		path2, err := f.Execute(r2, i2, "")
		if err != nil {
			return false, fmt.Errorf("execute data 2: %w", err)
		}
		return path1 == path2, nil
	}

	eq, err := compare(
		release("ar", "release-same", "track 1", "track 1"), 0,
		release("ar", "release-same", "track 2", "track 2"), 1,
	)
	if err != nil {
		return err
	}
	if eq {
		return fmt.Errorf("%w: two different tracks have the same path", ErrAmbiguousFormat)
	}

	eq, err = compare(
		release("ar", "release 1", "track same"), 0,
		release("ar", "release 2", "track same"), 0,
	)
	if err != nil {
		return err
	}
	if eq {
		return fmt.Errorf("%w: two releases with the same track info results in the same path", ErrAmbiguousFormat)
	}
	return nil
}

var funcMap = texttemplate.FuncMap{
	"join":     func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0":     func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
	"sort":     func(strs []string) []string { sort.Strings(strs); return strs },
	"safepath": func(p string) string { return fileutil.SafePath(p) },

	"artists":             musicbrainz.ArtistsNames,
	"artistsString":       musicbrainz.ArtistsString,
	"artistsEn":           musicbrainz.ArtistsEnNames,
	"artistsEnString":     musicbrainz.ArtistsEnString,
	"artistsCredit":       musicbrainz.ArtistsCreditNames,
	"artistsCreditString": musicbrainz.ArtistsCreditString,
	"artistsSort":         musicbrainz.ArtistsSortNames,
	"artistsSortString":   musicbrainz.ArtistsSortString,

	"the": func(strs []string) []string {
		for i, s := range strs {
			if a, b, ok := strings.Cut(s, " "); ok && (strings.EqualFold(a, "the") || strings.EqualFold(a, "a")) {
				strs[i] = b + ", " + a
			}
		}
		return strs
	},
}
