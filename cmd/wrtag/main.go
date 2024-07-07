package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/table/table"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/cmds"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] move [<move options>] <path>\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] copy [<copy options>] <path>\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "See also:\n")
		fmt.Fprintf(flag.Output(), "  $ %s move -h\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s copy -h\n", flag.Name())
	}
}

func main() {
	defer cmds.Logging()()
	cmds.WrapClient()
	var (
		cfg = cmds.WrtagConfig()
	)
	cmds.Parse()

	if flag.NArg() == 0 {
		slog.Error("no command provided")
		return
	}

	switch command, args := flag.Arg(0), flag.Args()[1:]; command {
	case "move", "copy":
		flag := flag.NewFlagSet(command, flag.ExitOnError)
		var (
			yes     = flag.Bool("yes", false, "use the found release anyway despite a low score")
			useMBID = flag.String("mbid", "", "overwrite matched mbid")
			dryRun  = flag.Bool("dry-run", false, "dry run")
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

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		dir := flag.Arg(0)
		dir, err := filepath.Abs(dir)
		if err != nil {
			slog.Error("making path abs", "err", err)
			return
		}
		if err := run(ctx, cfg, operation(command, *dryRun), dir, importCondition, *useMBID); err != nil {
			slog.Error("running", "command", command, "err", err)
			return
		}
	default:
		slog.Error("unknown command", "command", command)
		return
	}
}

func operation(name string, dryRun bool) wrtag.FileSystemOperation {
	switch name {
	case "copy":
		return wrtag.Copy{DryRun: dryRun}
	case "move":
		return wrtag.Move{DryRun: dryRun}
	}
	return nil
}

func run(
	ctx context.Context, cfg *wrtag.Config,
	op wrtag.FileSystemOperation, srcDir string, cond wrtag.ImportCondition, useMBID string,
) error {
	r, err := wrtag.ProcessDir(ctx, cfg, op, srcDir, cond, useMBID)
	if err != nil && !errors.Is(err, wrtag.ErrScoreTooLow) && !errors.Is(err, wrtag.ErrTrackCountMismatch) {
		return fmt.Errorf("processing: %w", err)
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

	for _, link := range r.ResearchLinks {
		slog.InfoContext(ctx, "search with", "name", link.Name, "url", link.URL)
	}

	if err != nil {
		return fmt.Errorf("processing: %w", err)
	}
	return nil
}

var dm = dmp.New()

func fmtDiff(diff []dmp.Diff) string {
	if d := dm.DiffPrettyText(diff); d != "" {
		return d
	}
	return "[empty]"
}
