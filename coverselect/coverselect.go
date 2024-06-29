package coverselect

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Compare ranks two potential cover paths, suitable for [slices.SortFunc].
func Compare(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a, b := artTypePos(a), artTypePos(b); a != b {
		return a - b
	}
	if a, b := num(a), num(b); a != b {
		return a - b
	}
	if a, b := filetypePos(a), filetypePos(b); a != b {
		return a - b
	}
	return 0
}

func IsCover(p string) bool {
	p = filepath.Ext(p)
	p = strings.ToLower(p)
	_, ok := filetypePriorities[p]
	return ok
}

type Selection string

func (h *Selection) Update(other string) {
	if *h == "" {
		*h = Selection(other)
		return
	}
	if Compare(string(*h), other) > 0 {
		*h = Selection(other)
	}
}

var filetypePriorities = map[string]int{
	".png":  2,
	".jpg":  1,
	".jpeg": 1,
	".bmp":  1,
	".gif":  1,
}

func filetypePos(path string) int {
	return -filetypePriorities[filepath.Ext(path)]
}

var artTypePriorities = map[string]int{
	"front":    3,
	"cover":    3,
	"album":    3,
	"folder":   2,
	"albumart": 2,
	"scan":     1,
	"back":     0, // ignore
	"artist":   0, // ignore
}

var artTypeExpr *regexp.Regexp

func init() {
	var quoted []string
	for k := range artTypePriorities {
		quoted = append(quoted, regexp.QuoteMeta(k))
	}
	quoteExpr := strings.Join(quoted, "|")
	artTypeExpr = regexp.MustCompile(quoteExpr)
}

func artTypePos(path string) int {
	m := artTypeExpr.FindAllString(path, -1)
	if len(m) == 0 {
		return 0
	}
	var p int
	for _, artType := range m {
		p += artTypePriorities[artType]
	}
	return -p
}

var numExpr = regexp.MustCompile(`\d+`)

func num(path string) int {
	m := numExpr.FindAllString(path, -1)
	if len(m) == 0 {
		return 0
	}
	var p int = 1
	for _, nstr := range m {
		n, _ := strconv.Atoi(nstr)
		p *= n
	}
	return p
}
