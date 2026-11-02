package search

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

// IterProfile is one iterator of FT.PROFILE's "Iterators profile": its
// type, what it reads, how often it was read and, for readers, how many
// documents it expects.
type IterProfile struct {
	Child     *IterProfile
	Type      string
	Term      string
	QueryType string
	Children  []*IterProfile
	Reads     int
	Estimate  int
	HasTerm   bool
	Container bool
	Null      bool
}

// profIter counts the reads of an iterator as RediSearch's profile
// iterator does: every Read and SkipTo, less the one that found the end.
type profIter struct {
	rsIter
	count int
	eof   bool
}

func (p *profIter) read() (int, *rsResult) {
	p.count++
	rc, r := p.rsIter.read()
	if rc == readEOF {
		p.eof = true
	}
	return rc, r
}

func (p *profIter) skipTo(id int64) (int, *rsResult) {
	p.count++
	rc, r := p.rsIter.skipTo(id)
	if rc == readEOF {
		p.eof = true
	}
	return rc, r
}

func unwrap(it rsIter) rsIter {
	if p, ok := it.(*profIter); ok {
		return p.rsIter
	}
	return it
}

// Profile runs a query's iterators, to their end when limit is negative or
// until limit results, as FT.PROFILE does, and describes them.
func (idx *Index) Profile(st *store.Store, n Node, limit int) *IterProfile {
	c := idx.cache(st)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncLocked(idx, st)
	c.fillTerms(n)
	e := newScoreEngine(idx, c, c.stats)
	e.profile = true
	root := e.build(n, allFields)
	if root == nil {
		return &IterProfile{Type: "EMPTY", Estimate: -1}
	}
	for found := 0; limit < 0 || found < limit; {
		rc, _ := root.read()
		if rc == readEOF {
			break
		}
		if rc == readOK {
			found++
		}
	}
	return describeIter(root)
}

func describeIter(it rsIter) *IterProfile {
	if it == nil {
		return &IterProfile{Null: true}
	}
	p := &IterProfile{Estimate: -1}
	if w, ok := it.(*profIter); ok {
		p.Reads = w.count
		if w.eof {
			p.Reads--
		}
	}
	switch x := unwrap(it).(type) {
	case *termReader:
		p.Type, p.Term, p.HasTerm, p.Estimate = x.profType, x.profTerm, true, x.estimate()
	case *unionIter:
		p.Type, p.QueryType, p.Container = "UNION", x.queryType, true
		for _, c := range x.orig {
			p.Children = append(p.Children, describeIter(c))
		}
	case *intersectIter:
		p.Type, p.Container = "INTERSECT", true
		for _, c := range x.its {
			p.Children = append(p.Children, describeIter(c))
		}
	case *notIter:
		p.Type, p.Child = "NOT", describeIter(x.child)
	case *optionalIter:
		p.Type, p.Child = "OPTIONAL", describeIter(x.child)
	case *wildcardIter:
		p.Type = "WILDCARD"
	case *listIter:
		p.Type, p.Term, p.HasTerm, p.Estimate = x.profType, x.profTerm, true, x.estimate()
	default:
		p.Type = "EMPTY"
	}
	return p
}

// numericRangeTerm is how FT.PROFILE names a numeric range: its lowest and
// highest value.
func numericRangeTerm(lo, hi float64) string {
	g := func(f float64) string {
		if math.IsInf(f, 0) {
			if f > 0 {
				return "inf"
			}
			return "-inf"
		}
		return strconv.FormatFloat(f, 'g', 6, 64)
	}
	return fmt.Sprintf("%s - %s", g(lo), g(hi))
}

// ProfileExprType is how FT.PROFILE names an APPLY or FILTER step, by its
// expression's root.
func ProfileExprType(e *Expr) string {
	switch n := e.root.(type) {
	case fieldNode:
		return "Property " + n.name
	case callNode:
		return "Function " + n.fn
	case unaryOp:
		return "Inverted"
	case binaryOp:
		switch n.op {
		case "+", "-", "*", "/", "%", "^":
			return "Operator " + n.op
		}
		return "Predicate " + n.op
	}
	return "Literal " + strings.TrimSpace(e.root.eval(nil).String())
}
