package coverselect_test

import (
	"testing"

	"go.senan.xyz/wrtag/coverselect"
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
			var s coverselect.Selection
			for _, c := range test.covers {
				s.Update(c)
			}
			if string(s) != test.expected {
				t.Errorf("with covers %v expected %q got %q", test.covers, test.expected, s)
			}
		})
	}
}
