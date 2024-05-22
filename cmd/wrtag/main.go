package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"go.senan.xyz/table/table"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flags"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
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

// updated while testing
var mb = flags.MusicBrainz()

func main() {
	defer flags.ExitError()
	var (
		keepFiles           = flags.KeepFiles()
		pathFormat          = flags.PathFormat()
		researchLinkQuerier = flags.Querier()
		tagWeights          = flags.TagWeights()
		addons              = flags.Addons()
	)
	flags.Parse()

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
		if err := run(ctx, mb, pathFormat, tagWeights, researchLinkQuerier, keepFiles, *addons, operation(command, *dryRun), dir, *useMBID, importCondition); err != nil {
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
	ctx context.Context, mb wrtag.MusicbrainzClient,
	pathFormat *pathformat.Format, tagWeights tagmap.TagWeights, researchLinkQuerier *researchlink.Querier, keepFiles map[string]struct{}, addons []wrtag.Addon,
	op wrtag.FileSystemOperation, srcDir string,
	useMBID string, importCondition wrtag.ImportCondition,
) error {
	r, err := wrtag.ProcessDir(ctx, mb, pathFormat, tagWeights, researchLinkQuerier, keepFiles, addons, op, srcDir, useMBID, importCondition)
	if err != nil && !errors.Is(err, wrtag.ErrScoreTooLow) {
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
