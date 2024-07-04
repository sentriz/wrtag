package coverparse_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"go.senan.xyz/wrtag/coverparse"
)

func TestSelection(t *testing.T) {
	cases := []struct {
		name     string
		covers   []string
		expected string
	}{
		{
			name:     "empty covers slice",
			covers:   []string{},
			expected: "",
		},
		{
			name:     "without keywords or numbers case sensitive",
			covers:   []string{"Cover1.jpg", "cover2.png"},
			expected: "Cover1.jpg",
		},
		{
			name:     "without keywords or numbers",
			covers:   []string{"cover1.jpg", "cover2.png"},
			expected: "cover1.jpg",
		},
		{
			name:     "with keywords and numbers",
			covers:   []string{"cover12.jpg", "cover2.png", "special_cover1.jpg"},
			expected: "special_cover1.jpg",
		},
		{
			name:     "with keywords and numbers with type prio",
			covers:   []string{"cover12.jpg", "cover3.png", "back1.png", "special_cover2.jpg"},
			expected: "special_cover2.jpg",
		},
		{
			name:     "with keywords and numbers with type prio and filetype",
			covers:   []string{"cover12.jpg", "cover3.png", "back1.png", "back.png", "special_cover2.jpg", "special_cover2.png"},
			expected: "special_cover2.png",
		},
		{
			name:     "with keywords but without numbers",
			covers:   []string{"cover12.jpg", "cover_keyword.png"},
			expected: "cover_keyword.png",
		},
		{
			name:     "without keywords but with numbers",
			covers:   []string{"cover1.jpg", "cover12.png"},
			expected: "cover1.jpg",
		},
		{
			name:     "with same highest score",
			covers:   []string{"cover1.jpg", "cover2.jpg", "cover_special.jpg"},
			expected: "cover_special.jpg",
		},
		{
			name: "cds",
			covers: []string{
				"cd 2/cover art file A10 01.png",
				"cd 2/cover art file A10 01.png",
				"cd 1/cover art file A10 01.png",
				"cd 1/cover art file A10 02.png",
				"cd 2/album art file A10 01.png",
				"cd 2/album art file A10 01.png",
				"cd 1/album art file A10 01.png",
				"cd 1/album art file A10 02.png",
				"cd 1/album art file A10 02",
			},
			expected: "cd 1/cover art file A10 01.png",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var s coverparse.Front
			for _, c := range test.covers {
				s.Compare(c)
			}
			if string(s) != test.expected {
				t.Errorf("with covers %v expected %q got %q", test.covers, test.expected, s)
			}
		})
	}
}

func TestCoverSorting(t *testing.T) {
	cases := []struct {
		name     string
		expected []string
	}{
		{
			name:     "basic front and back",
			expected: []string{"front.png", "back.png"},
		},
		{
			name:     "numerical front order",
			expected: []string{"front 9 1.png", "front 10 2.png"},
		},
		{
			name:     "mixed types",
			expected: []string{"front.png", "cover.jpg", "album 3.png"},
		},
		{
			name:     "different art types",
			expected: []string{"albumart 2.png", "folder.bmp", "scan 1.jpg"},
		},
		{
			name:     "same prefix with different numbers",
			expected: []string{"front 9 4.png", "front 10 2.png", "front 10 3.png"},
		},
		{
			name:     "different file extensions",
			expected: []string{"front 9 1.png", "front 10 2.png", "front 10 2.jpeg"},
		},
		{
			name:     "various cover types",
			expected: []string{"cover.png", "front.jpg", "albumart 1.gif", "folder.bmp"},
		},
		{
			name:     "ignored art types",
			expected: []string{"album.png", "artist.png", "back.jpg"},
		},
		{
			name:     "same art type with numbers",
			expected: []string{"scan 1.gif", "scan 2.jpg", "scan 10.png"},
		},
		{
			name:     "sequential covers",
			expected: []string{"cover 1.png", "cover 2.jpg", "cover 3.png"},
		},
		{
			name:     "cd directories order",
			expected: []string{"CD 1/front.png", "CD 2/front.png"},
		},
		{
			name:     "cd directories reverse order",
			expected: []string{"CD 1/front.png", "CD 2/front.png"},
		},
		{
			name:     "multiple files in each cd",
			expected: []string{"CD 1/front 1.png", "CD 1/front 2.png", "CD 2/front 1.png", "CD 2/front 2.png"},
		},
		{
			name:     "front and back in each cd",
			expected: []string{"CD 1/front.png", "CD 2/front.png", "CD 1/back.png", "CD 2/back.png"},
		},
		{
			name:     "numerical front order in cds",
			expected: []string{"CD 1/front 9 1.png", "CD 1/front 10 2.png", "CD 2/front 9 1.png", "CD 2/front 10 2.png"},
		},
		{
			name:     "different file extensions in cds",
			expected: []string{"CD 1/front 9 1.png", "CD 1/front 10 2.png", "CD 2/front 9 1.png", "CD 2/front 10 2.jpeg"},
		},
		{
			name:     "various cover types in cds",
			expected: []string{"CD 1/cover.png", "CD 1/front.jpg", "CD 2/cover.png", "CD 2/front.jpg"},
		},
	}

	r := rand.New(rand.NewPCG(1, 2))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inp := slices.Clone(tc.expected)
			r.Shuffle(len(inp), func(i, j int) {
				inp[i], inp[j] = inp[j], inp[i]
			})
			slices.SortFunc(inp, coverparse.Compare)
			if !slices.Equal(inp, tc.expected) {
				t.Errorf("expected %q got %q", tc.expected, inp)
			}
		})
	}
}
