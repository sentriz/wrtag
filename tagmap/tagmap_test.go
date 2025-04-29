package tagmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffer(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(TagWeights{}, &score)

	diff("x", "aaaaa", "aaaaa")
	diff("x", "aaaaa", "aaaaX")
	assert.InEpsilon(t, 90.0, score, 0) // 9 of 10 chars the same
}

func TestDiffWeightsLowerBound(t *testing.T) {
	t.Parallel()

	weights := TagWeights{
		"label":         0,
		"catalogue num": 0,
	}

	var score float64
	diff := Differ(weights, &score)

	// all the same, but label/catalogue num mismatch
	diff("label", "Columbia", "uh some other label")
	diff("catalogue num", "Columbia", "not the same catalogue num")

	diff("track 1", "The Day I Met God", "The Day I Met God")
	diff("track 2", "Catholic Day", "Catholic Day")
	diff("track 3", "Nine Plan Failed", "Nine Plan Failed")
	diff("track 4", "Family of Noise", "Family of Noise")
	diff("track 5", "Digital Tenderness", "Digital Tenderness")

	// but that's fine since we gave those 0 weight
	assert.InEpsilon(t, 100.0, score, 0)
}

func TestDiffWeightsUpperBound(t *testing.T) {
	t.Parallel()

	weights := TagWeights{
		"label":         2,
		"catalogue num": 2,
	}

	var score float64
	diff := Differ(weights, &score)

	// all the same, but label/catalogue num mismatch
	diff("label", "Columbia", "uh some other label")
	diff("catalogue num", "Columbia", "not the same catalogue num")

	diff("track 1", "The Day I Met God", "The Day I Met God")
	diff("track 2", "Catholic Day", "Catholic Day")
	diff("track 3", "Nine Plan Failed", "Nine Plan Failed")
	diff("track 4", "Family of Noise", "Family of Noise")
	diff("track 5", "Digital Tenderness", "Digital Tenderness")

	// bad score since we really care about label / catalogue num
	assert.InDelta(t, 32.0, score, 1)
}

func TestDiffNorm(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(TagWeights{}, &score)

	diff("label", "Columbia", "COLUMBIA")
	diff("catalogue num", "CLO LP 3", "CLOLP3")

	require.InEpsilon(t, 100.0, score, 0) // we don't care about case or spaces
}

func TestDiffIgnoreMissing(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(TagWeights{}, &score)

	diff("label", "", "COLUMBIA")
	diff("catalogue num", "CLO LP 3", "CLOLP3")

	assert.InEpsilon(t, 100.0, score, 0)
}

func TestNorm(t *testing.T) {
	t.Parallel()

	assert.Empty(t, norm(""))
	assert.Empty(t, norm(" "))
	assert.Equal(t, "123", norm(" 1!2!3 "))
	assert.Equal(t, "séan", norm("SÉan"))
	assert.Equal(t, "hello世界", norm("~~ 【 Hello, 世界。 】~~ 😉"))
}
