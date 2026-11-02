package search

import (
	"errors"
	"strings"
	"sync"
)

// synonyms is an index's synonym map (FT.SYNUPDATE): each term lists the
// groups it belongs to, as "~group" terms that documents index alongside
// the word and queries add to its expansions.
type synonyms struct {
	mu    sync.RWMutex
	terms map[string][]string
	order []string
}

//nolint:staticcheck // RediSearch's reply texts
var (
	errSynonymTerms  = errors.New("Maximum synonym terms limit reached")
	errSynonymGroups = errors.New("Maximum group IDs per term limit reached")
)

const (
	maxSynonymTerms  = 1000000
	maxSynonymGroups = 4096
)

// UpdateSynonyms adds the terms (lower-cased) to a synonym group. Unless
// skip, the documents are indexed again, as RediSearch scans the keys
// again; with skip, documents indexed before keep their old terms until
// their keys are written.
func (idx *Index) UpdateSynonyms(group string, terms []string, skip bool) error {
	s := idx.syn
	s.mu.Lock()
	if s.terms == nil {
		s.terms = make(map[string][]string)
	}
	id := "~" + group
	lowered := make([]string, len(terms))
	added := make(map[string]bool)
	for i, t := range terms {
		lowered[i] = strings.ToLower(t)
		if groups, ok := s.terms[lowered[i]]; ok {
			if !hasString(groups, id) && len(groups) >= maxSynonymGroups {
				s.mu.Unlock()
				return errSynonymGroups
			}
		} else {
			added[lowered[i]] = true
		}
	}
	if len(s.terms)+len(added) > maxSynonymTerms {
		s.mu.Unlock()
		return errSynonymTerms
	}
	for _, t := range lowered {
		groups, ok := s.terms[t]
		if !ok {
			s.order = append(s.order, t)
		}
		if !hasString(groups, id) {
			s.terms[t] = append(groups, id)
		}
	}
	s.mu.Unlock()
	if !skip {
		idx.cacheSet.rebuildAll()
	}
	return nil
}

func hasString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// synonymGroups returns the "~group" terms of a term.
func (idx *Index) synonymGroups(term string) []string {
	s := idx.syn
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.terms[term]
}

// Synonyms returns the synonym map's terms, in the order they were added,
// with their group ids.
func (idx *Index) Synonyms() (terms []string, groups [][]string) {
	s := idx.syn
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.order {
		terms = append(terms, t)
		ids := make([]string, len(s.terms[t]))
		for i, g := range s.terms[t] {
			ids[i] = g[1:]
		}
		groups = append(groups, ids)
	}
	return terms, groups
}
