package musicbrainz

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/araddon/dateparse"
	"go.senan.xyz/wrtag/clientutil"
)

var ErrNoResults = errors.New("no results")

type MBClient struct {
	BaseURL   string
	RateLimit time.Duration

	initOnce   sync.Once
	HTTPClient *http.Client
}

func (c *MBClient) GetRelease(ctx context.Context, mbid string) (*Release, error) {
	urlV := url.Values{}
	urlV.Set("fmt", "json")
	urlV.Set("inc", "recordings+artist-credits+labels+release-groups+genres+aliases+recording-level-rels+artist-rels+isrcs")

	url, _ := url.Parse(joinPath(c.BaseURL, "release", mbid))
	url.RawQuery = urlV.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)

	var sr Release
	if err := c.request(ctx, req, &sr); err != nil {
		return nil, fmt.Errorf("request release: %w", err)
	}

	return &sr, nil
}

type ReleaseQuery struct {
	MBReleaseID      string
	MBArtistID       string
	MBReleaseGroupID string

	Release      string
	Artist       string
	Date         time.Time
	Format       string
	Label        string
	CatalogueNum string
	Barcode      string
	NumTracks    int
}

func (c *MBClient) SearchRelease(ctx context.Context, q ReleaseQuery) (*Release, error) {
	if uuidExpr.MatchString(q.MBReleaseID) {
		release, err := c.GetRelease(ctx, q.MBReleaseID)
		if err != nil {
			return nil, fmt.Errorf("get direct release: %w", err)
		}
		return release, nil
	}

	// https://beta.musicbrainz.org/doc/MusicBrainz_API/Search#Release

	var params []string
	if q.MBArtistID != "" {
		params = append(params, field("arid", q.MBArtistID))
	}
	if q.MBReleaseGroupID != "" {
		params = append(params, field("rgid", q.MBReleaseGroupID))
	}
	if q.Release != "" {
		params = append(params, field("release", strings.ToLower(q.Release)))
	}
	if q.Artist != "" {
		params = append(params, field("artist", strings.ToLower(q.Artist)))
	}
	if !q.Date.IsZero() {
		params = append(params, field("date", q.Date.Format(time.DateOnly)))
	}
	if q.Format != "" {
		params = append(params, field("format", strings.ToLower(q.Format)))
	}
	if q.Label != "" {
		params = append(params, field("label", strings.ToLower(q.Label)))
	}
	if q.CatalogueNum != "" {
		params = append(params, field("catno", strings.ToLower(q.CatalogueNum)))
	}
	if q.Barcode != "" {
		params = append(params, field("barcode", q.Barcode))
	}
	if q.NumTracks > 0 {
		params = append(params, field("tracks", q.NumTracks))
	}
	if len(params) == 0 {
		return nil, ErrNoResults
	}

	queryStr := strings.Join(params, " ")

	urlV := url.Values{}
	urlV.Set("fmt", "json")
	urlV.Set("limit", "1")
	urlV.Set("query", queryStr)

	url, _ := url.Parse(joinPath(c.BaseURL, "release"))
	url.RawQuery = urlV.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)

	var sr struct {
		Releases []struct {
			ID    string `json:"id"`
			Score int    `json:"score"`
		} `json:"releases"`
	}
	if err := c.request(ctx, req, &sr); err != nil {
		return nil, fmt.Errorf("request release: %w", err)
	}
	if len(sr.Releases) == 0 || sr.Releases[0].ID == "" {
		return nil, ErrNoResults
	}
	releaseKey := sr.Releases[0]

	release, err := c.GetRelease(ctx, releaseKey.ID)
	if err != nil {
		return nil, fmt.Errorf("get release by mbid %s: %w", releaseKey.ID, err)
	}

	return release, nil
}

func (c *MBClient) request(ctx context.Context, r *http.Request, dest any) error {
	c.initOnce.Do(func() {
		c.HTTPClient = clientutil.Wrap(c.HTTPClient, clientutil.Chain(
			clientutil.WithRateLimit(c.RateLimit),
		))
	})

	r = r.WithContext(ctx)
	resp, err := c.HTTPClient.Do(r)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("musicbrainz returned non 2xx: %w", StatusError(resp.StatusCode))
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type ArtistCredit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     Artist `json:"artist"`
}

type Artist struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	TypeID         string  `json:"type-id"`
	SortName       string  `json:"sort-name"`
	Type           string  `json:"type"`
	Genres         []Genre `json:"genres"`
	Disambiguation string  `json:"disambiguation"`
	Aliases        []Alias `json:"aliases"`
}

type Genre struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Disambiguation string `json:"disambiguation"`
	Count          int    `json:"count"`
}

type Alias struct {
	Locale   string `json:"locale"`
	Primary  bool   `json:"primary"`
	Ended    bool   `json:"ended"`
	Type     string `json:"type"`
	TypeID   string `json:"type-id"`
	Begin    any    `json:"begin"`
	Name     string `json:"name"`
	End      any    `json:"end"`
	SortName string `json:"sort-name"`
}

type Relation struct {
	TargetType      string   `json:"target-type"`
	TypeID          string   `json:"type-id"`
	SourceCredit    string   `json:"source-credit"`
	TargetCredit    string   `json:"target-credit"`
	AttributeIDs    struct{} `json:"attribute-ids"`
	Direction       string   `json:"direction"`
	Attributes      []any    `json:"attributes"`
	Type            string   `json:"type"`
	AttributeValues struct{} `json:"attribute-values"`
	Begin           any      `json:"begin"`
	End             any      `json:"end"`
	Ended           bool     `json:"ended"`
	Artist          Artist   `json:"artist"`
	Work            Work     `json:"work"`
}

type Track struct {
	ID        string `json:"id"`
	Length    int    `json:"length"`
	Recording struct {
		FirstReleaseDate string         `json:"first-release-date"`
		Genres           []Genre        `json:"genres"`
		Video            bool           `json:"video"`
		Disambiguation   string         `json:"disambiguation"`
		ID               string         `json:"id"`
		Length           int            `json:"length"`
		Title            string         `json:"title"`
		Artists          []ArtistCredit `json:"artist-credit"`
		Relations        []Relation     `json:"relations"`
		ISRCs            []string       `json:"isrcs"`
	} `json:"recording"`
	Number   string         `json:"number"`
	Position int            `json:"position"`
	Title    string         `json:"title"`
	Artists  []ArtistCredit `json:"artist-credit"`
}

type Work struct {
	Type           any        `json:"type"`
	Relations      []Relation `json:"relations"`
	Language       string     `json:"language"`
	Languages      []string   `json:"languages"`
	Disambiguation string     `json:"disambiguation"`
	Title          string     `json:"title"`
	Iswcs          []any      `json:"iswcs"`
	ID             string     `json:"id"`
	Attributes     []any      `json:"attributes"`
	TypeID         any        `json:"type-id"`
}

type Media struct {
	TrackOffset int     `json:"track-offset"`
	TrackCount  int     `json:"track-count"`
	Tracks      []Track `json:"tracks"`
	Pregap      *Track  `json:"pregap,omitempty"`
	Format      string  `json:"format"`
	FormatID    string  `json:"format-id"`
	Title       string  `json:"title"`
	Position    int     `json:"position"`
}

type LabelInfo struct {
	Label         Label  `json:"label"`
	CatalogNumber string `json:"catalog-number"`
}

type Label struct {
	LabelCode      any     `json:"label-code"`
	Type           string  `json:"type"`
	Disambiguation string  `json:"disambiguation"`
	SortName       string  `json:"sort-name"`
	TypeID         string  `json:"type-id"`
	Genres         []Genre `json:"genres"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
}

type Release struct {
	Title              string `json:"title"`
	ID                 string `json:"id"`
	TextRepresentation struct {
		Language string `json:"language"`
		Script   string `json:"script"`
	} `json:"text-representation"`
	StatusID        string  `json:"status-id"`
	Asin            string  `json:"asin"`
	Genres          []Genre `json:"genres"`
	Country         string  `json:"country"`
	Barcode         string  `json:"barcode"`
	Disambiguation  string  `json:"disambiguation"`
	Packaging       string  `json:"packaging"`
	CoverArtArchive struct {
		Artwork  bool `json:"artwork"`
		Front    bool `json:"front"`
		Darkened bool `json:"darkened"`
		Back     bool `json:"back"`
		Count    int  `json:"count"`
	} `json:"cover-art-archive"`
	Artists       []ArtistCredit `json:"artist-credit"`
	Date          AnyTime        `json:"date"`
	Quality       string         `json:"quality"`
	Media         []Media        `json:"media"`
	Status        string         `json:"status"`
	ReleaseGroup  ReleaseGroup   `json:"release-group"`
	ReleaseEvents []struct {
		Area struct {
			ID             string   `json:"id"`
			Name           string   `json:"name"`
			Iso31661Codes  []string `json:"iso-3166-1-codes"`
			TypeID         any      `json:"type-id"`
			SortName       string   `json:"sort-name"`
			Disambiguation string   `json:"disambiguation"`
			Type           any      `json:"type"`
		} `json:"area"`
		Date AnyTime `json:"date"`
	} `json:"release-events"`
	PackagingID string      `json:"packaging-id"`
	LabelInfo   []LabelInfo `json:"label-info"`
}

type ReleaseGroup struct {
	FirstReleaseDate AnyTime                     `json:"first-release-date"`
	Genres           []Genre                     `json:"genres"`
	PrimaryTypeID    string                      `json:"primary-type-id"`
	Disambiguation   string                      `json:"disambiguation"`
	Artists          []ArtistCredit              `json:"artist-credit"`
	SecondaryTypeIDs []any                       `json:"secondary-type-ids"`
	PrimaryType      ReleaseGroupPrimaryType     `json:"primary-type"`
	ID               string                      `json:"id"`
	SecondaryTypes   []ReleaseGroupSecondaryType `json:"secondary-types"`
	Title            string                      `json:"title"`
}

type ReleaseGroupPrimaryType string

const (
	Album     ReleaseGroupPrimaryType = "Album"
	Single    ReleaseGroupPrimaryType = "Single"
	EP        ReleaseGroupPrimaryType = "EP"
	Broadcast ReleaseGroupPrimaryType = "Broadcast"
	Other     ReleaseGroupPrimaryType = "Other"
)

type ReleaseGroupSecondaryType string

const (
	AudioDrama     ReleaseGroupSecondaryType = "Audio drama"
	Audiobook      ReleaseGroupSecondaryType = "Audiobook"
	Compilation    ReleaseGroupSecondaryType = "Compilation"
	Demo           ReleaseGroupSecondaryType = "Demo"
	DJMix          ReleaseGroupSecondaryType = "DJ-mix"
	FieldRecording ReleaseGroupSecondaryType = "Field recording"
	Interview      ReleaseGroupSecondaryType = "Interview"
	Live           ReleaseGroupSecondaryType = "Live"
	MixtapeStreet  ReleaseGroupSecondaryType = "Mixtape/Street"
	Remix          ReleaseGroupSecondaryType = "Remix"
	Soundtrack     ReleaseGroupSecondaryType = "Soundtrack"
	Spokenword     ReleaseGroupSecondaryType = "Spokenword"
)

func ArtistsNames(credits []ArtistCredit) []string {
	var r []string
	for _, c := range credits {
		r = append(r, c.Artist.Name)
	}
	return r
}

func ArtistsString(credits []ArtistCredit) string {
	var sb strings.Builder
	for _, c := range credits {
		sb.WriteString(c.Artist.Name)
		sb.WriteString(c.JoinPhrase)
	}
	return sb.String()
}

func ArtistsEnNames(credits []ArtistCredit) []string {
	var r []string
	for _, c := range credits {
		r = append(r, artistEnName(c.Artist))
	}
	return r
}

func ArtistsEnString(credits []ArtistCredit) string {
	var sb strings.Builder
	for _, c := range credits {
		sb.WriteString(artistEnName(c.Artist))
		sb.WriteString(c.JoinPhrase)
	}
	return sb.String()
}

func ArtistsCreditNames(credits []ArtistCredit) []string {
	var r []string
	for _, c := range credits {
		r = append(r, c.Name)
	}
	return r
}

func ArtistsCreditString(credits []ArtistCredit) string {
	var sb strings.Builder
	for _, c := range credits {
		sb.WriteString(c.Name)
		sb.WriteString(c.JoinPhrase)
	}
	return sb.String()
}

func ArtistsSortNames(sorts []ArtistCredit) []string {
	var r []string
	for _, c := range sorts {
		r = append(r, c.Artist.SortName)
	}
	return r
}

func ArtistsSortString(sorts []ArtistCredit) string {
	var sb strings.Builder
	for _, c := range sorts {
		sb.WriteString(c.Artist.SortName)
		sb.WriteString(c.JoinPhrase)
	}
	return sb.String()
}

const enLocale = "en"

func artistEnName(artist Artist) string {
	for _, a := range artist.Aliases {
		if a.Locale == enLocale {
			return a.Name
		}
	}
	return artist.Name
}

// https://musicbrainz.org/artist/89ad4ac3-39f7-470e-963a-56509c5463
// https://musicbrainz.org/tag/special%20purpose
const variousArtistsMBID = "89ad4ac3-39f7-470e-963a-56509c546377"

func IsCompilation(rg ReleaseGroup) bool {
	if hasComp := slices.Contains(rg.SecondaryTypes, Compilation); hasComp {
		return true
	}
	if hasVA := slices.ContainsFunc(rg.Artists, func(ac ArtistCredit) bool {
		return ac.Artist.ID == variousArtistsMBID
	}); hasVA {
		return true
	}
	return false
}

func FlatTracks(media []Media) []Track {
	var tracks []Track
	for _, media := range media {
		if strings.Contains(media.Format, "DVD") {
			// not supported for now
			continue
		}
		if media.Pregap != nil {
			tracks = append(tracks, *media.Pregap)
		}
		for _, track := range media.Tracks {
			if track.Recording.Video {
				continue
			}
			tracks = append(tracks, track)
		}
	}
	return tracks
}

type GenreInfo struct {
	Name  string
	Count uint
}

func AnyGenres(release *Release) (genres []Genre) {
	defer func() {
		genres = mergeAndSortGenres(genres)
	}()

	// try release and artist first
	genres = append(genres, release.Genres...)
	genres = append(genres, release.ReleaseGroup.Genres...)
	for _, t := range FlatTracks(release.Media) {
		genres = append(genres, t.Recording.Genres...)
	}

	// add some artist genres too
	for _, a := range release.Artists {
		genres = append(genres, a.Artist.Genres...)
	}
	for _, a := range release.ReleaseGroup.Artists {
		genres = append(genres, a.Artist.Genres...)
	}
	if len(genres) > 0 {
		return genres
	}

	// fallback to label
	for _, l := range release.LabelInfo {
		genres = append(genres, l.Label.Genres...)
	}
	return genres
}

func AnyLabelInfo(release *Release) LabelInfo {
	if len(release.LabelInfo) > 0 {
		return release.LabelInfo[0]
	}
	return LabelInfo{}
}

type AnyTime struct {
	time.Time
}

func (at *AnyTime) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" {
		return nil
	}
	var err error
	at.Time, err = dateparse.ParseAny(str)
	if err != nil {
		return fmt.Errorf("parse any: %w", err)
	}
	return nil
}

func mergeAndSortGenres(genres []Genre) []Genre {
	genreIDs := map[string]Genre{}
	for _, g := range genres {
		if _, ok := genreIDs[g.ID]; !ok {
			genreIDs[g.ID] = g
			continue
		}
		f := genreIDs[g.ID]
		f.Count += g.Count
		genreIDs[g.ID] = f
	}
	var out []Genre
	for _, g := range genreIDs {
		out = append(out, g)
	}
	slices.SortFunc(out, func(a, b Genre) int {
		return cmp.Or(
			cmp.Compare(b.Count, a.Count),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return out
}

// https://lucene.apache.org/core/7_7_2/queryparser/org/apache/lucene/queryparser/classic/package-summary.html#Escaping_Special_Characters
var escapeLucene *strings.Replacer

func init() {
	var pairs []string
	for _, c := range []string{`&&`, `||`, `+`, `-`, `!`, `(`, `)`, `{`, `}`, `[`, `]`, `^`, `"`, `~`, `*`, `?`, `:`, `\`, `/`} {
		pairs = append(pairs, c, `\`+c)
	}
	escapeLucene = strings.NewReplacer(pairs...)
}

func field(k string, v any) string {
	vstr := fmt.Sprint(v)
	vstr = escapeLucene.Replace(vstr)
	return fmt.Sprintf("%s:(%v)", k, vstr)
}

func joinPath(base string, p ...string) string {
	r, _ := url.JoinPath(base, p...)
	return r
}

var uuidExpr = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
