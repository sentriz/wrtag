package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"go.senan.xyz/wrtag/cmd/internal/flags"
	"go.senan.xyz/wrtag/tags"
)

func init() {
	flag := flag.CommandLine
	flag.Usage = func() {
		fmt.Fprintf(flag.Output(), "Usage:\n")
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] read  <tag>... -- <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] write ( <tag> <value>... , )... -- <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s [<options>] clear <tag>... -- <path>...\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "Examples:\n")
		fmt.Fprintf(flag.Output(), "  $ %s read -- a.flac b.flac c.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s read artist title -- a.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s read -properties -- a.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s read -properties title length -- a.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s write album \"album name\" -- x.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s write artist \"Sensient\" , genres \"psy\" \"minimal\" \"techno\" -- dir/*.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s write artist \"Sensient\" , genres \"psy\" \"minimal\" \"techno\" -- dir/\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s clear -- a.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ %s clear lyrics artist_credit -- *.flac\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "See also:\n")
		fmt.Fprintf(flag.Output(), "  $ %s read -h\n", flag.Name())
	}
}

func main() {
	defer flags.ExitError()
	flags.Parse()

	if flag.NArg() == 0 {
		slog.Error("no command provided")
		return
	}

	switch command, args := flag.Arg(0), flag.Args()[1:]; command {
	case "read":
		flag := flag.NewFlagSet(command, flag.ExitOnError)
		var (
			withProperties = flag.Bool("properties", false, "read file properties like length and bitrate")
		)
		flag.Parse(args)

		args, paths := splitArgPaths(flag.Args())
		keys := parseTagKeys(args)
		if err := iterFiles(paths, func(p string) error { return read(p, *withProperties, keys) }); err != nil {
			slog.Error("process read", "err", err)
			return
		}
	case "write":
		args, paths := splitArgPaths(args)
		keyValues := parseTagKeyValues(args)
		if err := iterFiles(paths, func(p string) error { return write(p, keyValues) }); err != nil {
			slog.Error("process write", "err", err)
			return
		}
	case "clear":
		args, paths := splitArgPaths(args)
		keys := parseTagKeys(args)
		if err := iterFiles(paths, func(p string) error { return clear(p, keys) }); err != nil {
			slog.Error("process clear", "err", err)
			return
		}
	default:
		slog.Error("unknown command", "command", command)
		return
	}
}

func read(path string, withProperties bool, keys map[string]struct{}) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()

	wantKey := func(k string) bool {
		if len(keys) == 0 {
			return true
		}
		_, want := keys[k]
		return want
	}

	file.ReadAll(func(k string, vs []string) bool {
		if !wantKey(k) {
			return true
		}
		for _, v := range vs {
			fmt.Printf("%s\t%s\t%s\n", path, k, v)
		}
		return true
	})
	if !withProperties {
		return nil
	}

	if k := "length"; wantKey(k) {
		fmt.Printf("%s\t%s\t%s\n", path, k, formatDuration(file.Length()))
	}
	if k := "bitrate"; wantKey(k) {
		fmt.Printf("%s\t%s\t%d\n", path, k, file.Bitrate())
	}
	if k := "sample_rate"; wantKey(k) {
		fmt.Printf("%s\t%s\t%d\n", path, k, file.SampleRate())
	}
	if k := "num_channels"; wantKey(k) {
		fmt.Printf("%s\t%s\t%d\n", path, k, file.NumChannels())
	}
	return nil
}

func write(path string, keyValues map[string][]string) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()
	for k, vs := range keyValues {
		file.Write(k, vs...)
	}
	if err := file.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func clear(path string, keys map[string]struct{}) error {
	file, err := tags.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer file.Close()
	if len(keys) == 0 {
		file.ClearAll()
	} else {
		for k := range keys {
			file.Clear(k)
		}
	}
	if err := file.Save(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func splitArgPaths(argPaths []string) (args []string, paths []string) {
	if i := slices.Index(argPaths, "--"); i >= 0 {
		return argPaths[:i], argPaths[i+1:]
	}
	return nil, argPaths // no "--", presume paths
}

func parseTagKeys(args []string) map[string]struct{} {
	var r = map[string]struct{}{}
	for _, k := range args {
		r[k] = struct{}{}
	}
	return r
}

func parseTagKeyValues(args []string) map[string][]string {
	r := make(map[string][]string)
	var k string
	for _, v := range args {
		if v == "," {
			k = ""
			continue
		}
		if k == "" {
			k = v
			r[k] = nil
			continue
		}
		r[k] = append(r[k], v)
	}
	return r
}

func iterFiles(paths []string, f func(p string) error) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}
	var pathErrs []error
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}

		switch info.Mode().Type() {
		// recurse if dir, only attempt when CanRead
		case os.ModeDir:
			err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.Type().IsRegular() {
					return nil
				}
				if !tags.CanRead(path) {
					return nil
				}
				if err := f(path); err != nil {
					pathErrs = append(pathErrs, err)
					return nil
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("walk: %w", err)
			}
		// otherwise try directly, bubble errors
		default:
			if err := f(p); err != nil {
				pathErrs = append(pathErrs, err)
				continue
			}
		}
	}
	return errors.Join(pathErrs...)
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
