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
	"go.senan.xyz/wrtag/cmd/internal/cmds"
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
	defer cmds.Logging()()
	cmds.WrapClient()
	var (
		cfg        = cmds.FlagConfig()
		ageYounger = flag.Duration("age-younger", 0, "min duration a release should be left unsynced")
		ageOlder   = flag.Duration("age-older", 0, "max duration a release should be left unsynced")
		dryRun     = flag.Bool("dry-run", false, "do a dry run of imports")
		numWorkers = flag.Int("num-workers", 4, "number of directories to process concurrently")
	)
	cmds.FlagParse()

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

	var st stats
	var wg sync.WaitGroup
	for range *numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctxConsume(ctx, leaves, func(dir string) {
				st.saw.Add(1)
				r, err := processDir(ctx, *ageYounger, *ageOlder, cfg, wrtag.Move{DryRun: *dryRun}, dir)
				if err != nil && !errors.Is(err, context.Canceled) {
					st.errors.Add(1)
					slog.ErrorContext(ctx, "processing dir", "dir", dir, "err", err)
					return
				}
				if r != nil {
					st.processed.Add(1)
					slog.InfoContext(ctx, "processed dir", "dir", dir, "score", r.Score)
				}
			})
		}()
	}

	wg.Wait()

	st.took = time.Since(start)

	if st.errors.Load() > 0 {
		slog.Error("sync finished", "", &st)
		cfg.Notifications.Sendf(ctx, notifications.SyncError, "sync finished %v", &st)
		return
	}
	slog.Info("sync finished", "", &st)
	cfg.Notifications.Sendf(ctx, notifications.Complete, "sync finished %v", &st)
}

type stats struct {
	took      time.Duration
	saw       atomic.Uint64
	processed atomic.Uint64
	errors    atomic.Uint64
}

func (s *stats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Duration("took", s.took.Truncate(time.Millisecond)),
		slog.Uint64("saw", s.saw.Load()),
		slog.Uint64("processed", s.processed.Load()),
		slog.Uint64("errors", s.errors.Load()),
	)
}

func (s *stats) String() string {
	return s.LogValue().String()
}

func processDir(ctx context.Context, ageYounger, ageOlder time.Duration, cfg *wrtag.Config, op wrtag.FileSystemOperation, srcDir string) (*wrtag.SearchResult, error) {
	if ageYounger > 0 || ageOlder > 0 {
		info, err := os.Stat(srcDir)
		if err != nil {
			return nil, fmt.Errorf("stat dir: %w", err)
		}
		if ageYounger > 0 && time.Since(info.ModTime()) > ageYounger {
			return nil, err
		}
		if ageOlder > 0 && time.Since(info.ModTime()) < ageOlder {
			return nil, err
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
