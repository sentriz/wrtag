package main

import (
	"crypto/rand"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/tags"
	"go.senan.xyz/wrtag/tags/normtag"
)

//go:embed testdata/responses
var responses embed.FS

func TestMain(m *testing.M) {
	var t http.Transport
	t.RegisterProtocol("file", http.NewFileTransportFS(responses))

	http.DefaultTransport = &t

	os.Setenv("WRTAG_MB_BASE_URL", "file:///testdata/responses/musicbrainz/ws/2")
	os.Setenv("WRTAG_MB_RATE_LIMIT", "0")
	os.Setenv("WRTAG_CAA_BASE_URL", "file:///testdata/responses/coverartarchive")
	os.Setenv("WRTAG_CAA_RATE_LIMIT", "0")

	testscript.Main(m, map[string]func(){
		"wrtag":    main,
		"tag":      mainTag,
		"find":     mainFind,
		"touch":    mainTouch,
		"mime":     mainMIME,
		"mod-time": mainModTime,
		"rand":     mainRand,
	})
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
}

func mainTag() {
	flag.Parse()

	op := flag.Arg(0)
	switch op {
	case "write", "check":
	default:
		log.Fatalf("bad op %s", op)
	}

	pat := flag.Arg(1)
	paths := parsePattern(pat)
	if len(paths) == 0 {
		log.Fatalf("no paths to match pattern")
	}

	pairs := parseTagMap(flag.Args()[2:])

	var exit int
	for _, p := range paths {
		switch op {
		case "write":
			if err := ensureAudioFile(p); err != nil {
				log.Fatalf("ensure flac: %v", err)
			}
			var t = map[string][]string{}
			for k, vs := range pairs {
				normtag.Set(t, k, vs...)
			}
			if err := tags.WriteTags(p, t, 0); err != nil {
				log.Fatalf("write tag file: %v", err)
			}
		case "check":
			t, err := tags.ReadTags(p)
			if err != nil {
				log.Fatalf("read tags: %v", err)
			}
			for k, vs := range pairs {
				if got := normtag.Values(t, k); !slices.Equal(vs, got) {
					log.Printf("%s exp %q got %q", p, vs, got)
					exit = 1
				}
			}
		}
	}

	os.Exit(exit)
}

func mainFind() {
	maxDepth := flag.Int("max-depth", -1, "")
	flag.Parse()

	paths := flag.Args()
	sort.Strings(paths)

	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			path = filepath.Clean(path)
			if *maxDepth != -1 && strings.Count(path, string(filepath.Separator)) > *maxDepth {
				return nil
			}
			fmt.Println(path)
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}

func mainTouch() {
	flag.Parse()

	for _, p := range flag.Args() {
		if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
			log.Fatalf("mkdirall: %v", err)
		}
		if _, err := os.Create(p); err != nil {
			log.Fatalf("err creating: %v", err)
		}
	}
}

func mainMIME() {
	flag.Parse()

	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("error reading: %v", err)
	}

	mime := http.DetectContentType(data)
	fmt.Println(mime)
}

func mainModTime() {
	flag.Parse()

	pat := flag.Arg(0)
	paths := parsePattern(pat)
	if len(paths) == 0 {
		log.Fatalf("no paths to match pattern")
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			log.Fatalf("error stating: %v", err)
		}
		fmt.Println(info.ModTime().UnixNano())
	}
}

func mainRand() {
	flag.Parse()

	path, sizeStr := flag.Arg(0), flag.Arg(1)
	if path == "" || sizeStr == "" {
		log.Fatalf("bad args")
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("error creating: %v", err)
	}
	defer f.Close()

	size, _ := strconv.Atoi(sizeStr)
	_, _ = io.Copy(f, io.LimitReader(rand.Reader, int64(size)))
}

func parsePattern(pat string) []string {
	// assume the file exists if the pattern doesn't look like a glob
	if fileutil.GlobEscape(pat) == pat {
		return []string{pat}
	}
	paths, _ := filepath.Glob(pat)
	return paths
}

// copied from cmd/metadata.
func parseTagMap(args []string) map[string][]string {
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

var (
	//go:embed testdata/empty.flac
	emptyFLAC []byte
	//go:embed testdata/empty.m4a
	emptyM4A []byte
	//go:embed testdata/empty.mp3
	emptyMP3 []byte
)

func ensureAudioFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("make parents: %w", err)
	}

	var d []byte
	switch ext := filepath.Ext(path); ext {
	case ".flac":
		d = emptyFLAC
	case ".m4a":
		d = emptyM4A
	case ".mp3":
		d = emptyMP3
	}

	return os.WriteFile(path, d, os.ModePerm)
}
