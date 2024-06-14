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
var ErrBadData = errors.New("bad data")

const delimL, delimR = "{{", "}}"

type Data struct {
	Release       *musicbrainz.Release
	Track         *musicbrainz.Track
	TrackNum      int
	IsCompilation bool
	Ext           string
}

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
	*pf = Format{*tmpl, root}
	return nil
}

func (pf *Format) Root() string {
	return pf.root
}

func (pf *Format) Execute(data Data) (string, error) {
	if len(pf.tt.Templates()) == 0 {
		return "", fmt.Errorf("not initialised yet")
	}

	// make sure these are not used
	data.Track.Number = ""
	data.Track.Position = -1

	var buff strings.Builder
	if err := pf.tt.Execute(&buff, data); err != nil {
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

func validate(f Format) error {
	release := func(artist, name string) *musicbrainz.Release {
		var release musicbrainz.Release
		release.Title = name
		release.Artists = append(release.Artists, musicbrainz.ArtistCredit{Name: artist, Artist: musicbrainz.Artist{Name: artist}})
		return &release
	}
	track := func(title string) *musicbrainz.Track {
		return &musicbrainz.Track{Title: title}
	}
	compare := func(d1, d2 Data) (bool, error) {
		path1, err := f.Execute(d1)
		if err != nil {
			return false, fmt.Errorf("execute data 1: %w", err)
		}
		path2, err := f.Execute(d2)
		if err != nil {
			return false, fmt.Errorf("execute data 2: %w", err)
		}
		return path1 == path2, nil
	}

	eq, err := compare(
		Data{release("ar", "release-same"), track("track 1"), 1, false, ""},
		Data{release("ar", "release-same"), track("track 2"), 2, false, ""},
	)
	if err != nil {
		return err
	}
	if eq {
		return fmt.Errorf("%w: two different tracks have the same path", ErrAmbiguousFormat)
	}

	eq, err = compare(
		Data{release("ar", "release 1"), track("track-same"), 1, false, ""},
		Data{release("ar", "release 2"), track("track-same"), 1, false, ""},
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
	"sort":     func(strings []string) []string { sort.Strings(strings); return strings },
	"safepath": func(p string) string { return fileutil.SafePath(p) },

	"flatTracks": musicbrainz.FlatTracks,

	"artists":             musicbrainz.ArtistsNames,
	"artistsString":       musicbrainz.ArtistsString,
	"artistsCredit":       musicbrainz.ArtistsCreditNames,
	"artistsCreditString": musicbrainz.ArtistsCreditString,

	"disambig": func(r musicbrainz.Release) string {
		var parts []string
		if r.ReleaseGroup.Disambiguation != "" {
			parts = append(parts, r.ReleaseGroup.Disambiguation)
		}
		if r.Disambiguation != "" {
			parts = append(parts, r.Disambiguation)
		}
		return strings.Join(parts, ", ")
	},
}
