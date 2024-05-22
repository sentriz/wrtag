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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flags"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/tagmap"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
	}
}

// updated while testing
var mb = flags.MusicBrainz()

func main() {
	defer flags.ExitError()
	var (
		keepFiles  = flags.KeepFiles()
		notifs     = flags.Notifications()
		pathFormat = flags.PathFormat()
		tagWeights = flags.TagWeights()
		addons     = flags.Addons()
		interval   = flag.Duration("interval", 0, "max duration a release should be left unsynced")
		dryRun     = flag.Bool("dry-run", false, "dry run")
	)
	flags.EnvPrefix("wrtag") // reuse main binary's namespace
	flags.Parse()

	// walk the whole root dir by default, or some user provided dirs
	var dirs = []string{pathFormat.Root()}
	if flag.NArg() > 0 {
		dirs = flag.Args()
	}

	start := time.Now()
	leaves := make(chan string)
	go func() {
		for _, d := range dirs {
			d, _ = filepath.Abs(d)
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

	operation := wrtag.Move{DryRun: *dryRun}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var knownDests sync.Map
	var doneN, errN atomic.Uint32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctxConsume(ctx, leaves, func(dir string) {
				if err := processDir(ctx, &knownDests, mb, pathFormat, tagWeights, keepFiles, *addons, operation, dir, *interval); err != nil {
					slog.ErrorContext(ctx, "processing dir", "dir", dir, "err", err)
					errN.Add(1)
					return
				}
				doneN.Add(1)
			})
		}()
	}

	wg.Wait()

	slog := slog.With("took", time.Since(start), "dirs", doneN.Load(), "errs", errN.Load())
	if errN.Load() > 0 {
		notifs.Send(ctx, notifications.SyncError, "sync finished with errors")
		slog.Error("sync finished with errors")
		return
	}
	notifs.Send(ctx, notifications.Complete, "sync finished")
	slog.Info("sync finished")
}

func processDir(
	ctx context.Context,
	knownDests *sync.Map,
	mb wrtag.MusicbrainzClient, pathFormat *pathformat.Format, tagWeights tagmap.TagWeights, keepFiles map[string]struct{}, addons []wrtag.Addon,
	op wrtag.FileSystemOperation, srcDir string,
	interval time.Duration,
) error {
	{
		// make sure we don't try process a dir that was created while walking
		srcDir, _ := filepath.Abs(srcDir)
		if _, ok := knownDests.Load(srcDir); ok {
			return nil
		}
	}

	if interval > 0 {
		info, err := os.Stat(srcDir)
		if err != nil {
			return fmt.Errorf("stat dir: %w", err)
		}
		if time.Since(info.ModTime()) < interval {
			return nil
		}
	}

	r, err := wrtag.ProcessDir(ctx, mb, pathFormat, tagWeights, nil, keepFiles, addons, op, srcDir, "", wrtag.HighScoreOrMBID)
	if err != nil {
		return err
	}
	{
		destDir, _ := filepath.Abs(r.DestDir)
		knownDests.Store(destDir, nil)
	}

	if err := os.Chtimes(srcDir, time.Time{}, time.Now()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("chtimes %q: %v", srcDir, err)
	}

	slog.InfoContext(ctx, "processed dir", "dir", srcDir)
	return nil
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
