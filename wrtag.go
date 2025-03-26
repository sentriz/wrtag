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
	"strings"
	"syscall"
	"time"

	"github.com/KarpelesLab/reflink"
	"github.com/araddon/dateparse"
	"github.com/argusdusty/treelock"
	"go.senan.xyz/natcmp"
	"go.senan.xyz/wrtag/coverparse"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/originfile"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/tagmap"
	"go.senan.xyz/wrtag/tags"
)

var (
	ErrScoreTooLow        = errors.New("score too low")
	ErrTrackCountMismatch = errors.New("track count mismatch")
	ErrNoTracks           = errors.New("no tracks in dir")
	ErrNotSortable        = errors.New("tracks in dir can't be sorted")
	ErrSelfCopy           = errors.New("can't copy self to self")
)

func IsNonFatalError(err error) bool {
	return errors.Is(err, ErrScoreTooLow) || errors.Is(err, ErrTrackCountMismatch)
}

const minScore = 95

const (
	thresholdSizeClean uint64 = 20 * 1e6   // 20 MB
	thresholdSizeTrim  uint64 = 3000 * 1e6 // 3000 MB
)

type SearchResult struct {
	Release    *musicbrainz.Release
	Query      musicbrainz.ReleaseQuery
	Score      float64
	DestDir    string
	Diff       []tagmap.Diff
	OriginFile *originfile.OriginFile
}

type ImportCondition uint8

const (
	HighScore ImportCondition = iota
	HighScoreOrMBID
	Confirm
)

type Addon interface {
	ProcessRelease(context.Context, []string) error
}

type Config struct {
	MusicBrainzClient     musicbrainz.MBClient
	CoverArtArchiveClient musicbrainz.CAAClient
	PathFormat            pathformat.Format
	TagWeights            tagmap.TagWeights
	KeepFiles             map[string]struct{}
	Addons                []Addon
	UpgradeCover          bool
}

func ProcessDir(
	ctx context.Context, cfg *Config,
	op FileSystemOperation, srcDir string, cond ImportCondition, useMBID string,
) (*SearchResult, error) {
	if cfg.PathFormat.Root() == "" {
		return nil, fmt.Errorf("no path format provided")
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

	var mbid = searchTags.Get(tags.MBReleaseID)
	if useMBID != "" {
		mbid = useMBID
	}

	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      mbid,
		MBArtistID:       searchTags.Get(tags.MBArtistID),
		MBReleaseGroupID: searchTags.Get(tags.MBReleaseGroupID),
		Release:          searchTags.Get(tags.Album),
		Artist:           cmp.Or(searchTags.Get(tags.AlbumArtist), searchTags.Get(tags.Artist)),
		Date:             parseAnyTime(searchTags.Get(tags.Date)),
		Format:           searchTags.Get(tags.MediaFormat),
		Label:            searchTags.Get(tags.Label),
		CatalogueNum:     searchTags.Get(tags.CatalogueNum),
		Barcode:          searchTags.Get(tags.UPC),
		NumTracks:        len(pathTags),
	}

	// parse https://github.com/x1ppy/gazelle-origin files, if one exists
	originFile, err := originfile.Find(srcDir)
	if err != nil {
		return nil, fmt.Errorf("find origin file: %w", err)
	}

	if mbid == "" {
		if err := extendQueryWithOriginFile(&query, originFile); err != nil {
			return nil, fmt.Errorf("use origin file: %w", err)
		}
	}

	release, err := cfg.MusicBrainzClient.SearchRelease(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search musicbrainz: %w", err)
	}

	releaseTracks := musicbrainz.FlatTracks(release.Media)

	score, diff := tagmap.DiffRelease(cfg.TagWeights, release, releaseTracks, pathTags)

	if len(pathTags) != len(releaseTracks) {
		return &SearchResult{release, query, 0, "", diff, originFile}, fmt.Errorf("%w: %d remote / %d local", ErrTrackCountMismatch, len(releaseTracks), len(pathTags))
	}

	var shouldImport bool
	switch cond {
	case HighScoreOrMBID:
		shouldImport = score >= minScore || mbid != ""
	case HighScore:
		shouldImport = score >= minScore
	case Confirm:
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

	// lock both source and destination directories
	unlock := lockPaths(
		srcDir,
		destDir,
	)

	// calculate new paths
	destPaths := make([]string, 0, len(pathTags))
	for i, pt := range pathTags {
		destPath, err := cfg.PathFormat.Execute(release, i, strings.ToLower(filepath.Ext(pt.Path)))
		if err != nil {
			return nil, fmt.Errorf("create path: %w", err)
		}

		destPaths = append(destPaths, destPath)
	}

	dc := NewDirContext()

	// move/copy and tag
	for i := range len(pathTags) {
		pt, rt, destPath := pathTags[i], releaseTracks[i], destPaths[i]

		if err := op.ProcessFile(dc, pt.Path, destPath); err != nil {
			return nil, fmt.Errorf("process path %q: %w", filepath.Base(pt.Path), err)
		}

		destTags := tagmap.ReleaseTags(release, labelInfo, genres, i, &rt)

		if lvl, slog := slog.LevelDebug, slog.Default(); slog.Enabled(ctx, lvl) {
			logTagChanges(ctx, pt.Path, lvl, pt.Tags, destTags)
		}

		if op.IsDryRun() {
			continue
		}
		if tags.Equal(pt.Tags, destTags) {
			// try avoid more io if we can
			continue
		}

		if err := tags.WriteTags(destPath, destTags); err != nil { // not replacing here since some plugins use other tags
			return nil, fmt.Errorf("write tag file: %w", err)
		}
	}

	if err := processCover(ctx, cfg, op, dc, release, destDir, cover); err != nil {
		return nil, fmt.Errorf("process cover: %w", err)
	}

	// process addons with new files
	if !op.IsDryRun() {
		for _, addon := range cfg.Addons {
			if err := addon.ProcessRelease(ctx, destPaths); err != nil {
				return nil, fmt.Errorf("process addon: %w", err)
			}
		}
	}

	for kf := range cfg.KeepFiles {
		if err := op.ProcessFile(dc, filepath.Join(srcDir, kf), filepath.Join(destDir, kf)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("process keep file %q: %w", kf, err)
		}
	}

	if err := trimDestDir(dc, destDir, op.IsDryRun()); err != nil {
		return nil, fmt.Errorf("trim: %w", err)
	}

	unlock()

	if srcDir != destDir {
		if err := op.RemoveSrc(dc, cfg.PathFormat.Root(), srcDir); err != nil {
			return nil, fmt.Errorf("clean: %w", err)
		}
	}

	return &SearchResult{release, query, score, destDir, diff, originFile}, nil
}

type PathTags struct {
	Path string
	tags.Tags
}

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
			coverparse.BestBetween(&cover, path)
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
		var haveNum, havePath bool = true, true
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
			natcmp.Compare(a.Get(tags.DiscNumber), b.Get(tags.DiscNumber)),   // disc mumbers like "1", "2", "disc 1", "disc 10"
			natcmp.Compare(filepath.Dir(a.Path), filepath.Dir(b.Path)),       // might have disc folders instead of tags
			natcmp.Compare(a.Get(tags.TrackNumber), b.Get(tags.TrackNumber)), // track numbers, could be "A1" "B1" "1" "10" "100" "1/10" "2/10"
			natcmp.Compare(a.Path, b.Path),                                   // fallback to paths
		)
	})

	return string(cover), pathTags, nil
}

func DestDir(pathFormat *pathformat.Format, release *musicbrainz.Release) (string, error) {
	path, err := pathFormat.Execute(release, 0, ".eg")
	if err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	dir := filepath.Dir(path)
	dir = filepath.Clean(dir)
	return dir, nil
}

var _ FileSystemOperation = (*Move)(nil)
var _ FileSystemOperation = (*Copy)(nil)
var _ FileSystemOperation = (*Reflink)(nil)

type FileSystemOperation interface {
	IsDryRun() bool
	ProcessFile(dc DirContext, src, dest string) error
	RemoveSrc(dc DirContext, limit string, src string) error
}

type DirContext struct {
	knownDestPaths map[string]struct{}
}

func NewDirContext() DirContext {
	return DirContext{knownDestPaths: map[string]struct{}{}}
}

type Move struct {
	DryRun bool
}

func (m Move) IsDryRun() bool {
	return m.DryRun
}

func (m Move) ProcessFile(dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if m.DryRun {
		slog.Info("move", "from", src, "to", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
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

			slog.Debug("moved path", "from", src, "to", dest)
			return nil
		}
		return fmt.Errorf("rename: %w", err)
	}

	slog.Debug("moved path", "from", src, "to", dest)
	return nil
}

func (m Move) RemoveSrc(dc DirContext, limit string, src string) error {
	if limit == "" {
		panic("empty limit dir")
	}

	toRemove := []string{src}
	toLock := src

	if strings.HasPrefix(src, limit) {
		for d := filepath.Dir(src); d != filepath.Clean(limit); d = filepath.Dir(d) {
			toRemove = append(toRemove, d)
			toLock = d // only highest parent
		}
	}

	unlock := lockPaths(toLock)
	defer unlock()

	for _, p := range toRemove {
		if err := safeRemoveAll(p, m.DryRun); err != nil {
			return fmt.Errorf("safe remove all: %w", err)
		}
	}

	return nil
}

type Copy struct {
	DryRun bool
}

func (c Copy) IsDryRun() bool {
	return c.DryRun
}

func (c Copy) ProcessFile(dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.DryRun {
		slog.Info("copy", "from", src, "to", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return fmt.Errorf("create dest path: %w", err)
	}

	if err := copyFile(src, dest); err != nil {
		return err
	}

	slog.Debug("copied path", "from", src, "to", dest)
	return nil
}

func (Copy) RemoveSrc(dc DirContext, limit string, src string) error {
	return nil
}

// this will probably need to live with the cmd packages later to keep generic config out of this package
func OperationByName(name string, dryRun bool) (FileSystemOperation, error) {
	switch name {
	case "copy":
		return Copy{DryRun: dryRun}, nil
	case "move":
		return Move{DryRun: dryRun}, nil
	case "reflink":
		return Reflink{DryRun: dryRun}, nil
	default:
		return nil, fmt.Errorf("unknown operation")
	}
}

type Reflink struct {
	DryRun bool
}

func (c Reflink) IsDryRun() bool {
	return c.DryRun
}

func (c Reflink) ProcessFile(dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.DryRun {
		slog.Info("reflink", "from", src, "to", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return fmt.Errorf("create dest path: %w", err)
	}

	if err := reflink.Always(src, dest); err != nil {
		return fmt.Errorf("reflink file: %w", err)
	}

	slog.Debug("reflinked path", "from", src, "to", dest)
	return nil
}

func (Reflink) RemoveSrc(dc DirContext, limit string, src string) error {
	return nil
}

// trimDestDir deletes all items in a destination dir that don't look like they should be there
func trimDestDir(dc DirContext, dest string, dryRun bool) error {
	entries, err := os.ReadDir(dest)
	if dryRun && errors.Is(err, os.ErrNotExist) {
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
		size += uint64(info.Size())
		toDelete = append(toDelete, path)
	}
	if size > thresholdSizeTrim {
		return fmt.Errorf("extra files were too big remove: %d/%d", size, thresholdSizeTrim)
	}

	var deleteErrs []error
	for _, p := range toDelete {
		if dryRun {
			slog.Info("delete extra file", "path", p)
			continue
		}
		if err := os.Remove(p); err != nil {
			deleteErrs = append(deleteErrs, err)
		}
		slog.Info("deleted extra file", "path", p)
	}
	if err := errors.Join(deleteErrs...); err != nil {
		return fmt.Errorf("delete extra files: %w", err)
	}

	return nil
}

func copyFile(src, dest string) (err error) {
	srcf, err := os.Open(src)
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

	if !op.IsDryRun() && (cover == "" || cfg.UpgradeCover) {
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
			if err := (Move{}).ProcessFile(dc, coverTmp, coverPath(coverTmp)); err != nil {
				return fmt.Errorf("move new cover to dest: %w", err)
			}
			return nil
		}
	}

	// process any existing cover if we didn't fetch (or find) any from musicbrainz
	if cover != "" {
		if err := op.ProcessFile(dc, cover, coverPath(cover)); err != nil {
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

	// try avoid downloading
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
			size += uint64(info.Size())
		}
		return err
	})
	return size, err
}

func safeRemoveAll(src string, dryRun bool) error {
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
		slog.Info("remove all", "path", src)
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

	slog.Debug("removed path", "path", src)
	return nil
}

func parseAnyTime(str string) time.Time {
	t, _ := dateparse.ParseAny(str)
	return t
}

func logTagChanges(ctx context.Context, fileKey string, lvl slog.Level, before, after tags.Tags) {
	fileKey = filepath.Base(fileKey)
	for k := range after.Iter() {
		if before, after := before.Values(k), after.Values(k); !slices.Equal(before, after) {
			slog.Log(ctx, lvl, "tag change", "file", fileKey, "key", k, "from", before, "to", after)
		}
	}
}

func extendQueryWithOriginFile(q *musicbrainz.ReleaseQuery, originFile *originfile.OriginFile) error {
	if originFile == nil {
		return nil
	}
	slog.Debug("using origin file", "file", originFile)

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
