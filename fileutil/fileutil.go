package fileutil

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rainycape/unidecode"
	"golang.org/x/text/unicode/norm"
)

func GlobEscape(path string) string {
	var r strings.Builder
	for _, c := range path {
		switch c {
		case '*', '?', '[':
			r.WriteRune('[')
			r.WriteRune(c)
			r.WriteRune(']')
		default:
			r.WriteRune(c)
		}
	}
	return r.String()
}

func GlobDir(dir, pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(GlobEscape(dir), pattern))
}

var safePathReplacer = strings.NewReplacer(
	// unixy
	"\x00", "",
	string(filepath.Separator), " ",

	// windows
	`<`, "",
	`>`, "",
	`:`, "",
	`"`, "",
	`/`, "",
	`\`, "",
	`|`, "",
	`?`, "",
	`*`, "",
)

func SafePath(path string) string {
	path = safePathReplacer.Replace(path)
	path = normUnidecode(path)
	path = safePathReplacer.Replace(path) // some unidecode replaces can result in slashes
	path = strings.Join(strings.Fields(path), " ")
	return path
}

// normUnidecode tries to be compatible with beets.io's version, though there are some slight differences.
// see https://github.com/beetbox/beets/blob/master/beets/library.py
//   - unicodedata.normalize('NFC', path) (from https://docs.python.org/3/library/unicodedata.html)
//   - unidecode(path)                    (from https://pypi.org/project/unicode/)
func normUnidecode(text string) string {
	text = norm.NFC.String(text)
	text = unidecode.Unidecode(text)
	return text
}

func WalkLeaves(root string, fn func(path string, d fs.DirEntry) error) error {
	var lastDepth int
	var lastPath string
	var lastDirEntry fs.DirEntry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		path = filepath.Clean(path)
		depth := strings.Count(path, string(filepath.Separator))
		var ferr error
		if depth <= lastDepth {
			ferr = fn(lastPath, lastDirEntry)
		}
		lastDepth, lastPath, lastDirEntry = depth, path, d
		return ferr
	})
	if err != nil {
		return err
	}
	if lastPath != "" {
		return fn(lastPath, lastDirEntry)
	}
	return nil
}

// HasPrefix checks a path has a prefix, making sure to respect path boundaries. So that /aa & /a does not match, but /a/a & /a does.
func HasPrefix(path, prefix string) bool {
	if prefix == "" {
		return true
	}
	if filepath.IsAbs(path) != filepath.IsAbs(prefix) {
		return false
	}

	prefix = filepath.Clean(prefix)
	path = filepath.Clean(path)

	return path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator))
}
