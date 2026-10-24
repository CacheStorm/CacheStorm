package store

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

var (
	ErrStreamInvalidID  = errors.New("ERR Invalid stream ID specified as stream command argument")
	ErrStreamIDTooSmall = errors.New("ERR The ID specified in XADD is equal or smaller than the target stream top item")
	ErrStreamIDZero     = errors.New("ERR The ID specified in XADD must be greater than 0-0")
	ErrStreamExhausted  = errors.New("ERR The stream has exhausted the last possible ID, unable to add more items")
)

// StreamID is a parsed stream entry ID. Redis IDs are two unsigned 64-bit
// parts, compared numerically.
type StreamID struct {
	Ms  uint64
	Seq uint64
}

// MaxStreamID is the largest ID a stream can hold.
var MaxStreamID = StreamID{Ms: math.MaxUint64, Seq: math.MaxUint64}

func (id StreamID) String() string {
	return strconv.FormatUint(id.Ms, 10) + "-" + strconv.FormatUint(id.Seq, 10)
}

func (id StreamID) Compare(o StreamID) int {
	switch {
	case id.Ms < o.Ms:
		return -1
	case id.Ms > o.Ms:
		return 1
	case id.Seq < o.Seq:
		return -1
	case id.Seq > o.Seq:
		return 1
	}
	return 0
}

func (id StreamID) Less(o StreamID) bool { return id.Compare(o) < 0 }

func (id StreamID) IsZero() bool { return id.Ms == 0 && id.Seq == 0 }

// Next returns the smallest ID greater than id; ok is false at MaxStreamID.
func (id StreamID) Next() (StreamID, bool) {
	if id.Seq == math.MaxUint64 {
		if id.Ms == math.MaxUint64 {
			return id, false
		}
		return StreamID{Ms: id.Ms + 1}, true
	}
	return StreamID{Ms: id.Ms, Seq: id.Seq + 1}, true
}

// Prev returns the greatest ID smaller than id; ok is false at 0-0.
func (id StreamID) Prev() (StreamID, bool) {
	if id.Seq == 0 {
		if id.Ms == 0 {
			return id, false
		}
		return StreamID{Ms: id.Ms - 1, Seq: math.MaxUint64}, true
	}
	return StreamID{Ms: id.Ms, Seq: id.Seq - 1}, true
}

func parseUint64Strict(s string) (uint64, bool) {
	if s == "" || s[0] == '+' || s[0] == '-' {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil
}

// ParseStreamID parses "<ms>-<seq>" or a bare "<ms>", which takes missingSeq
// as its sequence part.
func ParseStreamID(s string, missingSeq uint64) (StreamID, error) {
	if len(s) > 127 {
		return StreamID{}, ErrStreamInvalidID
	}
	msPart, seqPart, hasSeq := strings.Cut(s, "-")
	ms, ok := parseUint64Strict(msPart)
	if !ok {
		return StreamID{}, ErrStreamInvalidID
	}
	if !hasSeq {
		return StreamID{Ms: ms, Seq: missingSeq}, nil
	}
	seq, ok := parseUint64Strict(seqPart)
	if !ok {
		return StreamID{}, ErrStreamInvalidID
	}
	return StreamID{Ms: ms, Seq: seq}, nil
}

// ParseStreamRangeID parses a range bound the way XRANGE does: "-" and "+"
// are the extremes, everything else is an ID whose missing sequence part is
// missingSeq.
func ParseStreamRangeID(s string, missingSeq uint64) (StreamID, error) {
	switch s {
	case "-":
		return StreamID{}, nil
	case "+":
		return MaxStreamID, nil
	}
	return ParseStreamID(s, missingSeq)
}

// ParseStreamInterval parses an XRANGE/XPENDING bound, including the "("
// exclusive prefix. The returned ID is already adjusted to be inclusive.
func ParseStreamInterval(s string, isEnd bool) (StreamID, error) {
	var missing uint64
	if isEnd {
		missing = math.MaxUint64
	}
	if len(s) > 1 && s[0] == '(' {
		id, err := ParseStreamID(s[1:], missing)
		if err != nil {
			return StreamID{}, err
		}
		var ok bool
		if isEnd {
			id, ok = id.Prev()
			if !ok {
				return StreamID{}, errors.New("ERR invalid end ID for the interval")
			}
		} else {
			id, ok = id.Next()
			if !ok {
				return StreamID{}, errors.New("ERR invalid start ID for the interval")
			}
		}
		return id, nil
	}
	return ParseStreamRangeID(s, missing)
}

// streamIDOf parses a stored ID leniently: an empty or unparseable ID is 0-0.
func streamIDOf(s string) StreamID {
	if s == "" {
		return StreamID{}
	}
	id, err := ParseStreamID(s, 0)
	if err != nil {
		return StreamID{}
	}
	return id
}
