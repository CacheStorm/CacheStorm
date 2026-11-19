package vectorset

import (
	"math"
	"strings"
	"sync"
	"testing"
)

type redisCase struct {
	expr  string
	match []int
	near  string
	err   bool
}

// The expectations below were recorded from Redis 8.2.11: every element of
// redisAttrs was added to a vector set with VADD ... SETATTR, and each
// expression was run with VSIM ... TRUTH FILTER. match lists the indexes of
// the elements Redis returned; near is the text after "near: " in Redis's
// syntax error reply.
var redisAttrs = []string{
	"{\"year\":1984,\"name\":\"The Matrix\",\"rating\":8.7,\"genre\":\"action\",\"tags\":[\"a\",\"b\",1],\"flag\":true,\"none\":null,\"s\":\"abc\"}",
	"{\"year\":2010,\"rating\":7.5,\"genre\":\"drama\",\"flag\":false,\"num\":\"12\",\"s\":\"\"}",
	"{\"year\":\"1999\",\"rating\":\"9\",\"n\":-3.5,\"arr\":[1,[2,3]],\"obj\":{\"a\":1}}",
	"{\"a-b\":5,\"x_1\":2,\"9\":9,\"esc\":\"a\\\"b\\\\c\\nd\",\"u\":\"\\u0041\",\"sl\":\"\\/\"}",
	"not json",
	"{\"year\":1984 garbage",
	"{\"bad\": tru, \"year\": 1984}",
	"{\"year\": 1984, \"year\": 2000}",
	"{\"big\":1e400,\"neg\":-0,\"plus\":+5,\"dot\":1.,\"hex\":\"0x1p3\",\"sp\":\" 5\",\"inf\":\"inf\",\"nan\":\"nan\",\"bad\":1e}",
	"[{\"year\":1984}]",
	"{ \"year\" : 1990 , \"s\" : \"barfoobar\" }",
	"{\"long\":\"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001\",\"n\":\"7\",\"num255\":\"111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111\"}",
	"{\"nested\":[[[]]],\"e\":[],\"mixed\":[1,\"1\",null,true],\"objarr\":[1,{\"a\":1}]}",
	"{\"o\":{\"year\":1984,\"x\":[1,\"}\"]},\"year\":5}",
	"{\"year\":1e3,\"t\":\"1e3\",\"f\":false,\"z\":0}",
	"{\"year\":\"abc\",\"s\":\"ABC\"}",
	"{\"year\":true}",
	"{\"s\":\"12\x00ab\",\"year\":null}",
	"{\"year\":1984, \"tags\":[\"drama\",\"action\"], \"name\":\"Alien\"}",
	"{\"\\u0079ear\":1, \"k\\\"\":2}",
}

var redisCases = []redisCase{
	{expr: "1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0", match: []int{}},
	{expr: "-1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0.0", match: []int{}},
	{expr: "\"x\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"\"", match: []int{}},
	{expr: "''", match: []int{}},
	{expr: "null", match: []int{}},
	{expr: "[]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "[0]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"0\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1e3", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1.", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "2E1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: ".year", match: []int{0, 1, 2, 5, 7, 10, 13, 14, 15, 16, 18}},
	{expr: ".flag", match: []int{0}},
	{expr: ".none", match: []int{}},
	{expr: ".s", match: []int{0, 10, 15, 17}},
	{expr: ".tags", match: []int{0, 18}},
	{expr: ".obj", match: []int{}},
	{expr: ".arr", match: []int{2}},
	{expr: ".missing", match: []int{}},
	{expr: ".a-b", match: []int{3}},
	{expr: ".x_1", match: []int{3}},
	{expr: ".9", match: []int{3}},
	{expr: ".year == 1984", match: []int{0, 5, 7, 18}},
	{expr: ".year == \"1984\"", match: []int{0, 5, 7, 18}},
	{expr: ".year == 1999", match: []int{2}},
	{expr: ".year > 1990", match: []int{1, 2}},
	{expr: ".year >= 1984", match: []int{0, 1, 2, 5, 7, 10, 18}},
	{expr: ".year < 2000", match: []int{0, 2, 5, 7, 10, 13, 14, 15, 16, 17, 18}},
	{expr: ".year <= 1984", match: []int{0, 5, 7, 13, 14, 15, 16, 17, 18}},
	{expr: ".year != 1984", match: []int{1, 2, 10, 13, 14, 15, 16, 17}},
	{expr: ".name == \"The Matrix\"", match: []int{0}},
	{expr: ".rating > 8", match: []int{0, 2}},
	{expr: ".rating / 2 > 4", match: []int{0, 2}},
	{expr: ".genre == \"action\" and .rating > 8.0", match: []int{0}},
	{expr: ".genre == \"action\" && .rating > 8.0", match: []int{0}},
	{expr: ".genre == \"drama\" or .year > 2005", match: []int{1}},
	{expr: ".genre == \"drama\" || .year == 5", match: []int{1}},
	{expr: "!.flag", match: []int{1}},
	{expr: "not .flag", match: []int{1}},
	{expr: ".missing or 1", match: []int{}},
	{expr: ".none == null", match: []int{0}},
	{expr: ".none != null", match: []int{}},
	{expr: ".year == null", match: []int{17}},
	{expr: "null == null", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "null == 0", match: []int{}},
	{expr: "0 == null", match: []int{}},
	{expr: ".n < 0", match: []int{2}},
	{expr: ".n == -3.5", match: []int{2}},
	{expr: ".n * 2 == -7", match: []int{2}},
	{expr: ".big > 1e308", match: []int{8}},
	{expr: ".neg == 0", match: []int{8}},
	{expr: ".plus == 5", match: []int{8}},
	{expr: ".dot == 1", match: []int{8}},
	{expr: ".hex == 8", match: []int{8}},
	{expr: ".sp == 5", match: []int{8}},
	{expr: ".inf > 1e308", match: []int{8}},
	{expr: ".nan == .nan", match: []int{8}},
	{expr: ".nan != .nan", match: []int{}},
	{expr: ".bad", match: []int{}},
	{expr: ".long == 1", match: []int{}},
	{expr: ".long == 0", match: []int{11}},
	{expr: ".n == 7", match: []int{11}},
	{expr: ".num255 > 1", match: []int{11}},
	{expr: ".num255 == 0", match: []int{}},
	{expr: ".nested", match: []int{12}},
	{expr: ".e", match: []int{12}},
	{expr: ".mixed", match: []int{12}},
	{expr: ".objarr", match: []int{}},
	{expr: "1 in .mixed", match: []int{12}},
	{expr: "null in .mixed", match: []int{12}},
	{expr: "\"1\" in .mixed", match: []int{12}},
	{expr: ".o", match: []int{}},
	{expr: ".year == 5", match: []int{13}},
	{expr: ".t == 1000", match: []int{14}},
	{expr: ".t == .year", match: []int{14}},
	{expr: ".f == 0", match: []int{14}},
	{expr: ".z", match: []int{}},
	{expr: ".s == \"ABC\"", match: []int{15}},
	{expr: ".year == 0", match: []int{15}},
	{expr: ".year + 1 == 1", match: []int{15, 17}},
	{expr: ".s == 12", match: []int{17}},
	{expr: ".s == \"12\"", match: []int{}},
	{expr: ".u == \"u0041\"", match: []int{3}},
	{expr: ".sl == \"/\"", match: []int{3}},
	{expr: ".esc == \"a\\\"b\\\\c\nd\"", match: []int{}},
	{expr: "yes", near: "yes", err: true},
	{expr: ".k", match: []int{}},
	{expr: "1 + 2 == 3", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "2 * 3 + 1 == 7", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 + 2 * 3 == 7", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(1 + 2) * 3 == 9", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "10 / 4 == 2.5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "7 % 3 == 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "-7 % 3 == -1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "7.5 % 2 == 1.5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 % 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 / 0 > 1e308", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0 / 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0 / 0 == 0 / 0", match: []int{}},
	{expr: "2 ** 10 == 1024", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "2 ** 3 ** 2 == 512", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(2 ** 3) ** 2 == 64", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "-2 ** 2 == 4", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "2 ** -1 == 0.5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(-8) ** (1/3)", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(-8) ** (1/3) == (-8) ** (1/3)", match: []int{}},
	{expr: "2 ** 0.5 * 2 ** 0.5 == 2", match: []int{}},
	{expr: "0.7281259730525967 ** 1.4354652571585325 == 0.6341649735521443", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0.7281259730525967 ** 1.4354652571585325 == 0.6341649735521442", match: []int{}},
	{expr: ".rating ** 2.2 == 116.66547558049412", match: []int{0}},
	{expr: ".rating ** 1.37 == 19.37052430914285 or .rating ** 1.37 == 15.806435040216904", match: []int{0, 1}},
	{expr: "2 ** 0.5 == 1.4142135623730951", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "10 - 2 - 3 == 5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "100 / 10 / 5 == 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "3-1 == 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "3 -1 == 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "3 - -1 == 4", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "3--1 == 4", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(3)-1 == 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 - 1", match: []int{}},
	{expr: "\"3\" + \"4\" == 7", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"abc\" + 1 == 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"1e2\" * 1 == 100", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\" 5\" + 0 == 5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"5 \" + 0 == 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"0x10\" == 16", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"0x1p3\" == 8", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"0x\" == 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"inf\" > 1e308", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"-Infinity\" < -1e308", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"nan(abc)\" != 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"nan\" == \"nan\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"1.5\" == 1.5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"+3\" == 3", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\".5\" == 0.5", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"1e\" == 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"\t7\" == 7", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "null + 1 == 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "[1,2] + 1 == 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "[1] == 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "[1] == [2]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"a\" == \"a\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"a\" == \"b\"", match: []int{}},
	{expr: "\"a\" < \"b\"", match: []int{}},
	{expr: "\"b\" > \"a\"", match: []int{}},
	{expr: "\"10\" > \"9\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"abc\" == 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "'a' == \"a\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 == 1.0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0.1 + 0.2 == 0.3", match: []int{}},
	{expr: "0.1 + 0.2 > 0.3", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1e308 * 10 > 1e308", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 and 0", match: []int{}},
	{expr: "1 and 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0 or 0", match: []int{}},
	{expr: "0 or \"x\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"\" or null", match: []int{}},
	{expr: "!0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "!1", match: []int{}},
	{expr: "!\"\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "!\"a\"", match: []int{}},
	{expr: "!null", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "![]", match: []int{}},
	{expr: "not 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "!0 + 1 == 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "!(0 + 1)", match: []int{}},
	{expr: "1 + !0 == 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 < 2 == 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 < 2 < 3", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "3 > 2 > 1", match: []int{}},
	{expr: "1 == 1 and 2 == 2 or 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0 and 1 or 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 or 0 and 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 or 1 and 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(1 or 1) and 0", match: []int{}},
	{expr: "1 and not 0", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "5 in [1, 2, 3]", match: []int{}},
	{expr: "2 in [1, 2, 3]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"foo\" in [1, \"foo\", \"bar\"]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"1\" in [1, 2]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 in [\"1\"]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "0 in [\"abc\"]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"foo\" in \"barfoobar\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"zap\" in \"foobar\"", match: []int{}},
	{expr: "\"\" in \"x\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"\" in \"\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"x\" in \"\"", match: []int{}},
	{expr: "1 in \"123\"", match: []int{}},
	{expr: "\"1\" in 123", match: []int{}},
	{expr: ".genre in [\"action\", \"drama\"]", match: []int{0, 1}},
	{expr: ".director in [\"Spielberg\", \"Nolan\"]", match: []int{}},
	{expr: "\"a\" in .tags", match: []int{0}},
	{expr: "\"x\" in .tags", match: []int{}},
	{expr: "1 in .tags", match: []int{0}},
	{expr: "\"action\" in .tags", match: []int{18}},
	{expr: "\"foo\" in .s", match: []int{10}},
	{expr: ".s in \"xxabcxx\"", match: []int{0, 1}},
	{expr: "[1] in [1]", match: []int{}},
	{expr: "null in [1]", match: []int{}},
	{expr: "1 in []", match: []int{}},
	{expr: "1 in [1,]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"a\\\"b\" in [\"a\\\"b\"]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "'a' in ['a']", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 in [ 1 , 2 ]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "-1 in [-1]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1e2 in [100]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 in [1.0]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "2 in .arr", match: []int{}},
	{expr: "[2,3] in .arr", match: []int{2}},
	{expr: "0 in .arr", match: []int{2}},
	{expr: ".year in [1984, 2010]", match: []int{0, 1, 5, 7, 18}},
	{expr: ".year in [\"1999\"]", match: []int{2}},
	{expr: ".year in [1999]", match: []int{2}},
	{expr: "\"Matrix\" in .name", match: []int{0}},
	{expr: "1 in 1", match: []int{}},
	{expr: "1 in [1] == 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 + 1 in [2]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 in [1] and 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 2 +", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "+ 1 2", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 2 + 3 *", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 not", match: []int{}},
	{expr: "( 1 )", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "((1))", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 ( 2 + )", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "(1) (2) ==", match: []int{}},
	{expr: "\"a\\\"b\" == \"a\\\"b\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "'it\\'s' == \"it\\'s\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"a\\nb\" == \"a\\nb\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "\"\\\\\" == \"\\\\\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "", near: "", err: true},
	{expr: " ", near: " ", err: true},
	{expr: "\t", near: "\t", err: true},
	{expr: "(", near: "(", err: true},
	{expr: ")", near: ")", err: true},
	{expr: "(1", near: "(1", err: true},
	{expr: "1)", near: "1)", err: true},
	{expr: "()", near: "()", err: true},
	{expr: "(1))", near: "(1))", err: true},
	{expr: "((1)", near: "((1)", err: true},
	{expr: "1 2", near: "1 2", err: true},
	{expr: "1 +", near: "1 +", err: true},
	{expr: "+", near: "+", err: true},
	{expr: "!", near: "!", err: true},
	{expr: "!!1", near: "!!1", err: true},
	{expr: "! !1", near: "! !1", err: true},
	{expr: "not not 1", near: "not not 1", err: true},
	{expr: "!(!1)", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1 ==", near: "1 ==", err: true},
	{expr: "== 1", near: "== 1", err: true},
	{expr: "1 = 1", near: " 1", err: true},
	{expr: "1 & 1", near: " 1", err: true},
	{expr: "1 | 1", near: " 1", err: true},
	{expr: "foo", near: "foo", err: true},
	{expr: "AND", near: "AND", err: true},
	{expr: "1 AND 1", near: " 1", err: true},
	{expr: "nul", near: "nul", err: true},
	{expr: "nullx", near: "nullx", err: true},
	{expr: "null)", near: "null)", err: true},
	{expr: "(.year == null)", near: "(.year == null)", err: true},
	{expr: "(null)", near: "(null)", err: true},
	{expr: "null and 1", match: []int{}},
	{expr: "1 and null", match: []int{}},
	{expr: "null || 1", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "1e-5", near: "-5", err: true},
	{expr: "1e+5", near: "+5", err: true},
	{expr: "1.2.3", near: "1.2.3", err: true},
	{expr: "1e", near: "1e", err: true},
	{expr: ".5", match: []int{}},
	{expr: "0x10", near: "10", err: true},
	{expr: "\"unterminated", near: "\"unterminated", err: true},
	{expr: "'unterm", near: "'unterm", err: true},
	{expr: "\"a\\\"", near: "\"a\\\"", err: true},
	{expr: "[1, 2", near: "[1, 2", err: true},
	{expr: "[1 2]", near: "2]", err: true},
	{expr: "[x]", near: "x]", err: true},
	{expr: "[1,]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "[,]", near: ",]", err: true},
	{expr: "[1,,2]", near: ",2]", err: true},
	{expr: "[-]", near: "]", err: true},
	{expr: "[1e-5]", near: "-5]", err: true},
	{expr: "[[1]]", near: "[1]]", err: true},
	{expr: "[null]", near: "null]", err: true},
	{expr: "[.year]", near: ".year]", err: true},
	{expr: ".", match: []int{}},
	{expr: "!.", match: []int{}},
	{expr: ". == 1", near: ". == 1", err: true},
	{expr: ".year.month", near: ".year.month", err: true},
	{expr: ".year . month", near: ". month", err: true},
	{expr: ".year==1984and 1", match: []int{0, 5, 7, 18}},
	{expr: "1 andy 1", near: " 1", err: true},
	{expr: "1 inx [1]", near: " [1]", err: true},
	{expr: "1 in[1]", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
	{expr: "x in [1]", near: " in [1]", err: true},
	{expr: "@", near: "@", err: true},
	{expr: "#", near: "#", err: true},
	{expr: "{", near: "{", err: true},
	{expr: "]", near: "]", err: true},
	{expr: ",", near: ",", err: true},
	{expr: "1,2", near: ",2", err: true},
	{expr: ".year >= 1980 and .year < 1990", match: []int{0, 5, 7, 18}},
	{expr: "1 ** ", near: "1 ** ", err: true},
	{expr: "** 2", near: "** 2", err: true},
	{expr: "2 *** 2", near: "2 *** 2", err: true},
	{expr: "2 ** * 2", near: "2 ** * 2", err: true},
	{expr: "1 + (2", near: "1 + (2", err: true},
	{expr: "1 + 2)", near: "1 + 2)", err: true},
	{expr: "((1 + 2) * 3", near: "((1 + 2) * 3", err: true},
	{expr: "a", near: "a", err: true},
	{expr: "_x", near: "_x", err: true},
	{expr: ".year ==", near: ".year ==", err: true},
	{expr: "== .year", near: "== .year", err: true},
	{expr: "-", near: "-", err: true},
	{expr: "- 1", near: "- 1", err: true},
	{expr: "-1 -", near: "-1 -", err: true},
	{expr: "--1", near: "--1", err: true},
	{expr: "- -1", near: "- -1", err: true},
	{expr: "1 - - 1", near: "1 - - 1", err: true},
	{expr: "!=1", near: "!=1", err: true},
	{expr: "1 !", match: []int{}},
	{expr: "1 !1", near: "1 !1", err: true},
	{expr: "in", near: "in", err: true},
	{expr: "or", near: "or", err: true},
	{expr: "1 or", near: "1 or", err: true},
	{expr: "or 1", near: "or 1", err: true},
	{expr: "1 not 1", near: "1 not 1", err: true},
	{expr: "\"abc\" \"def\"", near: "\"abc\" \"def\"", err: true},
	{expr: "[1] [2]", near: "[1] [2]", err: true},
	{expr: "111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111", near: "111111111111111111111111111111111111111111111", err: true},
	{expr: "1.00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", near: "0000000", err: true},
	{expr: "\"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\"", match: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
}

func nearText(src string, errpos int) string {
	if errpos >= len(src) {
		errpos = 0
	}
	return src[errpos:]
}

func TestRedisCases(t *testing.T) {
	for _, c := range redisCases {
		e, errpos := Compile(c.expr)
		if c.err {
			if e != nil {
				t.Errorf("%q: compiled, Redis reports a syntax error near %q", c.expr, c.near)
			} else if got := nearText(c.expr, errpos); got != c.near {
				t.Errorf("%q: error near %q, Redis says near %q", c.expr, got, c.near)
			}
			continue
		}
		if e == nil {
			t.Errorf("%q: syntax error near %q, Redis compiles it", c.expr, nearText(c.expr, errpos))
			continue
		}
		want := make(map[int]bool, len(c.match))
		for _, i := range c.match {
			want[i] = true
		}
		for i, a := range redisAttrs {
			if got := e.Match([]byte(a)); got != want[i] {
				t.Errorf("%q on %q: got %v, Redis %v", c.expr, a, got, want[i])
			}
		}
	}
}

func TestNULEndsExpression(t *testing.T) {
	e, _ := Compile("1\x00 +")
	if e == nil || !e.Match([]byte("{}")) {
		t.Fatal("expression should end at the NUL byte")
	}
}

// Recorded from Redis 8.2.11: arrays nest at most 1000 deep, counting the
// array that is the field's value.
func TestJSONNestingLimit(t *testing.T) {
	nest := func(n int, inner string) []byte {
		return []byte(`{"d":` + strings.Repeat("[", n) + inner + strings.Repeat("]", n) + `}`)
	}
	e, _ := Compile(".d")
	cases := []struct {
		n     int
		inner string
		want  bool
	}{
		{998, "", true}, {999, "", true}, {1000, "", true}, {1001, "", false},
		{998, "1", true}, {999, "1", true}, {1000, "1", false}, {1001, "1", false},
	}
	for _, c := range cases {
		if got := e.Match(nest(c.n, c.inner)); got != c.want {
			t.Errorf("depth %d inner %q: got %v want %v", c.n, c.inner, got, c.want)
		}
	}
}

// Verified against glibc's pow (which Redis uses) bit for bit; math.Pow
// returns a different last bit for these inputs.
func TestPowMatchesGlibc(t *testing.T) {
	cases := []struct{ x, y, want float64 }{
		{0.9159448117309811, -26, 9.803880713353726},
		{0.8364614512743888, 25.394426331407537, 0.010728991726386613},
		{7.412518562014903, 23.78998062575576, 4.9712932480604937e+20},
		{0.7281259730525967, 1.4354652571585325, 0.6341649735521443},
		{0.9873459197017372, -55608.2694561641, 3.5535958789412095e+307},
		{2, 0.5, 1.4142135623730951},
		{2, 10, 1024},
		{-8, 1.0 / 3, math.NaN()},
		{-2, 3, -8},
		{0, -1, math.Inf(1)},
		{math.Copysign(0, -1), -3, math.Inf(-1)},
		{1, math.NaN(), 1},
		{math.NaN(), 0, 1},
		{10, 400, math.Inf(1)},
		{10, -400, 0},
	}
	for _, c := range cases {
		got := cpow(c.x, c.y)
		if got != c.want && !(got != got && c.want != c.want) {
			t.Errorf("pow(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestConcurrentMatch(t *testing.T) {
	e, _ := Compile(`.year > 1980 and "a" in .tags or .genre == "drama"`)
	want := make([]bool, len(redisAttrs))
	for j, a := range redisAttrs {
		want[j] = e.Match([]byte(a))
	}
	if !want[0] || want[1] {
		t.Fatal("a missing selector anywhere must make the expression false")
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				for j, a := range redisAttrs {
					if e.Match([]byte(a)) != want[j] {
						t.Errorf("attrs %d: wrong result", j)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}

// Every prefix of these inputs must compile or fail cleanly and run without
// panicking, whatever JSON prefix it is matched against.
func TestPrefixesDoNotPanic(t *testing.T) {
	exprs := []string{
		`(.year - 2000) ** 2 < 100 and .rating / 2 > 4 || !(.genre in ["action", 'drama', -1.5e3, 7,]) && "x\"y" in .s`,
		`not .a-b % 3 != -2 ** -1 ** 0.5 or .x_1 >= .9 and null == .n || [] in "s" && 1e3 <= ((.t))`,
		`.a+.b-.c*.d/.e%.f**.g>.h>=.i<.j<=.k==.l!=.m in [1,"2",'3'] and !.n or not .o && .p || .q`,
	}
	docs := []string{
		`{"year":1984,"rating":9.5,"genre":"x","s":"x\"y","a-b":7,"x_1":[1,[2,"]"]],"9":-0.5e2,"n":null,"t":"1e3"}`,
		`{"a":1,"b":"2","c":true,"d":false,"e":[],"f":{"g":[1,{"h":"}"}]},"g":2,"h":"A\\\n","i":-1,"j":+1}`,
	}
	for _, src := range exprs {
		for i := 0; i <= len(src); i++ {
			e, errpos := Compile(src[:i])
			if e == nil {
				if errpos < 0 || errpos > i {
					t.Fatalf("%q: errpos %d out of range", src[:i], errpos)
				}
				continue
			}
			for _, d := range docs {
				for j := 0; j <= len(d); j++ {
					e.Match([]byte(d[:j]))
				}
			}
		}
	}
}
