package main

import (
	_ "embed"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"metadata": func() int { main(); return 0 },
		"flac":     func() int { mainFile(); return 0 },
		"mp3":      func() int { mainFile(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
}

//go:embed testdata/empty.flac
var emptyFlac []byte

//go:embed testdata/empty.mp3
var emptyMP3 []byte

func mainFile() {
	var d []byte
	switch name := os.Args[0]; name {
	case "mp3":
		d = emptyMP3
	case "flac":
		d = emptyFlac
	default:
		log.Fatalf("unknown filetype %q\n", name)
	}

	flag.Parse()

	path := flag.Arg(0)
	if path == "" {
		log.Fatalf("no path provided")
	}

	if err := os.WriteFile(path, d, 0666); err != nil {
		log.Fatalf("write file: %v\n", err)
	}
}

func TestParseTagMap(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string][]string{}, parseTagKeyValues(nil))
	assert.Equal(t, map[string][]string{}, parseTagKeyValues([]string{","}))
	assert.Equal(t, map[string][]string{}, parseTagKeyValues([]string{",", ","}))
	assert.Equal(t, map[string][]string{"genres": nil}, parseTagKeyValues([]string{"genres"}))
	assert.Equal(t, map[string][]string{"genres": {"a"}}, parseTagKeyValues([]string{"genres", "a"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b"}}, parseTagKeyValues([]string{"genres", "a", "b"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}}, parseTagKeyValues([]string{"genres", "a", "b", "c"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}}, parseTagKeyValues([]string{"genres", "a", "b", "c", ","}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}, "artists": nil}, parseTagKeyValues([]string{"genres", "a", "b", "c", ",", "artists"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}, "artists": {"a", "b"}}, parseTagKeyValues([]string{"genres", "a", "b", "c", ",", "artists", "a", "b"}))
	assert.Equal(t, map[string][]string{"genres": {"a", "b", "c"}, "artists": {"a", "b", "c"}}, parseTagKeyValues([]string{"genres", "a", "b", "c", ",", "artists", "a", "b", "c"}))
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "00:00", formatDuration(0))
	assert.Equal(t, "00:01", formatDuration(1*time.Second))
	assert.Equal(t, "00:59", formatDuration(59*time.Second))
	assert.Equal(t, "01:00", formatDuration(60*time.Second))
	assert.Equal(t, "01:01", formatDuration(61*time.Second))
	assert.Equal(t, "05:30", formatDuration((5*time.Minute)+(30*time.Second)))
	assert.Equal(t, "300:00", formatDuration(5*time.Hour))
}
