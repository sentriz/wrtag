package researchlink

import (
	"errors"
	"fmt"
	"strings"
	texttemplate "text/template"
	"time"
)

type source struct {
	name     string
	template *texttemplate.Template
}

type Querier struct {
	sources []source
}

func (q *Querier) IterSources(f func(string, *texttemplate.Template)) {
	for _, s := range q.sources {
		f(s.name, s.template)
	}
}

func (q *Querier) AddSource(name, templRaw string) error {
	templ, err := texttemplate.New("template").Funcs(funcMap).Parse(templRaw)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	q.sources = append(q.sources, source{
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

func (q *Querier) Search(query Query) ([]SearchResult, error) {
	var results []SearchResult
	var buildErrs []error
	for _, s := range q.sources {
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
