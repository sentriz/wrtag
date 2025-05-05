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

// https://github.com/sentriz/wrtag/issues/99
func TestNegativeScore(t *testing.T) {
	t.Parallel()

	var score float64
	diff := Differ(TagWeights{}, &score)

	diff("release", "Moon Boots", "Moon Boots")
	diff("artist", "Bird Bear Hare and Fish", "BBHF")
	diff("label", "", "SME Records")
	diff("catalogue num", "", "SECL-2324")
	diff("upc", "", "4547366368383")
	diff("media format", "", "CD")

	diff("track 1", "Bird Bear Hare and Fish – ウクライナ", "BBHF – ウクライナ")
	diff("track 2", "Bird Bear Hare and Fish – ライカ", "BBHF – ライカ")
	diff("track 3", "Bird Bear Hare and Fish – ダッシュボード", "BBHF – ダッシュボード")
	diff("track 4", "Bird Bear Hare and Fish – レプリカント", "BBHF – レプリカント")
	diff("track 5", "Bird Bear Hare and Fish – Hearts", "BBHF – Hearts")
	diff("track 6", "Bird Bear Hare and Fish – 夏の光", "BBHF – 夏の光")
	diff("track 7", "Bird Bear Hare and Fish – ページ", "BBHF – ページ")
	diff("track 8", "Bird Bear Hare and Fish – Wake Up", "BBHF – Wake Up")
	diff("track 9", "Bird Bear Hare and Fish – Different", "BBHF – Different")
	diff("track 10", "Bird Bear Hare and Fish – 骨の音", "BBHF – 骨の音")
	diff("track 11", "Bird Bear Hare and Fish – 次の火", "BBHF – 次の火")
	diff("track 12", "Bird Bear Hare and Fish – Work", "BBHF – Work")

	// probably we can come up with a better algorithm here to not produce a negative score
	assert.InEpsilon(t, -63, score, 1)
}

func TestNorm(t *testing.T) {
	t.Parallel()

	assert.Empty(t, norm(""))
	assert.Empty(t, norm(" "))
	assert.Equal(t, "123", norm(" 1!2!3 "))
	assert.Equal(t, "séan", norm("SÉan"))
	assert.Equal(t, "hello世界", norm("~~ 【 Hello, 世界。 】~~ 😉"))
}
