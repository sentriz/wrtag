package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/table/table"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/logging"
	"go.senan.xyz/wrtag/cmd/internal/wrtagflag"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/researchlink"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] move [<move options>] <path>\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] copy [<copy options>] <path>\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] sync [<sync options>] <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "See also:\n")
		fmt.Fprintf(flag.Output(), "  $ %s move -h\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s copy -h\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s sync -h\n", flag.Name())
	}
}

func main() {
	defer logging.Logging()()
	wrtagflag.DefaultClient()
	var (
		cfg                 = wrtagflag.Config()
		notifications       = wrtagflag.Notifications()
		researchLinkQuerier = wrtagflag.ResearchLinks()
	)
	wrtagflag.Parse()

	if flag.NArg() == 0 {
		slog.Error("no command provided")
		return
	}

	if cfg.PathFormat.Root() == "" {
		slog.Error("no path-format configured")
		return
	}

	switch command, args := flag.Arg(0), flag.Args()[1:]; command {
	case "move", "copy":
		flag := flag.NewFlagSet(command, flag.ExitOnError)
		var (
			yes     = flag.Bool("yes", false, "Use the found release anyway despite a low score")
			useMBID = flag.String("mbid", "", "Overwrite matched mbid")
			dryRun  = flag.Bool("dry-run", false, "Do a dry run of imports")
		)
		flag.Parse(args)

		var importCondition wrtag.ImportCondition
		if *yes {
			importCondition = wrtag.Confirm
		}

		if flag.NArg() != 1 {
			slog.Error("please provide a single directory")
			return
		}

		dir := flag.Arg(0)
		dir, err := filepath.Abs(dir)
		if err != nil {
			slog.Error("making path abs", "err", err)
			return

		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		op, err := wrtag.OperationByName(command, *dryRun)
		if err != nil {
			slog.Error("get operation by name", "err", err)
			return
		}

		if err := runOperation(ctx, cfg, researchLinkQuerier, op, dir, importCondition, *useMBID); err != nil {
			slog.Error("running", "command", command, "err", err)
			return
		}

	case "sync":
		flag := flag.NewFlagSet(command, flag.ExitOnError)
		var (
			ageYounger = flag.Duration("age-younger", 0, "Minimum duration a release should be left unsynced")
			ageOlder   = flag.Duration("age-older", 0, "Maximum duration a release should be left unsynced")
			dryRun     = flag.Bool("dry-run", false, "Do a dry run of imports")
			numWorkers = flag.Int("num-workers", runtime.NumCPU(), "Number of directories to process concurrently")
		)
		flag.Parse(args)

		// walk the whole root dir by default, or some user provided dirs if provided
		var dirs []string
		if args := flag.Args(); len(args) > 0 {
			dirs = append(dirs, args...)
		} else if root := cfg.PathFormat.Root(); root != "" {
			dirs = append(dirs, root)
		}

		for i := range dirs {
			var err error
			dirs[i], err = filepath.Abs(dirs[i])
			if err != nil {
				slog.Error("making path abs", "err", err)
				return
			}
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		start := time.Now()

		var stats syncStats
		if err := runSync(ctx, cfg, &stats, dirs, *ageYounger, *ageOlder, *dryRun, *numWorkers); err != nil {
			slog.Error("running", "command", command, "err", err)
			return
		}

		took := time.Since(start).Truncate(time.Millisecond)

		switch {
		case stats.errors.Load() > 0:
			slog.Error("sync finished", "took", took, "", &stats)
			notifications.Sendf(ctx, notifSyncError, "sync finished in %v %v", took, &stats)
		default:
			slog.Info("sync finished", "took", took, "", &stats)
			notifications.Sendf(ctx, notifSyncComplete, "sync finished in %v %v", took, &stats)
		}

	default:
		slog.Error("unknown command", "command", command)
		return
	}
}

func runOperation(
	ctx context.Context, cfg *wrtag.Config, researchLinks *researchlink.Builder,
	op wrtag.FileSystemOperation, srcDir string, cond wrtag.ImportCondition, useMBID string,
) error {
	r, searchErr := wrtag.ProcessDir(ctx, cfg, op, srcDir, cond, useMBID)
	if searchErr != nil && !wrtag.IsNonFatalError(searchErr) {
		return fmt.Errorf("processing: %w", searchErr)
	}

	slog.InfoContext(ctx, "matched",
		"score", fmt.Sprintf("%.2f%%", r.Score),
		"url", fmt.Sprintf("https://musicbrainz.org/release/%s", r.Release.ID),
	)

	t := table.NewStringWriter()
	for _, d := range r.Diff {
		fmt.Fprintf(t, "%s\t%s\t%s\n", d.Field, fmtDiff(d.Before), fmtDiff(d.After))
	}
	for _, row := range strings.Split(strings.TrimRight(t.String(), "\n"), "\n") {
		fmt.Fprintf(os.Stderr, "\t%s\n", row)
	}

	links, err := researchLinks.Build(researchlink.Query{
		Artist: r.Query.Artist,
		Album:  r.Query.Release,
		UPC:    r.Query.Barcode,
		Date:   r.Query.Date,
	})
	if err != nil {
		return fmt.Errorf("research search: %w", err)
	}
	for _, link := range links {
		slog.InfoContext(ctx, "search with", "name", link.Name, "url", link.URL)
	}

	if searchErr != nil {
		return fmt.Errorf("processing: %w", searchErr)
	}
	return nil
}

const (
	notifSyncComplete = "sync-complete"
	notifSyncError    = "sync-error"
)

func runSync(ctx context.Context, cfg *wrtag.Config, stats *syncStats, dirs []string, ageYounger, ageOlder time.Duration, dryRun bool, numWorkers int) error {
	leaves := make(chan string)
	go func() {
		for _, d := range dirs {
			err := fileutil.WalkLeaves(d, func(path string, _ fs.DirEntry) error {
				leaves <- path
				return nil
			})
			if err != nil {
				slog.Error("walking paths", "err", err)
				continue
			}
		}
		close(leaves)
	}()

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctxConsume(ctx, leaves, func(dir string) {
				stats.saw.Add(1)
				r, err := syncDir(ctx, cfg, ageYounger, ageOlder, wrtag.Move{DryRun: dryRun}, dir)
				if err != nil && !errors.Is(err, context.Canceled) {
					stats.errors.Add(1)
					slog.ErrorContext(ctx, "processing dir", "dir", dir, "err", err)
					return
				}
				if r != nil {
					stats.processed.Add(1)
					slog.InfoContext(ctx, "processed dir", "dir", dir, "score", r.Score)
				}
			})
		}()
	}

	wg.Wait()

	return nil
}

type syncStats struct {
	saw       atomic.Uint64
	processed atomic.Uint64
	errors    atomic.Uint64
}

func (s *syncStats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Uint64("saw", s.saw.Load()),
		slog.Uint64("processed", s.processed.Load()),
		slog.Uint64("errors", s.errors.Load()),
	)
}

func (s *syncStats) String() string {
	return s.LogValue().String()
}

func syncDir(ctx context.Context, cfg *wrtag.Config, ageYounger, ageOlder time.Duration, op wrtag.FileSystemOperation, srcDir string) (*wrtag.SearchResult, error) {
	if ageYounger > 0 || ageOlder > 0 {
		info, err := os.Stat(srcDir)
		if err != nil {
			return nil, fmt.Errorf("stat dir: %w", err)
		}
		if ageYounger > 0 && time.Since(info.ModTime()) > ageYounger {
			return nil, nil
		}
		if ageOlder > 0 && time.Since(info.ModTime()) < ageOlder {
			return nil, nil
		}
	}

	r, err := wrtag.ProcessDir(ctx, cfg, op, srcDir, wrtag.HighScoreOrMBID, "")
	if err != nil {
		return nil, err
	}

	if err := os.Chtimes(srcDir, time.Time{}, time.Now()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("chtimes %q: %v", srcDir, err)
	}
	return r, nil
}

func ctxConsume[T any](ctx context.Context, work <-chan T, f func(T)) {
	for {
		select { // prority select for ctx.Done()
		case <-ctx.Done():
			return
		default:
			select {
			case <-ctx.Done():
				return
			case w, ok := <-work:
				if !ok {
					return
				}
				f(w)
			}
		}
	}
}

var dm = dmp.New()

func fmtDiff(diff []dmp.Diff) string {
	if d := dm.DiffPrettyText(diff); d != "" {
		return d
	}
	return "[empty]"
}
