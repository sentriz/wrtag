package tags

import (
	"path/filepath"
	"slices"
	"strings"

	"go.senan.xyz/taglib"
)

func CanRead(absPath string) bool {
	// Extensions taken from fileref.cpp in taglib.
	// Note, even with this many options in the switch/case, this is still >3x faster than checking a map[string]struct{}
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".3g2", ".aac", ".afc", ".aif", ".aifc", ".aiff", ".ape", ".asf", ".dff", ".dsdiff", ".dsf", ".flac",
		".it", ".m4a", ".m4b", ".m4p", ".m4r", ".m4v", ".mod", ".module", ".mp2", ".mp3", ".mp4", ".mpc", ".nst",
		".oga", ".ogg", ".opus", ".s3m", ".shn", ".spx", ".tta", ".wav", ".wma", ".wow", ".wv", ".xm":
		return true
	}
	return false
}

func ReadTags(path string) (map[string][]string, error) {
	return taglib.ReadTags(path)
}

type WriteOption = taglib.WriteOption

const (
	Clear = taglib.Clear
)

func WriteTags(path string, tags map[string][]string, opts WriteOption) error {
	return taglib.WriteTags(path, tags, opts)
}

func ReadImage(path string) ([]byte, error) {
	return taglib.ReadImage(path)
}

type Properties = taglib.Properties

func ReadProperties(path string) (Properties, error) {
	return taglib.ReadProperties(path)
}

func Equal(a, b map[string][]string) bool {
	// not using maps.EqualFunc(x, slices.Equal) here since we don't care
	// about a difference in not present vs present but 0 len
	for k, vs := range a {
		if ovs := b[k]; !slices.Equal(vs, ovs) {
			return false
		}
	}
	for k, vs := range b {
		if ovs := a[k]; !slices.Equal(vs, ovs) {
			return false
		}
	}
	return true
}
