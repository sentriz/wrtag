package researchlink

import (
	"errors"
	"fmt"
	"iter"
	"strings"
	texttemplate "text/template"
	"time"
)

type source struct {
	name     string
	template *texttemplate.Template
}

type Builder struct {
	sources []source
}

func (b *Builder) IterSources() iter.Seq2[string, *texttemplate.Template] {
	return func(yield func(string, *texttemplate.Template) bool) {
		for _, s := range b.sources {
			if !yield(s.name, s.template) {
				break
			}
		}
	}
}

func (b *Builder) AddSource(name, templRaw string) error {
	templ, err := texttemplate.New("template").Funcs(funcMap).Parse(templRaw)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	b.sources = append(b.sources, source{
		name:     name,
		template: templ,
	})
	return nil
}

type Query struct {
	Artist string
	Album  string
	UPC    string
	Date   time.Time
}

type SearchResult struct {
	Name, URL string
}

func (b *Builder) Build(query Query) ([]SearchResult, error) {
	var results []SearchResult
	var buildErrs []error
	for _, s := range b.sources {
		var buff strings.Builder
		if err := s.template.Execute(&buff, query); err != nil {
			buildErrs = append(buildErrs, fmt.Errorf("%s: %w", s.name, err))
			continue
		}
		results = append(results, SearchResult{Name: s.name, URL: buff.String()})
	}
	return results, errors.Join(buildErrs...)
}

var funcMap = texttemplate.FuncMap{
	"join": func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
}
