package command

import (
	"fmt"
	"regexp"
	"testing"
)

var profileTimes = regexp.MustCompile(`(\+[A-Za-z ]*[Tt]ime\r\n)\$\d+\r\n[^\r]*\r\n`)

// The replies are Redis 8.2's, with times replaced by T.
func TestFTProfile(t *testing.T) {
	h := newCursorHarness(t)
	for _, c := range []struct {
		want string
		args []string
	}{
		{"*2\r\n*3\r\n:2\r\n$3\r\nd:2\r\n$3\r\nd:4\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*8\r\n+Type\r\n+INTERSECT\r\n+Time\r\nT\r\n+Number of reading operations\r\n:2\r\n+Child iterators\r\n*2\r\n*10\r\n+Type\r\n+TEXT\r\n+Term\r\n+hello\r\n+Time\r\nT\r\n+Number of reading operations\r\n:3\r\n+Estimated number of matches\r\n:3\r\n*8\r\n+Type\r\n+NOT\r\n+Time\r\nT\r\n+Number of reading operations\r\n:3\r\n+Child iterator\r\n*10\r\n+Type\r\n+TEXT\r\n+Term\r\n+world\r\n+Time\r\nT\r\n+Number of reading operations\r\n:2\r\n+Estimated number of matches\r\n:2\r\n+Result processors profile\r\n*3\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n*6\r\n+Type\r\n+Scorer\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n*6\r\n+Type\r\n+Sorter\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "SEARCH", "QUERY", "hello -world", "NOCONTENT"}},
		{"*2\r\n*3\r\n:3\r\n$3\r\nd:2\r\n*6\r\n$1\r\nt\r\n$5\r\nhello\r\n$1\r\nn\r\n$1\r\n2\r\n$1\r\ng\r\n$1\r\nb\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*8\r\n+Type\r\n+INTERSECT\r\n+Time\r\nT\r\n+Number of reading operations\r\n:3\r\n+Child iterators\r\n*2\r\n*10\r\n+Type\r\n+UNION\r\n+Query type\r\n+UNION\r\n+Time\r\nT\r\n+Number of reading operations\r\n:4\r\n+Child iterators\r\n*2\r\n*10\r\n+Type\r\n+TEXT\r\n+Term\r\n+hello\r\n+Time\r\nT\r\n+Number of reading operations\r\n:3\r\n+Estimated number of matches\r\n:3\r\n*10\r\n+Type\r\n+TEXT\r\n+Term\r\n+world\r\n+Time\r\nT\r\n+Number of reading operations\r\n:2\r\n+Estimated number of matches\r\n:2\r\n*10\r\n+Type\r\n+NUMERIC\r\n+Term\r\n+1 - 5\r\n+Time\r\nT\r\n+Number of reading operations\r\n:3\r\n+Estimated number of matches\r\n:5\r\n+Result processors profile\r\n*5\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:3\r\n*6\r\n+Type\r\n+Scorer\r\n+Time\r\nT\r\n+Results processed\r\n:3\r\n*6\r\n+Type\r\n+Sorter\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n*6\r\n+Type\r\n+Pager/Limiter\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n*6\r\n+Type\r\n+Loader\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "SEARCH", "LIMITED", "QUERY", "hello|world @n:[2 4]", "LIMIT", "1", "1"}},
		{"*2\r\n*3\r\n:2\r\n*4\r\n$1\r\nt\r\n$5\r\nhello\r\n$1\r\nn\r\n$1\r\n2\r\n*4\r\n$1\r\nt\r\n$5\r\nworld\r\n$1\r\nn\r\n$1\r\n3\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*10\r\n+Type\r\n+UNION\r\n+Query type\r\n+TAG\r\n+Time\r\nT\r\n+Number of reading operations\r\n:3\r\n+Child iterators\r\n*2\r\n*10\r\n+Type\r\n+TAG\r\n+Term\r\n+a\r\n+Time\r\nT\r\n+Number of reading operations\r\n:2\r\n+Estimated number of matches\r\n:2\r\n*10\r\n+Type\r\n+TAG\r\n+Term\r\n+b\r\n+Time\r\nT\r\n+Number of reading operations\r\n:2\r\n+Estimated number of matches\r\n:2\r\n+Result processors profile\r\n*4\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n*6\r\n+Type\r\n+Loader\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n*6\r\n+Type\r\n+Filter - Predicate >\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n*6\r\n+Type\r\n+Pager/Limiter\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "@g:{a|b}", "LOAD", "1", "@t", "FILTER", "@n>1", "LIMIT", "0", "2"}},
		{"*2\r\n*2\r\n:2\r\n*2\r\n$1\r\nt\r\n$5\r\nhello\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*6\r\n+Type\r\n+WILDCARD\r\n+Time\r\nT\r\n+Number of reading operations\r\n:2\r\n+Result processors profile\r\n*4\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n*6\r\n+Type\r\n+Loader\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n*6\r\n+Type\r\n+Filter - Predicate ==\r\n+Time\r\nT\r\n+Results processed\r\n:0\r\n*6\r\n+Type\r\n+Pager/Limiter\r\n+Time\r\nT\r\n+Results processed\r\n:1\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "*", "LOAD", "1", "@t", "FILTER", "@t=='hello'", "LIMIT", "0", "1"}},
		{"*2\r\n*3\r\n:5\r\n*4\r\n$7\r\n__score\r\n$13\r\n1.07317073313\r\n$1\r\nt\r\n$3\r\nfoo\r\n*4\r\n$7\r\n__score\r\n$13\r\n1.07317073313\r\n$1\r\nt\r\n$5\r\nhello\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*6\r\n+Type\r\n+WILDCARD\r\n+Time\r\nT\r\n+Number of reading operations\r\n:5\r\n+Result processors profile\r\n*4\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Scorer\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Loader\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Sorter\r\n+Time\r\nT\r\n+Results processed\r\n:2\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "*", "ADDSCORES", "SORTBY", "1", "@t", "LIMIT", "0", "2"}},
		{"*2\r\n*4\r\n:3\r\n*4\r\n$1\r\nn\r\n$1\r\n3\r\n$1\r\nx\r\n$1\r\n6\r\n*4\r\n$1\r\nn\r\n$1\r\n4\r\n$1\r\nx\r\n$1\r\n8\r\n*4\r\n$1\r\nn\r\n$1\r\n5\r\n$1\r\nx\r\n$2\r\n10\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*6\r\n+Type\r\n+WILDCARD\r\n+Time\r\nT\r\n+Number of reading operations\r\n:5\r\n+Result processors profile\r\n*3\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Projector - Operator *\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Filter - Predicate >\r\n+Time\r\nT\r\n+Results processed\r\n:3\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "*", "APPLY", "@n*2", "AS", "x", "FILTER", "@x>4"}},
		{"-Property `nosuch` not loaded nor in pipeline\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "hello", "FILTER", "@nosuch>1"}},
		{"*2\r\n*1\r\n:0\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*6\r\n+Type\r\n+EMPTY\r\n+Time\r\nT\r\n+Number of reading operations\r\n:0\r\n+Result processors profile\r\n*1\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:0\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "@n:[10 20]"}},
		{"*2\r\n*4\r\n:3\r\n*4\r\n$1\r\ng\r\n$1\r\na\r\n$1\r\nc\r\n$1\r\n2\r\n*4\r\n$1\r\ng\r\n$1\r\nb\r\n$1\r\nc\r\n$1\r\n2\r\n*4\r\n$1\r\ng\r\n$1\r\nc\r\n$1\r\nc\r\n$1\r\n1\r\n*4\r\n+Shards\r\n*1\r\n*16\r\n+Total profile time\r\nT\r\n+Parsing time\r\nT\r\n+Workers queue time\r\nT\r\n+Pipeline creation time\r\nT\r\n+Total GIL time\r\nT\r\n+Warning\r\n*1\r\n+None\r\n+Iterators profile\r\n*6\r\n+Type\r\n+WILDCARD\r\n+Time\r\nT\r\n+Number of reading operations\r\n:5\r\n+Result processors profile\r\n*4\r\n*6\r\n+Type\r\n+Index\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Loader\r\n+Time\r\nT\r\n+Results processed\r\n:5\r\n*6\r\n+Type\r\n+Grouper\r\n+Time\r\nT\r\n+Results processed\r\n:3\r\n*6\r\n+Type\r\n+Sorter\r\n+Time\r\nT\r\n+Results processed\r\n:3\r\n+Coordinator\r\n*0\r\n", []string{"FT.PROFILE", "i", "AGGREGATE", "QUERY", "*", "GROUPBY", "1", "@g", "REDUCE", "COUNT", "0", "AS", "c", "SORTBY", "2", "@g", "ASC"}},
		{"-FT.PROFILE does not support cursor\r\n", []string{"FT.PROFILE", "i", "SEARCH", "QUERY", "hello", "WITHCURSOR"}},
		{"-No `SEARCH` or `AGGREGATE` provided\r\n", []string{"FT.PROFILE", "i", "FOO", "QUERY", "hello"}},
		{"-The QUERY keyword is expected\r\n", []string{"FT.PROFILE", "i", "SEARCH", "NOQUERY", "hello"}},
		{"-nosuch: no such index\r\n", []string{"FT.PROFILE", "nosuch", "SEARCH", "QUERY", "hello"}},
	} {
		if got := profileTimes.ReplaceAllString(h.do(c.args...), "${1}T\r\n"); got != c.want {
			t.Errorf("%v:\n got %q\nwant %q", c.args, got, c.want)
		}
	}
}

func TestSimulateProcessors(t *testing.T) {
	steps := []profileStep{{name: "Loader"}, {name: "Filter - Predicate >"}, {name: "Pager/Limiter", offset: 1, limit: 1}}
	tr := &aggregateTrace{matched: 5, filters: []filterTrace{{passed: []int{1, 3, 4}, failAt: -1}}}
	counts, rows := simulateProcessors(steps, tr)
	if want := []int{3, 3, 1, 1}; fmt.Sprint(counts) != fmt.Sprint(want) || rows != 4 {
		t.Errorf("paged: got %v, %d", counts, rows)
	}
	tr = &aggregateTrace{matched: 5, filters: []filterTrace{{passed: []int{0}, failAt: 2}}}
	counts, rows = simulateProcessors(steps[:2], tr)
	if want := []int{2, 2, 1}; fmt.Sprint(counts) != fmt.Sprint(want) || rows != 3 {
		t.Errorf("failed: got %v, %d", counts, rows)
	}
}
