package natcmp

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompare(t *testing.T) {
	assert.Equal(t, -1, Compare("abc10", "abc100"))
	assert.Equal(t, +1, Compare("abc100", "abc10"))
	assert.Equal(t, +1, Compare("abc10.20 final.zip", "abc10.10 final.zip"))
	assert.Equal(t, 0, Compare("", ""))
	assert.Equal(t, 0, Compare("abc100", "abc100"))

	for range 16 {
		assert.Equal(t, sciValues, reSort(sciValues, Compare))
	}
	for range 16 {
		assert.Equal(t, docValues, reSort(docValues, Compare))
	}
}

func TestChunkify(t *testing.T) {
	expect := func(ch func() (string, int, bool), expStr string, expI int, expMore bool) {
		t.Helper()
		str, i, more := ch()
		assert.Equal(t, expStr, str)
		assert.Equal(t, expI, i)
		assert.Equal(t, expMore, more)
	}

	ch := chunkify("abc123a")
	expect(ch, "abc", 0, true)
	expect(ch, "", 123, true)
	expect(ch, "a", 0, true)
	expect(ch, "", 0, false)

	ch = chunkify("abc")
	expect(ch, "abc", 0, true)
	expect(ch, "", 0, false)

	ch = chunkify("123")
	expect(ch, "", 123, true)
	expect(ch, "", 0, false)

	ch = chunkify("")
	expect(ch, "", 0, false)
}

func reSort[T any](vs []T, cmp func(a, b T) int) []T {
	vs = slices.Clone(vs)
	rand.Shuffle(len(vs), func(i, j int) {
		vs[i], vs[j] = vs[j], vs[i]
	})
	slices.SortFunc(vs, cmp)
	return vs
}

var (
	sciValues = []string{
		"10X Radonius",
		"20X Radonius",
		"20X Radonius Prime",
		"30X Radonius",
		"40X Radonius",
		"200X Radonius",
		"1000X Radonius Maximus",
		"Allegia 6R Clasteron",
		"Allegia 50 Clasteron",
		"Allegia 50B Clasteron",
		"Allegia 51 Clasteron",
		"Allegia 500 Clasteron",
		"Alpha 2",
		"Alpha 2A",
		"Alpha 2A-900",
		"Alpha 2A-8000",
		"Alpha 100",
		"Alpha 200",
		"Callisto Morphamax",
		"Callisto Morphamax 500",
		"Callisto Morphamax 600",
		"Callisto Morphamax 700",
		"Callisto Morphamax 5000",
		"Callisto Morphamax 6000 SE",
		"Callisto Morphamax 6000 SE2",
		"Callisto Morphamax 7000",
		"Xiph Xlater 5",
		"Xiph Xlater 40",
		"Xiph Xlater 50",
		"Xiph Xlater 58",
		"Xiph Xlater 300",
		"Xiph Xlater 500",
		"Xiph Xlater 2000",
		"Xiph Xlater 5000",
		"Xiph Xlater 10000",
	}

	docValues = []string{
		"z1.doc",
		"z2.doc",
		"z3.doc",
		"z4.doc",
		"z5.doc",
		"z6.doc",
		"z7.doc",
		"z8.doc",
		"z9.doc",
		"z10.doc",
		"z11.doc",
		"z12.doc",
		"z13.doc",
		"z14.doc",
		"z15.doc",
		"z16.doc",
		"z17.doc",
		"z18.doc",
		"z19.doc",
		"z20.doc",
		"z100.doc",
		"z101.doc",
		"z102.doc",
	}
)
