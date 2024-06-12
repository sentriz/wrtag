package natcmp

import (
	"cmp"
	"strconv"
)

// Compare performs a natural comparison between two strings, a and b, suitable for
// use with [slices.SortFunc] by returning an integer comparing their natural order:
//
//	-1 if a < b
//	 0 if a == b
//	 1 if a > b
//
// Natural sorting orders numeric substrings in ascending order and
// string substrings using lexicographical order.
//
// Reference: https://web.archive.org/web/20210803201519/http://davekoelle.com/alphanum.html
func Compare(a, b string) int {
	ach, bch := chunkify(a), chunkify(b)
	for {
		astr, aint, amore := ach()
		bstr, bint, bmore := bch()
		switch {
		case !amore && !bmore:
			return 0
		case !amore:
			return -1
		case !bmore:
			return +1
		}
		if c := cmp.Compare(astr, bstr); c != 0 {
			return c
		}
		if c := cmp.Compare(aint, bint); c != 0 {
			return c
		}
	}
}

func chunkify(str string) func() (string, int, bool) {
	var start, end int
	return func() (string, int, bool) {
		if end >= len(str) {
			return "", 0, false
		}
		start = end
		isDigit := isAsciiDigit(str[start])
		for end < len(str) && isAsciiDigit(str[end]) == isDigit {
			end++
		}
		token := str[start:end]
		if isDigit {
			n, _ := strconv.Atoi(token)
			return "", n, true
		}
		return token, 0, true
	}
}

func isAsciiDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
