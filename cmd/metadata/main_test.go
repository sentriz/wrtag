package main

import (
	_ "embed"
	"os"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/cmd/internal/testcmds"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"metadata":           func() int { main(); return 0 },
		"create-audio-files": func() int { testcmds.CreateAudioFiles(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
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
