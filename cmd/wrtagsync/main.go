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
	"go.senan.xyz/wrtag/cmd/internal/mainlib"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/notifications"
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

func main() {
	defer mainlib.Logging()()
	mainlib.WrapClient()
	var (
		cfg        = flags.Config()
		ageYounger = flag.Duration("age-younger", 0, "min duration a release should be left unsynced")
		ageOlder   = flag.Duration("age-older", 0, "max duration a release should be left unsynced")
		dryRun     = flag.Bool("dry-run", false, "do a dry run of imports")
		numWorkers = flag.Int("num-workers", 4, "number of directories to process concurrently")
	)
	flags.Parse()

	// walk the whole root dir by default, or some user provided dirs
	var dirs = []string{cfg.PathFormat.Root()}
	if flag.NArg() > 0 {
		dirs = flag.Args()
	}
	for i := range dirs {
		var err error
		dirs[i], err = filepath.Abs(dirs[i])
		if err != nil {
			slog.Error("making path abs", "err", err)
			return
		}
	}

	start := time.Now()
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var doneN, errN atomic.Uint32
	var wg sync.WaitGroup
	for range *numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctxConsume(ctx, leaves, func(dir string) {
				if err := processDir(ctx, *ageYounger, *ageOlder, cfg, wrtag.Move{DryRun: *dryRun}, dir); err != nil {
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
		cfg.Notifications.Send(ctx, notifications.SyncError, "sync finished with errors")
		slog.Error("sync finished with errors")
		return
	}
	cfg.Notifications.Send(ctx, notifications.Complete, "sync finished")
	slog.Info("sync finished")
}

func processDir(ctx context.Context, ageYounger, ageOlder time.Duration, cfg *wrtag.Config, op wrtag.FileSystemOperation, srcDir string) error {
	if ageYounger > 0 || ageOlder > 0 {
		info, err := os.Stat(srcDir)
		if err != nil {
			return fmt.Errorf("stat dir: %w", err)
		}
		if ageYounger > 0 && time.Since(info.ModTime()) > ageYounger {
			return nil
		}
		if ageOlder > 0 && time.Since(info.ModTime()) < ageOlder {
			return nil
		}
	}

	if _, err := wrtag.ProcessDir(ctx, cfg, op, srcDir, wrtag.HighScoreOrMBID, ""); err != nil {
		return err
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
