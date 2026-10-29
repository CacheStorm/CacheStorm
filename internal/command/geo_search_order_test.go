package command

import (
	"strings"
	"testing"
)

// TestGeoSearchOrderMatchesRedis pins replies Redis 7.4 gives: without a sort
// order members come in the order the geohash boxes are scanned, ANY stops at
// the first members found, and STORE with a WITH* option names the command.
func TestGeoSearchOrderMatchesRedis(t *testing.T) {
	h := newLuaHarness(t)
	RegisterGeoCommands(h.router)
	RegisterSortedSetCommands(h.router)
	h.do("GEOADD", "Sicily", "13.361389", "38.115556", "Palermo", "15.087269", "37.502669", "Catania", "12.5", "37.8", "Marsala")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"GEORADIUS", "Sicily", "15", "37", "200", "km"}, "*2\r\n$7\r\nPalermo\r\n$7\r\nCatania\r\n"},
		{[]string{"GEORADIUS", "Sicily", "15", "37", "200", "km", "COUNT", "1", "ANY"}, "*1\r\n$7\r\nPalermo\r\n"},
		{[]string{"GEORADIUS", "Sicily", "15", "37", "200", "km", "COUNT", "1"}, "*1\r\n$7\r\nCatania\r\n"},
		{[]string{"GEORADIUSBYMEMBER_RO", "Sicily", "Palermo", "100", "mi"}, "*2\r\n$7\r\nMarsala\r\n$7\r\nPalermo\r\n"},
		{[]string{"GEORADIUS", "Sicily", "15", "37", "200", "km", "STORE", "d", "WITHDIST"}, "-ERR STORE option in GEORADIUS is not compatible with WITHDIST, WITHHASH and WITHCOORD options\r\n"},
		{[]string{"GEOSEARCHSTORE", "d", "Sicily", "FROMMEMBER", "Palermo", "BYBOX", "400", "400", "km", "WITHDIST"}, "-ERR GEOSEARCHSTORE is not compatible with WITHDIST, WITHHASH and WITHCOORD options\r\n"},
		{[]string{"GEOSEARCH", "Sicily", "FROMLONLAT", "15", "37", "BYBOX", "x", "1", "km"}, "-ERR need numeric width\r\n"},
		{[]string{"GEOSEARCH", "Sicily", "FROMLONLAT", "15", "37", "BYBOX", "1", "x", "km"}, "-ERR need numeric height\r\n"},
		{[]string{"GEORADIUSBYMEMBER", "Sicily", "nosuch", "x", "km"}, "-ERR could not decode requested zset member\r\n"},
		{[]string{"GEORADIUSBYMEMBER", "nokey", "nosuch", "x", "km"}, "*0\r\n"},
	} {
		if got := h.do(c.args...); got != c.want {
			t.Errorf("%s\n got: %q\nwant: %q", strings.Join(c.args, " "), got, c.want)
		}
	}
}

// TestGeoPartialSortBreaksTiesLikeRedis checks pqsort against members at the
// same distance: Redis returns the ones its partial quicksort leaves first.
func TestGeoPartialSortBreaksTiesLikeRedis(t *testing.T) {
	hits := make([]geoHit, 12)
	for i := range hits {
		hits[i] = geoHit{member: string(rune('a' + i)), dist: float64([]int{3, 1, 1, 2, 1, 3, 2, 1, 2, 3, 1, 2}[i])}
	}
	cmp := func(x, y *geoHit) int {
		switch {
		case x.dist > y.dist:
			return 1
		case x.dist < y.dist:
			return -1
		}
		return 0
	}
	geoPartialSort(hits, cmp, 0, 2)
	for i := 1; i < 3; i++ {
		if hits[i].dist < hits[i-1].dist {
			t.Fatalf("not sorted: %+v", hits[:3])
		}
	}
	if hits[2].dist != 1 {
		t.Fatalf("partial sort missed the smallest values: %+v", hits)
	}
}
