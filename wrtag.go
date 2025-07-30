// Package wrtag provides functionality for music file tagging and organisation.
// It allows for automatic lookup of music metadata from MusicBrainz, tagging files
// with proper metadata, and organising files in a directory structure based on
// a configurable path format.
package wrtag

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/KarpelesLab/reflink"
	"github.com/araddon/dateparse"
	"github.com/argusdusty/treelock"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/natcmp"
	"go.senan.xyz/wrtag/addon"
	"go.senan.xyz/wrtag/coverparse"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/originfile"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/tags"
)

var (
	// ErrScoreTooLow is returned when the match confidence between local tracks and
	// MusicBrainz data is below the required threshold.
	ErrScoreTooLow = errors.New("score too low")

	// ErrTrackCountMismatch is returned when the number of tracks in the local directory
	// doesn't match the number of tracks in the MusicBrainz release.
	ErrTrackCountMismatch = errors.New("track count mismatch")

	// ErrNoTracks is returned when no audio tracks are found in the source directory.
	ErrNoTracks = errors.New("no tracks in dir")

	// ErrNotSortable is returned when the tracks in a directory cannot be reliably sorted
	// due to missing track numbers or numerical identifiers in filenames.
	ErrNotSortable = errors.New("tracks in dir can't be sorted")

	// ErrSelfCopy is returned when attempting to copy a file to itself.
	ErrSelfCopy = errors.New("can't copy self to self")
)

// IsNonFatalError determines whether an error is non-fatal during processing.
// Non-fatal errors include low match scores and track count mismatches.
func IsNonFatalError(err error) bool {
	return errors.Is(err, ErrScoreTooLow) || errors.Is(err, ErrTrackCountMismatch)
}

// The minimum score required for a MusicBrainz match to be considered valid.
const minScore = 95

// Max number of genres to write to a track.
const numTrackGenres = 6

const (
	// thresholdSizeClean is the maximum size (20MB) of a directory that can be
	// automatically cleaned up.
	thresholdSizeClean uint64 = 20 * 1e6 // 20 MB

	// thresholdSizeTrim is the maximum size (3GB) of files that can be automatically
	// trimmed from a destination directory.
	thresholdSizeTrim uint64 = 3000 * 1e6 // 3000 MB
)

// SearchResult contains the results of a MusicBrainz lookup and potential import operation.
type SearchResult struct {
	// Release contains the matched MusicBrainz release data
	Release *musicbrainz.Release

	// Query contains the search parameters used for the MusicBrainz lookup
	Query musicbrainz.ReleaseQuery

	// Score indicates the confidence of the match (0-100)
	Score float64

	// DestDir is the destination directory path where files were or would be placed
	DestDir string

	// Diff contains the differences between local tags and MusicBrainz tags
	Diff []Diff

	// OriginFile contains information from any gazelle-origin file found in the source directory
	OriginFile *originfile.OriginFile
}

// ImportCondition defines the conditions under which a release will be imported.
type ImportCondition uint8

const (
	// HighScore requires the match to have a high confidence score.
	HighScore ImportCondition = iota

	// HighScoreOrMBID accepts either a high score or a matching MusicBrainz ID.
	HighScoreOrMBID

	// Always always imports regardless of score.
	Always
)

// Config contains configuration options for processing music directories.
type Config struct {
	// MusicBrainzClient is used to search and retrieve release data from MusicBrainz
	MusicBrainzClient musicbrainz.MBClient

	// CoverArtArchiveClient is used to retrieve cover art
	CoverArtArchiveClient musicbrainz.CAAClient

	// PathFormat defines the directory structure for organising music files
	PathFormat pathformat.Format

	// DiffWeights defines the relative importance of different tags when calculating match scores
	DiffWeights DiffWeights

	// TagConfig defines options for modifying the default tag set
	TagConfig TagConfig

	// KeepFiles specifies files that should be preserved during processing
	KeepFiles map[string]struct{}

	// Addons are plugins that can perform additional processing after the main import
	Addons []addon.Addon

	// UpgradeCover specifies whether to attempt to replace existing covers with better versions
	UpgradeCover bool
}

// ProcessDir processes a music directory by looking up metadata on MusicBrainz and
// either moving, copying, or reflinking the files to a new location with proper tags.
// It returns a SearchResult containing information about the match and operation.
//
// The srcDir must be an absolute path.
// The cond parameter determines the conditions under which the import will proceed.
// The useMBID parameter can be used to force a specific MusicBrainz release ID.
func ProcessDir(
	ctx context.Context, cfg *Config,
	op FileSystemOperation, srcDir string, cond ImportCondition, useMBID string,
) (*SearchResult, error) {
	if cfg.PathFormat.Root() == "" {
		return nil, errors.New("no path format provided")
	}

	if !filepath.IsAbs(srcDir) {
		panic("src dir not abs") // this is a programmer error for now
	}
	srcDir = filepath.Clean(srcDir)

	cover, pathTags, err := ReadReleaseDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	if len(pathTags) == 0 {
		return nil, ErrNoTracks
	}

	searchTags := pathTags[0].Tags

	var mbid = searchTags.Get(tags.MusicBrainzReleaseID)
	if useMBID != "" {
		mbid = useMBID
	}

	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      mbid,
		MBArtistID:       searchTags.Get(tags.MusicBrainzArtistID),
		MBReleaseGroupID: searchTags.Get(tags.MusicBrainzReleaseGroupID),
		Release:          searchTags.Get(tags.Album),
		Artist:           cmp.Or(searchTags.Get(tags.AlbumArtist), searchTags.Get(tags.Artist)),
		Date:             parseAnyTime(searchTags.Get(tags.Date)),
		Format:           searchTags.Get(tags.MediaFormat),
		Label:            searchTags.Get(tags.Label),
		CatalogueNum:     searchTags.Get(tags.CatalogueNum),
		Barcode:          searchTags.Get(tags.Barcode),
		NumTracks:        len(pathTags),
	}

	// parse https://github.com/x1ppy/gazelle-origin files, if one exists
	originFile, err := originfile.Find(srcDir)
	if err != nil {
		return nil, fmt.Errorf("find origin file: %w", err)
	}

	if mbid == "" {
		if err := extendQueryWithOriginFile(ctx, &query, originFile); err != nil {
			return nil, fmt.Errorf("use origin file: %w", err)
		}
	}

	release, err := cfg.MusicBrainzClient.SearchRelease(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search musicbrainz: %w", err)
	}

	releaseTracks := musicbrainz.FlatTracks(release.Media)

	score, diff := DiffRelease(cfg.DiffWeights, release, releaseTracks, pathTags)

	if len(pathTags) != len(releaseTracks) {
		return &SearchResult{release, query, 0, "", diff, originFile}, fmt.Errorf("%w: %d remote / %d local", ErrTrackCountMismatch, len(releaseTracks), len(pathTags))
	}

	var shouldImport bool
	switch cond {
	case HighScoreOrMBID:
		shouldImport = score >= minScore || mbid != ""
	case HighScore:
		shouldImport = score >= minScore
	case Always:
		shouldImport = true
	}

	if !shouldImport {
		return &SearchResult{release, query, score, "", diff, originFile}, ErrScoreTooLow
	}

	destDir, err := DestDir(&cfg.PathFormat, release)
	if err != nil {
		return nil, fmt.Errorf("gen dest dir: %w", err)
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)
	genres := musicbrainz.AnyGenres(release)

	// calculate new paths
	destPaths := make([]string, 0, len(pathTags))
	for i, pt := range pathTags {
		destPath, err := cfg.PathFormat.Execute(release, i, strings.ToLower(filepath.Ext(pt.Path)))
		if err != nil {
			return nil, fmt.Errorf("create path: %w", err)
		}

		destPaths = append(destPaths, destPath)
	}

	// lock both source and destination directories
	unlock := lockPaths(
		srcDir,
		destDir,
	)

	dc := NewDirContext()

	// move/copy and tag
	for i := range pathTags {
		pt, rt, destPath := pathTags[i], releaseTracks[i], destPaths[i]

		if err := op.ProcessPath(ctx, dc, pt.Path, destPath); err != nil {
			return nil, fmt.Errorf("process path %q: %w", filepath.Base(pt.Path), err)
		}

		var destTags = tags.Tags{}
		WriteRelease(destTags, release, labelInfo, genres, i, &rt)
		ApplyTagConfig(destTags, pt.Tags, cfg.TagConfig)

		if lvl, slog := slog.LevelDebug, slog.Default(); slog.Enabled(ctx, lvl) {
			logTagChanges(ctx, pt.Path, lvl, pt.Tags, destTags)
		}

		if !op.CanModifyDest() {
			continue
		}
		if tags.Equal(pt.Tags, destTags) {
			// try to avoid more io if we can
			continue
		}

		if err := tags.WriteTags(destPath, destTags, tags.Clear); err != nil {
			return nil, fmt.Errorf("write tag file: %w", err)
		}
	}

	if err := processCover(ctx, cfg, op, dc, release, destDir, cover); err != nil {
		return nil, fmt.Errorf("process cover: %w", err)
	}

	// process addons with new files
	if op.CanModifyDest() {
		for _, addon := range cfg.Addons {
			if err := addon.ProcessRelease(ctx, destPaths); err != nil {
				return nil, fmt.Errorf("process addon: %w", err)
			}
		}
	}

	for kf := range cfg.KeepFiles {
		if err := op.ProcessPath(ctx, dc, filepath.Join(srcDir, kf), filepath.Join(destDir, kf)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("process keep file %q: %w", kf, err)
		}
	}

	if err := trimDestDir(ctx, dc, destDir, op.CanModifyDest()); err != nil {
		return nil, fmt.Errorf("trim: %w", err)
	}

	unlock()

	if srcDir != destDir {
		if err := op.PostSource(ctx, dc, cfg.PathFormat.Root(), srcDir); err != nil {
			return nil, fmt.Errorf("clean: %w", err)
		}
	}

	return &SearchResult{release, query, score, destDir, diff, originFile}, nil
}

// PathTags associates a file path with its tags.
type PathTags struct {
	// Path is the file path
	Path string

	// Tags contains the audio file's metadata tags
	tags.Tags
}

// ReadReleaseDir reads a directory containing music files and extracts tags from each file.
// It returns the path to the cover image (if found) and a slice of PathTags for each audio file.
// Files are sorted by disc number, directory, track number, and finally path.
func ReadReleaseDir(dirPath string) (string, []PathTags, error) {
	mainPaths, err := fileutil.GlobDir(dirPath, "*")
	if err != nil {
		return "", nil, fmt.Errorf("glob dir: %w", err)
	}
	discPaths, err := fileutil.GlobDir(dirPath, "*/*") // recurse once for any disc1/ disc2/ dirs
	if err != nil {
		return "", nil, fmt.Errorf("glob dir for discs: %w", err)
	}

	var cover string
	var pathTags []PathTags

	paths := append(mainPaths, discPaths...)
	for _, path := range paths {
		if coverparse.IsCover(path) {
			cover = coverparse.BestBetween(cover, path)
			continue
		}

		if tags.CanRead(path) {
			tags, err := tags.ReadTags(path)
			if err != nil {
				return "", nil, fmt.Errorf("read track: %w", err)
			}
			pathTags = append(pathTags, PathTags{
				Path: path,
				Tags: tags,
			})
			continue
		}
	}

	if len(pathTags) == 0 {
		return "", nil, ErrNoTracks
	}

	{
		// validate we aren't accidentally importing something like an artist folder, which may look
		// like a multi disc album to us, but will have all its tracks in one subdirectory
		discDirs := map[string]struct{}{}
		for _, pt := range pathTags {
			discDirs[filepath.Dir(pt.Path)] = struct{}{}
		}
		if len(discDirs) == 1 && filepath.Dir(pathTags[0].Path) != filepath.Clean(dirPath) {
			return "", nil, fmt.Errorf("validate tree: %w", ErrNoTracks)
		}
	}

	if len(pathTags) >= 2 {
		// validate that we have track numbers, or track numbers in filenames to sort on. if we don't any
		// then releases that consist only of untitled tracks may get mixed up
		var haveNum, havePath = true, true
		for _, pt := range pathTags {
			if haveNum && pt.Get(tags.TrackNumber) == "" {
				haveNum = false
			}
			if havePath && !strings.ContainsFunc(filepath.Base(pt.Path), func(r rune) bool { return '0' <= r && r <= '9' }) {
				havePath = false
			}
		}
		if !haveNum && !havePath {
			return "", nil, fmt.Errorf("no track numbers or numbers in filenames present: %w", ErrNotSortable)
		}
	}

	slices.SortFunc(pathTags, func(a, b PathTags) int {
		return cmp.Or(
			natcmp.Compare(a.Get(tags.DiscNumber), b.Get(tags.DiscNumber)),   // disc numbers like "1", "2", "disc 1", "disc 10"
			natcmp.Compare(filepath.Dir(a.Path), filepath.Dir(b.Path)),       // might have disc folders instead of tags
			natcmp.Compare(a.Get(tags.TrackNumber), b.Get(tags.TrackNumber)), // track numbers, could be "A1" "B1" "1" "10" "100" "1/10" "2/10"
			natcmp.Compare(a.Path, b.Path),                                   // fallback to paths
		)
	})

	return cover, pathTags, nil
}

// DestDir generates the destination directory path for a release based on the given path format.
func DestDir(pathFormat *pathformat.Format, release *musicbrainz.Release) (string, error) {
	path, err := pathFormat.Execute(release, 0, ".eg")
	if err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	dir := filepath.Dir(path)
	dir = filepath.Clean(dir)
	return dir, nil
}

// WriteRelease populates a Tags structure with metadata from a MusicBrainz release and track.
// It writes both album-level tags (release title, artists, dates, labels) and track-level tags
// (track title, artists, track number) to the provided Tags instance.
func WriteRelease(
	t tags.Tags,
	release *musicbrainz.Release, labelInfo musicbrainz.LabelInfo, genres []musicbrainz.Genre,
	i int, trk *musicbrainz.Track,
) {
	formatDate := func(d time.Time) string {
		if d.IsZero() {
			return ""
		}
		return d.Format(time.DateOnly)
	}
	formatBool := func(b bool) string {
		if !b {
			return ""
		}
		return "1"
	}

	genreNames := make([]string, 0, numTrackGenres)
	for _, g := range genres[:min(6, len(genres))] {
		genreNames = append(genreNames, g.Name)
	}

	disambiguationParts := trimZero(release.ReleaseGroup.Disambiguation, release.Disambiguation)
	disambiguation := strings.Join(disambiguationParts, ", ")

	var remixers, remixersCredit []string
	for _, r := range trk.Recording.Relations {
		if r.Artist.ID != "" && r.Type == "remixer" {
			remixers = append(remixers, r.Artist.Name)
			remixersCredit = append(remixersCredit, cmp.Or(r.TargetCredit, r.Artist.Name))
		}
	}

	var composers, composersCredit []string
	for _, rel := range trk.Recording.Relations {
		for _, rel := range rel.Work.Relations {
			if rel.Artist.ID != "" && rel.Type == "composer" {
				composers = append(composers, rel.Artist.Name)
				composersCredit = append(composersCredit, cmp.Or(rel.TargetCredit, rel.Artist.Name))
			}
		}
	}

	// t.Set(x, trimZero(y)...) so that we clear out tags with no value from the map

	t.Set(tags.Album, trimZero(release.Title)...)
	t.Set(tags.AlbumArtist, trimZero(musicbrainz.ArtistsString(release.Artists))...)
	t.Set(tags.AlbumArtists, trimZero(musicbrainz.ArtistsNames(release.Artists)...)...)
	t.Set(tags.AlbumArtistCredit, trimZero(musicbrainz.ArtistsCreditString(release.Artists))...)
	t.Set(tags.AlbumArtistsCredit, trimZero(musicbrainz.ArtistsCreditNames(release.Artists)...)...)
	t.Set(tags.Date, trimZero(formatDate(release.Date.Time))...)
	t.Set(tags.OriginalDate, trimZero(formatDate(release.ReleaseGroup.FirstReleaseDate.Time))...)
	t.Set(tags.MediaFormat, trimZero(release.Media[0].Format)...)
	t.Set(tags.Label, trimZero(labelInfo.Label.Name)...)
	t.Set(tags.CatalogueNum, trimZero(labelInfo.CatalogNumber)...)
	t.Set(tags.Barcode, trimZero(release.Barcode)...)
	t.Set(tags.Compilation, trimZero(formatBool(musicbrainz.IsCompilation(release.ReleaseGroup)))...)
	t.Set(tags.ReleaseType, trimZero(strings.ToLower(string(release.ReleaseGroup.PrimaryType)))...)

	t.Set(tags.MusicBrainzReleaseID, trimZero(release.ID)...)
	t.Set(tags.MusicBrainzReleaseGroupID, trimZero(release.ReleaseGroup.ID)...)
	t.Set(tags.MusicBrainzAlbumArtistID, trimZero(mapFunc(release.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)...)
	t.Set(tags.MusicBrainzAlbumComment, trimZero(disambiguation)...)

	t.Set(tags.Title, trimZero(trk.Title)...)
	t.Set(tags.Artist, trimZero(musicbrainz.ArtistsString(trk.Artists))...)
	t.Set(tags.Artists, trimZero(musicbrainz.ArtistsNames(trk.Artists)...)...)
	t.Set(tags.ArtistCredit, trimZero(musicbrainz.ArtistsCreditString(trk.Artists))...)
	t.Set(tags.ArtistsCredit, trimZero(musicbrainz.ArtistsCreditNames(trk.Artists)...)...)
	t.Set(tags.Genre, trimZero(cmp.Or(genreNames...))...)
	t.Set(tags.Genres, trimZero(genreNames...)...)
	t.Set(tags.TrackNumber, trimZero(strconv.Itoa(i+1))...)
	t.Set(tags.DiscNumber, trimZero(strconv.Itoa(1))...)

	t.Set(tags.ISRC, trimZero(trk.Recording.ISRCs...)...)

	t.Set(tags.Remixer, trimZero(strings.Join(remixers, ", "))...)
	t.Set(tags.Remixers, trimZero(remixers...)...)
	t.Set(tags.RemixerCredit, trimZero(strings.Join(remixersCredit, ", "))...)
	t.Set(tags.RemixersCredit, trimZero(remixersCredit...)...)

	t.Set(tags.Composer, trimZero(strings.Join(composers, ", "))...)
	t.Set(tags.Composers, trimZero(composers...)...)
	t.Set(tags.ComposerCredit, trimZero(strings.Join(composersCredit, ", "))...)
	t.Set(tags.ComposersCredit, trimZero(composersCredit...)...)

	t.Set(tags.MusicBrainzRecordingID, trimZero(trk.Recording.ID)...)
	t.Set(tags.MusicBrainzTrackID, trimZero(trk.ID)...)
	t.Set(tags.MusicBrainzArtistID, trimZero(mapFunc(trk.Artists, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.ID })...)...)
}

// Diff represents a comparison between two tag values, showing the differences
// using diff-match-patch format for visualization.
type Diff struct {
	// Field is the name of the tag field being compared
	Field string
	// Before contains the diff segments for the original value
	Before, After []dmp.Diff
	// Equal indicates whether the two values are identical
	Equal bool
}

// DiffWeights maps tag field names to their relative importance when calculating match scores.
// Higher weights make differences in those fields have greater impact on the overall score.
type DiffWeights map[string]float64

// DiffRelease compares local tag files against a MusicBrainz release and calculates a match score.
// It returns a score (0-100) indicating match confidence and detailed diffs for each compared field.
// The weights parameter allows customizing the importance of different tag fields in the score calculation.
func DiffRelease[T interface{ Get(string) string }](weights DiffWeights, release *musicbrainz.Release, tracks []musicbrainz.Track, tagFiles []T) (float64, []Diff) {
	if len(tracks) == 0 {
		return 0, nil
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)

	var score float64
	diff := Differ(&score)

	weight := func(t string) float64 {
		if w, ok := weights[t]; ok {
			return w
		}
		return 1
	}

	var diffs []Diff
	{
		tf := tagFiles[0]
		diffs = append(diffs,
			diff(weight("release"), "release", tf.Get(tags.Album), release.Title),
			diff(weight("artist"), "artist", tf.Get(tags.AlbumArtist), musicbrainz.ArtistsString(release.Artists)),
			diff(weight("label"), "label", tf.Get(tags.Label), labelInfo.Label.Name),
			diff(weight("catalogue num"), "catalogue num", tf.Get(tags.CatalogueNum), labelInfo.CatalogNumber),
			diff(weight("barcode"), "barcode", tf.Get(tags.Barcode), release.Barcode),
			diff(weight("media format"), "media format", tf.Get(tags.MediaFormat), release.Media[0].Format),
		)
	}

	for i := range max(len(tagFiles), len(tracks)) {
		var a, b string
		if i < len(tagFiles) {
			a = strings.Join(trimZero(tagFiles[i].Get(tags.Artist), tagFiles[i].Get(tags.Title)), " – ")
		}
		if i < len(tracks) {
			b = strings.Join(trimZero(musicbrainz.ArtistsString(tracks[i].Artists), tracks[i].Title), " – ")
		}
		diffs = append(diffs, diff(weight("track"), fmt.Sprintf("track %d", i+1), a, b))
	}

	return score, diffs
}

var (
	dm = dmp.New()
)

// Differ creates a difference function that compares two strings and updates a running score.
// The returned function calculates text differences and accumulates weighted distances for scoring.
func Differ(score *float64) func(weight float64, field string, a, b string) Diff {
	var total float64
	var totalDist float64

	return func(w float64, field, a, b string) Diff {
		diffs := dm.DiffMain(a, b, false)

		var d Diff
		d.Field = field
		d.Before = filterFunc(diffs, func(d dmp.Diff) bool { return d.Type <= dmp.DiffEqual })
		d.After = filterFunc(diffs, func(d dmp.Diff) bool { return d.Type >= dmp.DiffEqual })
		d.Equal = a == b

		if a == "" || b == "" {
			return d
		}

		// separate, norm diff for score calculation. only if we have both fields
		aNorm, bNorm := diffNormText(a), diffNormText(b)

		diffs = dm.DiffMain(aNorm, bNorm, false)
		totalDist += float64(dm.DiffLevenshtein(diffs)) * w
		total += float64(max(len([]rune(aNorm)), len([]rune(bNorm))))

		*score = 100 - (totalDist * 100 / total)

		return d
	}
}

func diffNormText(input string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
		if unicode.IsNumber(r) {
			return r
		}
		return -1
	}, input)
}

// TagConfig defines which tags to preserve from source files and which to remove.
// This allows fine-tuning of the tagging process beyond the default behavior.
type TagConfig struct {
	// Keep specifies additional tag fields to preserve from the source file
	Keep []string
	// Drop specifies tag fields to remove from the final output
	Drop []string
}

// ApplyTagConfig applies tag configuration rules to merge source tags into destination tags.
// It preserves specified tags from the source and removes unwanted tags according to the config.
func ApplyTagConfig(
	dest, source tags.Tags,
	conf TagConfig,
) {
	for _, k := range defaultKeepConfig {
		dest.Set(k, source.Values(k)...)
	}
	for _, k := range conf.Keep {
		dest.Set(k, source.Values(k)...)
	}
	for _, k := range conf.Drop {
		dest.Set(k)
	}
}

// defaultKeepConfig is set of tags which are kept as-is when replacing tags.
var defaultKeepConfig = []string{
	tags.ReplayGainTrackGain,
	tags.ReplayGainTrackPeak,
	tags.ReplayGainAlbumGain,
	tags.ReplayGainAlbumPeak,
	tags.ReplayGainTrackRange,
	tags.ReplayGainAlbumRange,
	tags.ReplayGainReferenceLoudness,
	tags.BPM,
	tags.Key,
	tags.Lyrics,
	tags.AcoustIDFingerprint,
	tags.AcoustIDID,
	tags.Encoder,
	tags.EncodedBy,
	tags.Comment,
}

// FileSystemOperation defines operations that can be performed on files during the import/tagging process.
// Implementations handle different ways to transfer files (move, copy, reflink) while maintaining consistent behaviours.
type FileSystemOperation interface {
	// CanModifyDest returns whether this operation can modify existing destination files.
	// Note: If down the line some sort of "in place" tagging operation is needed, then a `CanModifySource` may be appropriate too.
	CanModifyDest() bool

	// ProcessPath handles transferring a file from src to dest path.
	// It ensures the destination directory exists and records the path in the DirContext.
	// The exact behaviour (move/copy/reflink) depends on the specific implementation.
	// Returns an error if the operation fails.
	ProcessPath(ctx context.Context, dc DirContext, src, dest string) error

	// PostSource performs any cleanup or post-processing on the source directory
	// after all files have been processed. For example, removing empty directories
	// after a move operation.
	// The limit parameter specifies a boundary directory that should not be removed.
	// Returns an error if the cleanup operation fails.
	PostSource(ctx context.Context, dc DirContext, limit string, src string) error
}

// DirContext tracks known files in the destination directory. After a release is put in place,
// unknown files not in the DirContext will be deleted.
type DirContext struct {
	knownDestPaths map[string]struct{}
}

// NewDirContext creates a new DirContext to track destination paths.
func NewDirContext() DirContext {
	return DirContext{knownDestPaths: map[string]struct{}{}}
}

// Move implements FileSystemOperation to move files from source to destination.
type Move struct {
	dryRun bool
}

// NewMove creates a new Move operation with the specified dry-run mode.
// If dryRun is true, no files will actually be moved.
func NewMove(dryRun bool) Move { return Move{dryRun: dryRun} }

// CanModifyDest returns whether this operation can modify destination files.
// For Move operations, this is determined by the dryRun setting.
func (m Move) CanModifyDest() bool {
	return !m.dryRun
}

// ProcessPath moves a file from src to dest, ensuring the destination directory exists.
// If the operation is in dry-run mode, it will only log the intended action.
// If src and dest are the same, no action is taken.
func (m Move) ProcessPath(ctx context.Context, dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if m.dryRun {
		slog.InfoContext(ctx, "move", "from", src, "to", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create dest path: %w", err)
	}

	if err := os.Rename(src, dest); err != nil {
		if errNo := syscall.Errno(0); errors.As(err, &errNo) && errNo == 18 /*  invalid cross-device link */ {
			// we tried to rename across filesystems, copy and delete instead
			if err := copyFile(src, dest); err != nil {
				return fmt.Errorf("copy from move: %w", err)
			}
			if err := os.Remove(src); err != nil {
				return fmt.Errorf("remove from move: %w", err)
			}

			slog.DebugContext(ctx, "moved path", "from", src, "to", dest)
			return nil
		}
		return fmt.Errorf("rename: %w", err)
	}

	slog.DebugContext(ctx, "moved path", "from", src, "to", dest)
	return nil
}

// PostSource cleans up the source directory after all files have been moved.
// It removes empty directories up to the specified limit directory.
func (m Move) PostSource(ctx context.Context, dc DirContext, limit string, src string) error {
	if limit == "" {
		panic("empty limit dir")
	}

	toRemove := []string{src}
	toLock := src

	if fileutil.HasPrefix(src, limit) {
		for d := filepath.Dir(src); d != filepath.Clean(limit); d = filepath.Dir(d) {
			toRemove = append(toRemove, d)
			toLock = d // only highest parent
		}
	}

	unlock := lockPaths(toLock)
	defer unlock()

	for _, p := range toRemove {
		if err := safeRemoveAll(ctx, p, m.dryRun); err != nil {
			return fmt.Errorf("safe remove all: %w", err)
		}
	}

	return nil
}

// Copy implements FileSystemOperation to copy files from source to destination.
type Copy struct {
	dryRun bool
}

// NewCopy creates a new Copy operation with the specified dry-run mode.
// If dryRun is true, no files will actually be copied.
func NewCopy(dryRun bool) Copy { return Copy{dryRun: dryRun} }

// CanModifyDest returns whether this operation can modify destination files.
// For Copy operations, this is determined by the dryRun setting.
func (c Copy) CanModifyDest() bool {
	return !c.dryRun
}

// ProcessPath copies a file from src to dest, ensuring the destination directory exists.
// If the operation is in dry-run mode, it will only log the intended action.
// If src and dest are the same, it returns ErrSelfCopy.
func (c Copy) ProcessPath(ctx context.Context, dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.dryRun {
		slog.InfoContext(ctx, "copy", "from", src, "to", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create dest path: %w", err)
	}

	if err := copyFile(src, dest); err != nil {
		return err
	}

	slog.DebugContext(ctx, "copied path", "from", src, "to", dest)
	return nil
}

// PostSource performs any necessary cleanup of the source directory.
// For Copy operations, this is a no-op since the source files remain in place.
func (Copy) PostSource(ctx context.Context, dc DirContext, limit string, src string) error {
	return nil
}

// Reflink implements FileSystemOperation to copy files using reflink (copy-on-write) when supported.
type Reflink struct {
	dryRun bool
}

// NewReflink creates a new Reflink operation with the specified dry-run mode.
// If dryRun is true, no files will actually be reflinked.
func NewReflink(dryRun bool) Reflink { return Reflink{dryRun: dryRun} }

// CanModifyDest returns whether this operation can modify destination files.
// For Reflink operations, this is determined by the dryRun setting.
func (c Reflink) CanModifyDest() bool {
	return !c.dryRun
}

// ProcessPath creates a reflink (copy-on-write) clone of a file from src to dest.
// If the operation is in dry-run mode, it will only log the intended action.
// If src and dest are the same, it returns ErrSelfCopy.
func (c Reflink) ProcessPath(ctx context.Context, dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.dryRun {
		slog.InfoContext(ctx, "reflink", "from", src, "to", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return fmt.Errorf("create dest path: %w", err)
	}

	if err := reflink.Always(src, dest); err != nil {
		return fmt.Errorf("reflink file: %w", err)
	}

	slog.DebugContext(ctx, "reflinked path", "from", src, "to", dest)
	return nil
}

// PostSource performs any necessary cleanup of the source directory.
// For Reflink operations, this is a no-op since the source files remain in place.
func (Reflink) PostSource(ctx context.Context, dc DirContext, limit string, src string) error {
	return nil
}

// trimDestDir deletes all items in a destination dir that don't look like they should be there.
func trimDestDir(ctx context.Context, dc DirContext, dest string, canModifyDest bool) error {
	entries, err := os.ReadDir(dest)
	if !canModifyDest && errors.Is(err, os.ErrNotExist) {
		// this is fine if we're only doing a dry run
	} else if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var toDelete []string
	var size uint64
	for _, entry := range entries {
		path := filepath.Join(dest, entry.Name())
		if _, ok := dc.knownDestPaths[path]; ok {
			continue
		}
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get info: %w", err)
		}
		size += uint64(info.Size()) //nolint:gosec
		toDelete = append(toDelete, path)
	}
	if size > thresholdSizeTrim {
		return fmt.Errorf("extra files were too big to remove: %d/%d", size, thresholdSizeTrim)
	}

	var deleteErrs []error
	for _, p := range toDelete {
		if !canModifyDest {
			slog.InfoContext(ctx, "delete extra file", "path", p)
			continue
		}
		if err := os.Remove(p); err != nil {
			deleteErrs = append(deleteErrs, err)
		}
		slog.InfoContext(ctx, "deleted extra file", "path", p)
	}
	if err := errors.Join(deleteErrs...); err != nil {
		return fmt.Errorf("delete extra files: %w", err)
	}

	return nil
}

func copyFile(src, dest string) (err error) {
	srcf, err := os.Open(src) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer srcf.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), "")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rerr := os.Remove(tmp.Name()); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}
	}()

	if _, err := io.Copy(tmp, srcf); err != nil {
		return fmt.Errorf("do copy: %w", err)
	}

	if st, err := srcf.Stat(); err == nil {
		if err := tmp.Chmod(st.Mode()); err != nil {
			return fmt.Errorf("chmod tmp: %w", err)
		}
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync tmp: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}

	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("do rename: %w", err)
	}
	return nil
}

func processCover(
	ctx context.Context, cfg *Config,
	op FileSystemOperation, dc DirContext, release *musicbrainz.Release, destDir string, cover string,
) error {
	coverPath := func(p string) string {
		return filepath.Join(destDir, "cover"+filepath.Ext(p))
	}

	if op.CanModifyDest() && (cover == "" || cfg.UpgradeCover) {
		skipFunc := func(resp *http.Response) bool {
			if resp.ContentLength > 8388608 /* 8 MiB */ {
				return true // too big to download
			}
			if cover == "" {
				return false
			}
			info, err := os.Stat(cover)
			if err != nil {
				return false
			}
			return resp.ContentLength == info.Size()
		}

		coverTmp, err := tryDownloadMusicBrainzCover(ctx, &cfg.CoverArtArchiveClient, release, skipFunc)
		if err != nil {
			return fmt.Errorf("maybe fetch better cover: %w", err)
		}
		if coverTmp != "" {
			if err := (Move{}).ProcessPath(ctx, dc, coverTmp, coverPath(coverTmp)); err != nil {
				return fmt.Errorf("move new cover to dest: %w", err)
			}
			return nil
		}
	}

	// process any existing cover if we didn't fetch (or find) any from musicbrainz
	if cover != "" {
		if err := op.ProcessPath(ctx, dc, cover, coverPath(cover)); err != nil {
			return fmt.Errorf("move file to dest: %w", err)
		}
	}
	return nil
}

func tryDownloadMusicBrainzCover(ctx context.Context, caa *musicbrainz.CAAClient, release *musicbrainz.Release, skipFunc func(*http.Response) bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	coverURL, err := caa.GetCoverURL(ctx, release)
	if err != nil {
		return "", err
	}
	if coverURL == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := caa.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request cover url: %w", err)
	}
	defer resp.Body.Close()

	// try to avoid downloading
	if skipFunc(resp) {
		return "", nil
	}

	ext := path.Ext(coverURL)
	tmpf, err := os.CreateTemp("", ".wrtag-cover-tmp-*"+ext)
	if err != nil {
		return "", fmt.Errorf("mktmp: %w", err)
	}
	defer tmpf.Close()

	if _, err := io.Copy(tmpf, resp.Body); err != nil {
		return "", fmt.Errorf("copy to tmp: %w", err)
	}

	return tmpf.Name(), nil
}

func dirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("get info %w", err)
		}
		if !info.IsDir() {
			size += uint64(info.Size()) //nolint:gosec
		}
		return err
	})
	return size, err
}

func safeRemoveAll(ctx context.Context, src string, dryRun bool) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		// skip if we have any child directories
		if entry.IsDir() {
			return nil
		}
	}

	if dryRun {
		slog.InfoContext(ctx, "remove all", "path", src)
		return nil
	}

	size, err := dirSize(src)
	if err != nil {
		return fmt.Errorf("get dir size for sanity check: %w", err)
	}
	if size > thresholdSizeClean {
		return fmt.Errorf("folder was too big for clean up: %d/%d", size, thresholdSizeClean)
	}

	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("error cleaning up folder: %w", err)
	}

	slog.DebugContext(ctx, "removed path", "path", src)
	return nil
}

func mapFunc[T, To any](elms []T, f func(int, T) To) []To {
	var res = make([]To, 0, len(elms))
	for i, v := range elms {
		res = append(res, f(i, v))
	}
	return res
}

func filterFunc[T any](elms []T, f func(T) bool) []T {
	var res = make([]T, 0, len(elms))
	for _, el := range elms {
		if f(el) {
			res = append(res, el)
		}
	}
	return res
}

func trimZero[T comparable](elms ...T) []T {
	var zero T
	return slices.DeleteFunc(elms, func(t T) bool { return t == zero })
}

func parseAnyTime(str string) time.Time {
	t, _ := dateparse.ParseAny(str)
	return t
}

func logTagChanges(ctx context.Context, fileKey string, lvl slog.Level, before, after tags.Tags) {
	fileKey = filepath.Base(fileKey)
	for k := range after {
		if before, after := before[k], after[k]; !slices.Equal(before, after) {
			slog.Log(ctx, lvl, "tag change", "file", fileKey, "key", k, "from", before, "to", after)
		}
	}
}

func extendQueryWithOriginFile(ctx context.Context, q *musicbrainz.ReleaseQuery, originFile *originfile.OriginFile) error { //nolint:unparam
	if originFile == nil {
		return nil
	}
	slog.DebugContext(ctx, "using origin file", "file", originFile)

	if originFile.RecordLabel != "" {
		q.Label = originFile.RecordLabel
	}
	if originFile.CatalogueNumber != "" {
		q.CatalogueNum = originFile.CatalogueNumber
	}
	if originFile.Media != "" {
		media := originFile.Media
		media = strings.ReplaceAll(media, "WEB", "Digital Media")
		q.Format = media
	}
	if originFile.EditionYear > 0 {
		q.Date = time.Date(originFile.EditionYear, 0, 0, 0, 0, 0, 0, time.UTC)
	}
	return nil
}

var trlock = treelock.NewTreeLock()

func lockPaths(paths ...string) func() {
	for i := range paths {
		paths[i] = filepath.Clean(paths[i])
	}
	paths = slices.Compact(paths)

	keys := make([][]string, 0, len(paths))
	for _, path := range paths {
		key := strings.Split(path, string(filepath.Separator))
		keys = append(keys, key)
	}

	trlock.LockMany(keys...)
	return func() {
		trlock.UnlockMany(keys...)
	}
}
