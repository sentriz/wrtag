package originfile

import (
	"fmt"
	"os"

	"go.senan.xyz/wrtag/fileutil"
	"gopkg.in/yaml.v3"
)

// https://github.com/x1ppy/gazelle-origin

const dirPat = "origin.y*ml"

func Find(dir string) (*OriginFile, error) {
	matches, err := fileutil.GlobDir(dir, dirPat)
	if err != nil {
		return nil, fmt.Errorf("glob for origin file: %w", err)
	}
	if len(matches) == 0 {
		return nil, nil
	}

	match := matches[0]
	res, err := Parse(match)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return res, nil
}

func Parse(path string) (*OriginFile, error) {
	f, err := os.Open(path) //nolint:gosec // path is only from user's config
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var res OriginFile
	if err := yaml.NewDecoder(f).Decode(&res); err != nil {
		return nil, fmt.Errorf("parse origin file: %w", err)
	}
	return &res, nil
}

type OriginFile struct {
	Artist          string `yaml:"Artist"`
	Name            string `yaml:"Name"`
	Edition         any    `yaml:"Edition"`
	EditionYear     int    `yaml:"Edition year"`
	Media           string `yaml:"Media"`
	CatalogueNumber string `yaml:"Catalog number"`
	RecordLabel     string `yaml:"Record label"`
	OriginalYear    int    `yaml:"Original year"`
	Format          string `yaml:"Format"`
	Encoding        string `yaml:"Encoding"`
	Log             any    `yaml:"Log"`
	Directory       string `yaml:"Directory"`
	Size            int    `yaml:"Size"`
	FileCount       int    `yaml:"File count"`
	InfoHash        any    `yaml:"Info hash"`
	Permalink       string `yaml:"Permalink"`
}

func (o *OriginFile) String() string {
	return fmt.Sprintf("%s - %s (%d) [%s #%s]", o.Artist, o.Name, o.EditionYear, o.RecordLabel, o.CatalogueNumber)
}
