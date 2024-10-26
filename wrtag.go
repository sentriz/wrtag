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

	"github.com/argusdusty/treelock"
	"go.senan.xyz/natcmp"
	"go.senan.xyz/wrtag/coverparse"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/originfile"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
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

const minScore = 95

const (
	thresholdSizeClean uint64 = 20 * 1e6   // 20 MB
	thresholdSizeTrim  uint64 = 3000 * 1e6 // 3000 MB
)

type SearchResult struct {
	Release       *musicbrainz.Release
	Score         float64
	DestDir       string
	Diff          []tagmap.Diff
	ResearchLinks []researchlink.SearchResult
	OriginFile    *originfile.OriginFile
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
	ResearchLinkQuerier   researchlink.Querier
	KeepFiles             map[string]struct{}
	Addons                []Addon
	Notifications         notifications.Notifications
	UpgradeCover          bool
}

func ProcessDir(
	ctx context.Context, cfg *Config,
	op FileSystemOperation, srcDir string, cond ImportCondition, useMBID string,
) (*SearchResult, error) {
	if !filepath.IsAbs(srcDir) {
		panic("src dir not abs") // this is a programmer error for now
	}
	srcDir = filepath.Clean(srcDir)

	cover, files, err := ReadReleaseDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	searchFile := files[0]

	var mbid = searchFile.Read(tags.MBReleaseID)
	if useMBID != "" {
		mbid = useMBID
	}

	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      mbid,
		MBArtistID:       searchFile.Read(tags.MBArtistID),
		MBReleaseGroupID: searchFile.Read(tags.MBReleaseGroupID),
		Release:          searchFile.Read(tags.Album),
		Artist:           cmp.Or(searchFile.Read(tags.AlbumArtist), searchFile.Read(tags.Artist)),
		Date:             searchFile.ReadTime(tags.Date),
		Format:           searchFile.Read(tags.MediaFormat),
		Label:            searchFile.Read(tags.Label),
		CatalogueNum:     searchFile.Read(tags.CatalogueNum),
		Barcode:          searchFile.Read(tags.UPC),
		NumTracks:        len(files),
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

	var researchLinks []researchlink.SearchResult
	researchLinks, err = cfg.ResearchLinkQuerier.Search(researchlink.Query{
		Artist: cmp.Or(searchFile.Read(tags.AlbumArtist), searchFile.Read(tags.Artist)),
		Album:  searchFile.Read(tags.Album),
		UPC:    searchFile.Read(tags.UPC),
		Date:   searchFile.ReadTime(tags.Date),
	})
	if err != nil {
		return nil, fmt.Errorf("research querier search: %w", err)
	}

	releaseTracks := musicbrainz.FlatTracks(release.Media)

	score, diff := tagmap.DiffRelease(cfg.TagWeights, release, releaseTracks, files)

	if len(releaseTracks) != len(files) {
		return &SearchResult{release, 0, "", diff, researchLinks, originFile}, fmt.Errorf("%w: %d remote / %d local", ErrTrackCountMismatch, len(releaseTracks), len(files))
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
		return &SearchResult{release, score, "", diff, researchLinks, originFile}, ErrScoreTooLow
	}

	destDir, err := DestDir(&cfg.PathFormat, release)
	if err != nil {
		return nil, fmt.Errorf("gen dest dir: %w", err)
	}

	labelInfo := musicbrainz.AnyLabelInfo(release)
	genres := musicbrainz.AnyGenres(release)
	isCompilation := musicbrainz.IsCompilation(release.ReleaseGroup)

	// lock both source and destination directories
	unlock := lockPaths(
		srcDir,
		destDir,
	)

	dc := NewDirContext()

	destPaths := make([]string, 0, len(releaseTracks))
	for i, t := range releaseTracks {
		file := files[i]
		pathFormatData := pathformat.Data{Release: release, Track: &t, TrackNum: i + 1, Ext: strings.ToLower(filepath.Ext(file.Path())), IsCompilation: isCompilation}
		destPath, err := cfg.PathFormat.Execute(pathFormatData)
		if err != nil {
			return nil, fmt.Errorf("create path: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return nil, fmt.Errorf("create dest path: %w", err)
		}
		if err := op.ProcessFile(dc, file.Path(), destPath); err != nil {
			return nil, fmt.Errorf("process path %q: %w", filepath.Base(file.Path()), err)
		}

		if !op.ReadOnly() {
			err = tags.Write(destPath, func(f *tags.File) error {
				tagmap.WriteTo(release, labelInfo, genres, i, &t, f)
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("write tag file: %w", err)
			}
		}

		destPaths = append(destPaths, destPath)
	}

	if err := processCover(ctx, cfg, op, dc, release, destDir, cover); err != nil {
		return nil, fmt.Errorf("process cover: %w", err)
	}

	// process addons with new tag.Files
	if !op.ReadOnly() {
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

	if err := trimDestDir(dc, destDir, op.ReadOnly()); err != nil {
		return nil, fmt.Errorf("trim: %w", err)
	}

	unlock()

	if srcDir != destDir {
		if err := op.RemoveSrc(dc, cfg.PathFormat.Root(), srcDir); err != nil {
			return nil, fmt.Errorf("clean: %w", err)
		}
	}

	return &SearchResult{release, score, destDir, diff, researchLinks, originFile}, nil
}

func ReadReleaseDir(path string) (string, []*tags.File, error) {
	mainPaths, err := fileutil.GlobDir(path, "*")
	if err != nil {
		return "", nil, fmt.Errorf("glob dir: %w", err)
	}
	discPaths, err := fileutil.GlobDir(path, "*/*") // recurse once for any disc1/ disc2/ dirs
	if err != nil {
		return "", nil, fmt.Errorf("glob dir for discs: %w", err)
	}

	var cover string
	var files []*tags.File
	for _, p := range append(mainPaths, discPaths...) {
		if coverparse.IsCover(p) {
			coverparse.BestBetween(&cover, p)
			continue
		}

		if tags.CanRead(p) {
			file, err := tags.Read(p)
			if err != nil {
				return "", nil, fmt.Errorf("read track: %w", err)
			}
			files = append(files, file)
			file.Close()
			continue
		}
	}
	if len(files) == 0 {
		return "", nil, ErrNoTracks
	}

	{
		// validate we aren't accidentally importing something like an artist folder, which may look
		// like a multi disc album to us, but will have all its tracks in one subdirectory
		discDirs := map[string]struct{}{}
		for _, pf := range files {
			discDirs[filepath.Dir(pf.Path())] = struct{}{}
		}
		if len(discDirs) == 1 && filepath.Dir(files[0].Path()) != filepath.Clean(path) {
			return "", nil, fmt.Errorf("validate tree: %w", ErrNoTracks)
		}
	}

	{
		// validate that we have track numbers, or track numbers in filenames to sort on. if we don't any
		// then releases that consist only of untitled tracks may get mixed up
		var haveNum, havePath bool = true, true
		for _, f := range files {
			if haveNum && f.Read(tags.TrackNumber) == "" {
				haveNum = false
			}
			if havePath && !strings.ContainsFunc(filepath.Base(f.Path()), func(r rune) bool { return '0' <= r && r <= '9' }) {
				havePath = false
			}
		}
		if !haveNum && !havePath {
			return "", nil, fmt.Errorf("no track numbers or numbers in filenames present: %w", ErrNotSortable)
		}
	}

	slices.SortFunc(files, func(a, b *tags.File) int {
		return cmp.Or(
			natcmp.Compare(a.Read(tags.DiscNumber), b.Read(tags.DiscNumber)),   // disc mumbers like "1", "2", "disc 1", "disc 10"
			natcmp.Compare(filepath.Dir(a.Path()), filepath.Dir(b.Path())),     // might have disc folders instead of tags
			natcmp.Compare(a.Read(tags.TrackNumber), b.Read(tags.TrackNumber)), // track numbers, could be "A1" "B1" "1" "10" "100" "1/10" "2/10"
			natcmp.Compare(a.Path(), b.Path()),                                 // fallback to paths
		)
	})

	return string(cover), files, nil
}

func DestDir(pathFormat *pathformat.Format, release *musicbrainz.Release) (string, error) {
	dummyTrack := &musicbrainz.Track{Title: "track"}
	path, err := pathFormat.Execute(pathformat.Data{Release: release, Track: dummyTrack, TrackNum: 1, Ext: ".eg"})
	if err != nil {
		return "", fmt.Errorf("create path: %w", err)
	}
	dir := filepath.Dir(path)
	dir = filepath.Clean(dir)
	return dir, nil
}

var _ FileSystemOperation = (*Move)(nil)
var _ FileSystemOperation = (*Copy)(nil)

type FileSystemOperation interface {
	ReadOnly() bool
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

func (m Move) ReadOnly() bool {
	return m.DryRun
}

func (m Move) ProcessFile(dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if m.DryRun {
		slog.Info("[dry run] move", "from", src, "to", dest)
		return nil
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

func (c Copy) ReadOnly() bool {
	return c.DryRun
}

func (c Copy) ProcessFile(dc DirContext, src, dest string) error {
	dc.knownDestPaths[dest] = struct{}{}

	if filepath.Clean(src) == filepath.Clean(dest) {
		return ErrSelfCopy
	}

	if c.DryRun {
		slog.Info("[dry run] copy", "from", src, "to", dest)
		return nil
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

// trimDestDir deletes all items in a destination dir that don't look like they should be there
func trimDestDir(dc DirContext, dest string, dryRun bool) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
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
			slog.Info("[dry run] delete extra file", "path", p)
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
	defer func() {
		if err != nil {
			if rerr := os.Remove(dest); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}
	}()

	srcf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer srcf.Close()

	destf, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("open dest: %w", err)
	}
	defer destf.Close()

	if _, err := io.Copy(destf, srcf); err != nil {
		return fmt.Errorf("do copy: %w", err)
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

	if !op.ReadOnly() && (cover == "" || cfg.UpgradeCover) {
		skipFunc := func(resp *http.Response) bool {
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
		slog.Info("[dry run] remove all", "path", src)
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
