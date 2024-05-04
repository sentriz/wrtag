package fileutil_test

import (
	"io/fs"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/fileutil"
)

func TestSafePath(t *testing.T) {
	assert.Equal(t, "hello", fileutil.SafePath("hello"))
	assert.Equal(t, "hello", fileutil.SafePath("hello/"))
	assert.Equal(t, "hello a", fileutil.SafePath("hello/a"))
	assert.Equal(t, "hello a", fileutil.SafePath("hello / a"))
	assert.Equal(t, "hello", fileutil.SafePath("hel\x00lo"))
	assert.Equal(t, "a b", fileutil.SafePath("a  b"))
	assert.Equal(t, "(2004) Kesto (234.484)", fileutil.SafePath("(2004) Kesto (234.48:4)"))
	assert.Equal(t, "01.33 Rahina I Mayhem I", fileutil.SafePath("01.33 Rähinä I Mayhem I"))
	assert.Equal(t, "50 C .flac", fileutil.SafePath("50 ¢.flac"))
	assert.Equal(t, "(2007)", fileutil.SafePath("(2007) ✝")) // need to fix this
}

func TestWalkLeaves(t *testing.T) {
	var act []string
	require.NoError(t, fileutil.WalkLeaves("testdata/leaves", func(path string, d fs.DirEntry) error {
		act = append(act, path)
		return nil
	}))

	exp := []string{
		"testdata/leaves/b/a/b/c/leaf",
		"testdata/leaves/b/a/b/leaf",
		"testdata/leaves/b/d/b/c/leaf",
		"testdata/leaves/a/b/b/c/leaf",
		"testdata/leaves/a/d/b/c/leaf-a",
		"testdata/leaves/a/d/b/c/leaf-b",
		"testdata/leaves/a/d/b/c/leaf-c",
	}

	require.Equal(t, len(exp), len(act))

	sort.Strings(act)
	sort.Strings(exp)
	require.Equal(t, exp, act)
}
