package fileutil_test

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/fileutil"
)

func TestSafePath(t *testing.T) {
	t.Parallel()

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

func TestSafePathUnicode(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", fileutil.SafePathUnicode("hello"))
	assert.Equal(t, "hello", fileutil.SafePathUnicode("hello/"))
	assert.Equal(t, "hello a", fileutil.SafePathUnicode("hello/a"))
	assert.Equal(t, "hello a", fileutil.SafePathUnicode("hello / a"))
	assert.Equal(t, "hello", fileutil.SafePathUnicode("hel\x00lo"))
	assert.Equal(t, "a b", fileutil.SafePathUnicode("a  b"))
	assert.Equal(t, "(2004) Kesto (234.484)", fileutil.SafePathUnicode("(2004) Kesto (234.48:4)"))
	assert.Equal(t, "01.33 Rähinä I Mayhem I", fileutil.SafePathUnicode("01.33 Rähinä I Mayhem I"))
	assert.Equal(t, "50 ¢.flac", fileutil.SafePathUnicode("50 ¢.flac"))
	assert.Equal(t, "(2007) ✝", fileutil.SafePathUnicode("(2007) ✝"))
	assert.Equal(t, "_", fileutil.SafePathUnicode(">///<"))
}

func TestTruncateFilename(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello.flac", fileutil.TrimLength("hello.flac", 255))
	assert.Equal(t, "hello.flac", fileutil.TrimLength("hello.flac", 10))
	assert.Equal(t, "hell.flac", fileutil.TrimLength("hello.flac", 9))
	assert.Equal(t, "h.flac", fileutil.TrimLength("hello.flac", 6))
	assert.Equal(t, "hell", fileutil.TrimLength("hello.flac", 4))       // ext doesn't fit
	assert.Equal(t, "abc.flac", fileutil.TrimLength("abc def.flac", 8)) // trailing space trimmed

	long := strings.Repeat("a", 300) + ".flac"
	assert.Equal(t, strings.Repeat("a", 250)+".flac", fileutil.TrimLength(long, 255))

	assert.Equal(t, "ähn", fileutil.TrimLength("ähnlich", 3)) // unicode-aware

	// only the basename gets truncated, leading directories are left alone
	assert.Equal(t, "/very long dir name not truncated/hell.flac", fileutil.TrimLength("/very long dir name not truncated/hello.flac", 9))
}

func TestWalkLeaves(t *testing.T) {
	t.Parallel()

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

	require.Len(t, act, len(exp))

	sort.Strings(act)
	sort.Strings(exp)
	require.Equal(t, exp, act)
}
