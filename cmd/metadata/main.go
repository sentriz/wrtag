package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"go.senan.xyz/wrtag/cmd/internal/wrtaglog"
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
		fmt.Fprintf(flag.Output(), "  # <tag> is an audio metadata tag key\n")
		fmt.Fprintf(flag.Output(), "  # <value> is an audio metadata tag value\n")
		fmt.Fprintf(flag.Output(), "  # <path> is path(s) to audio files, dir(s) to find audio files in, or \"-\" for list audio file paths from stdin\n")
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
		fmt.Fprintf(flag.Output(), "  $ find x/ -type f | %s write artist \"Sensient\" , album \"Blue Neevus\" -\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ find y/ -type f | %s read artist title -\n", flag.Name())
		fmt.Fprintf(flag.Output(), "  $ find y/ -type f -name \"*extended*\" | %s read -properties length -\n", flag.Name())
		fmt.Fprintf(flag.Output(), "\n")
		fmt.Fprintf(flag.Output(), "See also:\n")
		fmt.Fprintf(flag.Output(), "  $ %s read -h\n", flag.Name())
	}
}

func main() {
	defer wrtaglog.Setup()()
	flag.Parse()

	if flag.NArg() == 0 {
		slog.Error("no command provided")
		return
	}

	switch command, args := flag.Arg(0), flag.Args()[1:]; command {
	case "read":
		flag := flag.NewFlagSet(command, flag.ExitOnError)
		var (
			withProperties = flag.Bool("properties", false, "Read file properties like length and bitrate")
		)
		flag.Parse(args)

		out := bufio.NewWriter(os.Stdout)
		defer out.Flush()

		args, paths := splitArgPaths(flag.Args())
		if err := iterFiles(paths, func(p string) error { return cmdRead(out, p, *withProperties, args) }); err != nil {
			slog.Error("process read", "err", err)
			return
		}
	case "write":
		args, paths := splitArgPaths(args)
		keyValues := parseTagKeyValues(args)
		if err := iterFiles(paths, func(p string) error { return cmdWrite(p, keyValues) }); err != nil {
			slog.Error("process write", "err", err)
			return
		}
	case "clear":
		args, paths := splitArgPaths(args)
		if err := iterFiles(paths, func(p string) error { return cmdClear(p, args) }); err != nil {
			slog.Error("process clear", "err", err)
			return
		}
	default:
		slog.Error("unknown command", "command", command)
		return
	}
}

func cmdRead(to io.Writer, path string, withProperties bool, keys []string) error {
	t, err := tags.ReadTags(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if len(keys) == 0 {
		for _, k := range slices.Sorted(maps.Keys(t)) {
			for _, v := range t[k] {
				fmt.Fprintf(to, "%s\t%s\t%s\n", path, k, v)
			}
		}
	} else {
		for _, k := range keys {
			for _, v := range t.Values(k) {
				fmt.Fprintf(to, "%s\t%s\t%s\n", path, tags.NormKey(k), v)
			}
		}
	}

	if !withProperties {
		return nil
	}

	properties, err := tags.ReadProperties(path)
	if err != nil {
		return err
	}

	wantProperty := func(k string) bool {
		if len(keys) == 0 {
			return true
		}
		if slices.Contains(keys, k) {
			return true
		}
		return false
	}

	if k := "length"; wantProperty(k) {
		fmt.Fprintf(to, "%s\t%s\t%s\n", path, k, formatDuration(properties.Length))
	}
	if k := "bitrate"; wantProperty(k) {
		fmt.Fprintf(to, "%s\t%s\t%d\n", path, k, properties.Bitrate)
	}
	if k := "sample_rate"; wantProperty(k) {
		fmt.Fprintf(to, "%s\t%s\t%d\n", path, k, properties.SampleRate)
	}
	if k := "channels"; wantProperty(k) {
		fmt.Fprintf(to, "%s\t%s\t%d\n", path, k, properties.Channels)
	}

	return nil
}

func cmdWrite(path string, keyValues map[string][]string) error {
	var t = tags.Tags{}
	for k, vs := range keyValues {
		t.Set(k, vs...)
	}
	if err := tags.WriteTags(path, t, 0); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func cmdClear(path string, keys []string) error {
	if len(keys) == 0 {
		if err := tags.WriteTags(path, tags.Tags{}, tags.Clear); err != nil {
			return err
		}
		return nil
	}
	var t = tags.Tags{}
	for _, k := range keys {
		t.Set(k)
	}
	if err := tags.WriteTags(path, t, 0); err != nil {
		return err
	}
	return nil
}

func splitArgPaths(argPaths []string) (args []string, paths []string) {
	if len(argPaths) == 0 {
		return nil, nil
	}
	// UX exception for standalone "-", assume everything before is arg
	if i := len(argPaths) - 1; argPaths[i] == "-" {
		return argPaths[:i], argPaths[i:]
	}
	if i := slices.Index(argPaths, "--"); i >= 0 {
		return argPaths[:i], argPaths[i+1:]
	}
	return nil, argPaths // no delimiter so presume paths
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
		return errors.New("no paths provided")
	}

	var pathErrs []error
	for _, p := range paths {
		if p == "-" {
			// read paths from stdin if we have them
			sc := bufio.NewScanner(os.Stdin)
			for sc.Scan() {
				if err := f(sc.Text()); err != nil {
					pathErrs = append(pathErrs, err)
					continue
				}
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("scan stdin: %w", err)
			}
			continue
		}

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
