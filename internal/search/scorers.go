package search

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
)

// The scorers of RediSearch's ext/default.c. BM25STD is the default; the
// others score the result tree the iterators return for a document as
// RediSearch's do, including TFIDF's and BM25's division by the slop, the
// distance between the terms of an intersection.
var scorerNames = map[string]bool{
	"BM25STD": true, "BM25STD.NORM": true, "BM25STD.TANH": true, "BM25": true,
	"TFIDF": true, "TFIDF.DOCNORM": true, "DISMAX": true, "DOCSCORE": true, "HAMMING": true,
}

// ParseScorer checks a SCORER argument and returns its canonical name.
func ParseScorer(name string) (string, error) {
	up := strings.ToUpper(name)
	if !scorerNames[up] {
		return "", errors.New("No such scorer " + name) //nolint:staticcheck // RediSearch's reply text
	}
	return up, nil
}

// scorerOf is the scorer a query uses: its SCORER, else the default.
func scorerOf(opts QueryOptions) string {
	if opts.Scorer != "" {
		return opts.Scorer
	}
	if v := configValue("DEFAULT_SCORER"); v != nil && scorerNames[*v] {
		return *v
	}
	return "BM25STD"
}

// bm25stdFamily reports whether the scorer is BM25STD or one of its
// normalisations, which the plain match scores.
func bm25stdFamily(scorer string) bool { return strings.HasPrefix(scorer, "BM25STD") }

// score is scorer's score of document d, which the root returned as r.
func (e *scoreEngine) score(scorer string, r *rsResult, d *Document) float64 {
	switch scorer {
	case "TFIDF", "TFIDF.DOCNORM":
		norm := float64(d.maxFreq())
		if scorer == "TFIDF.DOCNORM" {
			norm = float64(d.length)
		}
		if e.stats.score == 0 || norm == 0 {
			return 0
		}
		return e.stats.score * e.tfidf(r) / norm / float64(e.slop(r, d))
	case "BM25":
		return e.stats.score * e.bm25Legacy(r) / float64(e.slop(r, d))
	case "DISMAX":
		return e.stats.score * e.dismax(r)
	case "DOCSCORE":
		return e.stats.score
	case "HAMMING":
		// Hamming distance between payloads; CacheStorm keeps none.
		return 0
	}
	return e.stats.score * e.bm25(r, d)
}

// tfidfIDF is the IDF TFIDF and BM25 use: logb(1 + N/n).
func (e *scoreEngine) tfidfIDF(term string) float64 {
	n := e.stats.termDocs(term)
	if n == 0 {
		n = 1
	}
	return math.Logb(float64(float32(1)) + float64(e.stats.Docs)/float64(n))
}

func (e *scoreEngine) tfidf(r *rsResult) float64 {
	switch {
	case r.typ == resTerm:
		return r.weight * float64(r.freq) * e.tfidfIDF(r.term)
	case r.typ&(resUnion|resIntersect) != 0:
		ret := 0.0
		for _, c := range r.children {
			ret += e.tfidf(c)
		}
		return r.weight * ret
	}
	return r.weight * float64(r.freq)
}

// bm25Legacy is the BM25 scorer RediSearch kept beside BM25STD: b 0.5, and
// the average document length where the document's own belongs.
func (e *scoreEngine) bm25Legacy(r *rsResult) float64 {
	const k1, b = float32(1.2), float32(0.5)
	f := float64(r.freq)
	denom := func() float64 { return f + float64(k1)*(float64(1-b)+float64(b)*e.stats.avgLen()) }
	switch {
	case r.typ == resTerm:
		return e.tfidfIDF(r.term) * f / denom()
	case r.typ&(resUnion|resIntersect) != 0:
		ret := 0.0
		for _, c := range r.children {
			ret += e.bm25Legacy(c)
		}
		return ret * r.weight
	case f != 0:
		return r.weight * f / denom()
	}
	return 0
}

// dismax sums an intersection's children and takes a union's best.
func (e *scoreEngine) dismax(r *rsResult) float64 {
	ret := 0.0
	switch {
	case r.typ&resIntersect != 0:
		for _, c := range r.children {
			ret += e.dismax(c)
		}
	case r.typ&resUnion != 0:
		for _, c := range r.children {
			ret = max(ret, e.dismax(c))
		}
	default:
		ret = float64(r.freq)
	}
	return r.weight * ret
}

// maxFreq is the frequency of the document's most frequent term.
func (d *Document) maxFreq() uint32 {
	m := uint32(0)
	for _, tf := range d.terms {
		m = max(m, tf.freq)
	}
	return m
}

// slop is RediSearch's IndexResult_MinOffsetDelta: for an aggregate, the
// sum over consecutive children of their nearest positions'
// distance; 1 for a single term.
func (e *scoreEngine) slop(r *rsResult, d *Document) int {
	if r.typ&(resUnion|resIntersect) == 0 || len(r.children) <= 1 {
		return 1
	}
	stream := e.idx.textStream(d)
	dist, num := 0, len(r.children)
	for i := 0; i < num; {
		for i < num && !hasOffsets(r.children[i]) {
			i++
		}
		if i == num {
			break
		}
		v1 := e.offsets(r.children[i], stream)
		i++
		for i < num && !hasOffsets(r.children[i]) {
			i++
		}
		if i == num {
			if dist == 0 {
				dist = 100
			}
			break
		}
		v2 := e.offsets(r.children[i], stream)
		if len(v1) == 0 || len(v2) == 0 {
			continue
		}
		a, b := 0, 0
		cd := absDelta(v2[0], v1[0])
		for cd > 1 && a < len(v1) && b < len(v2) {
			cd = min(absDelta(v2[b], v1[a]), cd)
			if v2[b] > v1[a] {
				a++
			} else {
				b++
			}
		}
		dist += cd
	}
	if dist == 0 {
		return num - 1
	}
	return dist
}

func absDelta(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func hasOffsets(r *rsResult) bool {
	if r.typ == resTerm {
		return r.freq > 0
	}
	for _, c := range r.children {
		if hasOffsets(c) {
			return true
		}
	}
	return false
}

// offsets are the positions, from 1, of a result's terms in the document's
// text: a stem term ("+stem") is at the words that stem to it; an
// aggregate's are its children's, merged.
func (e *scoreEngine) offsets(r *rsResult, stream []string) []int {
	if r.typ != resTerm {
		var out []int
		for _, c := range r.children {
			out = append(out, e.offsets(c, stream)...)
		}
		sort.Ints(out)
		return out
	}
	var out []int
	stem, isStem := strings.CutPrefix(r.term, "+")
	for i, w := range stream {
		if w == r.term || isStem && w != stem && snowballEnglish(w) == stem {
			out = append(out, i+1)
		}
	}
	return out
}

// normalizeScores applies BM25STD.NORM (each score over the highest) and
// BM25STD.TANH (tanh of the score over BM25STD_TANH_FACTOR).
func normalizeScores(scorer string, hits []Hit) {
	switch scorer {
	case "BM25STD.NORM":
		top := 0.0
		for _, h := range hits {
			top = max(top, h.Score)
		}
		if top > 0 {
			for i := range hits {
				hits[i].Score /= top
			}
		}
	case "BM25STD.TANH":
		factor := 4.0
		if v := configValue("BM25STD_TANH_FACTOR"); v != nil {
			if f, err := strconv.ParseFloat(*v, 64); err == nil && f >= 1 {
				factor = f
			}
		}
		for i := range hits {
			hits[i].Score = cTanh(hits[i].Score / factor)
		}
	}
}

// cTanh is glibc's tanh, which rounds differently from math.Tanh in the
// last bit.
func cTanh(x float64) float64 {
	ax := math.Abs(x)
	var z float64
	switch {
	case math.IsNaN(x):
		return x
	case ax >= 22:
		z = 1
	case ax < 0x1p-55:
		return x
	case ax >= 1:
		t := math.Expm1(2 * ax)
		z = 1 - 2/(t+2)
	default:
		t := math.Expm1(-2 * ax)
		z = -t / (t + 2)
	}
	if x < 0 {
		return -z
	}
	return z
}
