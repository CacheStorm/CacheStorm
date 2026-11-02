#!/usr/bin/env python3
"""Generates the LALR(1) tables of RediSearch's query grammars
(src/query_parser/v1/parser.y and v2/parser.y), with conflicts, default
reductions and fallback tokens resolved the way lemon resolves them, so
queries parse as Redis parses them.

    python3 scripts/gen_search_parser.py 1 > internal/search/dialect1_tables.go
    python3 scripts/gen_search_parser.py 2 > internal/search/dialect2_tables.go
"""
import sys

DIALECT = sys.argv[1] if len(sys.argv) > 1 else "1"

# Precedence, lowest first, as parser.y declares it.
PREC1 = [
    ("left", ["LOWEST"]), ("left", ["TILDE"]), ("left", ["TAGLIST"]), ("left", ["QUOTE"]),
    ("left", ["COLON"]), ("left", ["MINUS"]), ("left", ["NUMBER"]), ("left", ["STOPWORD"]),
    ("left", ["TERMLIST"]), ("left", ["TERM"]), ("left", ["PREFIX", "SUFFIX", "CONTAINS"]),
    ("left", ["PERCENT"]), ("left", ["ATTRIBUTE"]), ("right", ["LP"]), ("left", ["RP"]),
    ("left", ["MODIFIER"]), ("left", ["AND"]), ("left", ["OR"]), ("left", ["ORX"]), ("left", ["ARROW"]),
]

# Rules in parser.y order: (name, lhs, rhs, explicit precedence).
RULES1 = [
    ("query_expr", "query", ["expr"], None),
    ("query_empty", "query", [], None),
    ("query_star", "query", ["STAR"], None),
    ("and", "expr", ["expr", "expr"], "AND"),
    ("expr_union", "expr", ["union"], "ORX"),
    ("union_new", "union", ["expr", "OR", "expr"], "OR"),
    ("union_add", "union", ["union", "OR", "expr"], "ORX"),
    ("modifier_expr", "expr", ["modifier", "COLON", "expr"], "MODIFIER"),
    ("modifierlist_expr", "expr", ["modifierlist", "COLON", "expr"], "MODIFIER"),
    ("parens", "expr", ["LP", "expr", "RP"], None),
    ("attribute", "attribute", ["ATTRIBUTE", "COLON", "term"], None),
    ("attrs_one", "attribute_list", ["attribute"], None),
    ("attrs_add", "attribute_list", ["attribute_list", "SEMICOLON", "attribute"], None),
    ("attrs_semi", "attribute_list", ["attribute_list", "SEMICOLON"], None),
    ("attrs_empty", "attribute_list", [], None),
    ("attributes", "expr", ["expr", "ARROW", "LB", "attribute_list", "RB"], None),
    ("quote_termlist", "expr", ["QUOTE", "termlist", "QUOTE"], "TERMLIST"),
    ("quote_term", "expr", ["QUOTE", "term", "QUOTE"], "TERMLIST"),
    ("expr_term", "expr", ["term"], "LOWEST"),
    ("expr_affix", "expr", ["affix"], "PREFIX"),
    ("expr_termlist", "expr", ["termlist"], "TERMLIST"),
    ("expr_stopword", "expr", ["STOPWORD"], "STOPWORD"),
    ("termlist_new", "termlist", ["term", "term"], "TERMLIST"),
    ("termlist_add", "termlist", ["termlist", "term"], "TERMLIST"),
    ("termlist_stopword", "termlist", ["termlist", "STOPWORD"], "TERMLIST"),
    ("not", "expr", ["MINUS", "expr"], None),
    ("optional", "expr", ["TILDE", "expr"], None),
    ("affix_prefix", "affix", ["PREFIX"], "PREFIX"),
    ("affix_suffix", "affix", ["SUFFIX"], "PREFIX"),
    ("affix_contains", "affix", ["CONTAINS"], "PREFIX"),
    ("fuzzy1", "expr", ["PERCENT", "term", "PERCENT"], "PREFIX"),
    ("fuzzy2", "expr", ["PERCENT", "PERCENT", "term", "PERCENT", "PERCENT"], "PREFIX"),
    ("fuzzy3", "expr", ["PERCENT", "PERCENT", "PERCENT", "term", "PERCENT", "PERCENT", "PERCENT"], "PREFIX"),
    ("fuzzy1_stop", "expr", ["PERCENT", "STOPWORD", "PERCENT"], "PREFIX"),
    ("fuzzy2_stop", "expr", ["PERCENT", "PERCENT", "STOPWORD", "PERCENT", "PERCENT"], "PREFIX"),
    ("fuzzy3_stop", "expr", ["PERCENT", "PERCENT", "PERCENT", "STOPWORD", "PERCENT", "PERCENT", "PERCENT"], "PREFIX"),
    ("modifier", "modifier", ["MODIFIER"], None),
    ("modifierlist_new", "modifierlist", ["modifier", "OR", "term"], None),
    ("modifierlist_add", "modifierlist", ["modifierlist", "OR", "term"], None),
    ("tag_expr", "expr", ["modifier", "COLON", "tag_list"], None),
    ("tags_term", "tag_list", ["LB", "term"], "TAGLIST"),
    ("tags_stopword", "tag_list", ["LB", "STOPWORD"], "TAGLIST"),
    ("tags_affix", "tag_list", ["LB", "affix"], "TAGLIST"),
    ("tags_termlist", "tag_list", ["LB", "termlist"], "TAGLIST"),
    ("tags_add_term", "tag_list", ["tag_list", "OR", "term"], "TAGLIST"),
    ("tags_add_stopword", "tag_list", ["tag_list", "OR", "STOPWORD"], "TAGLIST"),
    ("tags_add_affix", "tag_list", ["tag_list", "OR", "affix"], "TAGLIST"),
    ("tags_add_termlist", "tag_list", ["tag_list", "OR", "termlist"], "TAGLIST"),
    ("tags_close", "tag_list", ["tag_list", "RB"], "TAGLIST"),
    ("numeric_expr", "expr", ["modifier", "COLON", "numeric_range"], None),
    ("numeric_range", "numeric_range", ["LSQB", "num", "num", "RSQB"], "NUMBER"),
    ("geo_expr", "expr", ["modifier", "COLON", "geo_filter"], None),
    ("geo_filter", "geo_filter", ["LSQB", "num", "num", "num", "TERM", "RSQB"], "NUMBER"),
    ("num", "num", ["NUMBER"], None),
    ("num_exclusive", "num", ["LP", "num"], None),
    ("num_negative", "num", ["MINUS", "num"], None),
    ("term", "term", ["TERM"], None),
    ("term_number", "term", ["NUMBER"], None),
]


PREC2 = [
    ("left", ["LOWEST"]), ("left", ["TEXTEXPR"]), ("left", ["ORX"]), ("left", ["OR"]),
    ("left", ["ISMISSING"]), ("left", ["MODIFIER"]), ("left", ["RP", "RB", "RSQB"]), ("left", ["EXACT"]),
    ("left", ["TERM"]), ("left", ["QUOTE", "SQUOTE"]), ("left", ["LP", "LB", "LSQB"]), ("left", ["TILDE", "MINUS"]),
    ("left", ["AND"]), ("left", ["ARROW"]), ("left", ["COLON"]), ("left", ["NOT_EQUAL", "EQUALS"]),
    ("left", ["GE", "GT", "LE", "LT"]), ("left", ["NUMBER"]), ("left", ["SIZE"]), ("left", ["STAR"]),
    ("left", ["TAGLIST"]), ("left", ["TERMLIST"]), ("left", ["PREFIX", "SUFFIX", "CONTAINS"]), ("left", ["PERCENT"]),
    ("left", ["ATTRIBUTE"]), ("left", ["VERBATIM", "WILDCARD"]),
]
FALLBACK2 = {"EXACT": "TERM", "AS_T": "TERM", "ISMISSING": "TERM"}

RULES2 = [
    ("query_expr", "query", ["expr"], None),
    ("query_empty", "query", [], None),
    ("query_star", "query", ["star"], None),
    ("star", "star", ["STAR"], None),
    ("star_parens", "star", ["LP", "star", "RP"], None),
    ("expr_text", "expr", ["text_expr"], "TEXTEXPR"),
    ("and", "expr", ["expr", "expr"], "AND"),
    ("and_text_expr", "expr", ["text_expr", "expr"], "AND"),
    ("and_expr_text", "expr", ["expr", "text_expr"], "AND"),
    ("text_and", "text_expr", ["text_expr", "text_expr"], "AND"),
    ("expr_union", "expr", ["union"], "ORX"),
    ("union_new", "union", ["expr", "OR", "expr"], "OR"),
    ("union_add", "union", ["union", "OR", "expr"], "OR"),
    ("union_text_expr", "union", ["text_expr", "OR", "expr"], "OR"),
    ("union_expr_text", "union", ["expr", "OR", "text_expr"], "OR"),
    ("text_expr_union", "text_expr", ["text_union"], "ORX"),
    ("text_union_new", "text_union", ["text_expr", "OR", "text_expr"], "OR"),
    ("text_union_add", "text_union", ["text_union", "OR", "text_expr"], "OR"),
    ("modifier_expr", "expr", ["modifier", "COLON", "text_expr"], None),
    ("modifierlist_expr", "expr", ["modifierlist", "COLON", "text_expr"], None),
    ("parens", "expr", ["LP", "expr", "RP"], None),
    ("text_parens", "text_expr", ["LP", "text_expr", "RP"], None),
    ("attribute", "attribute", ["ATTRIBUTE", "COLON", "param_term"], None),
    ("attrs_one", "attribute_list", ["attribute"], None),
    ("attrs_add", "attribute_list", ["attribute_list", "SEMICOLON", "attribute"], None),
    ("attrs_semi", "attribute_list", ["attribute_list", "SEMICOLON"], None),
    ("attrs_empty", "attribute_list", [], None),
    ("attributes", "expr", ["expr", "ARROW", "LB", "attribute_list", "RB"], None),
    ("text_attributes", "text_expr", ["text_expr", "ARROW", "LB", "attribute_list", "RB"], None),
    ("exact", "text_expr", ["EXACT"], "TERMLIST"),
    ("quote_attr", "text_expr", ["QUOTE", "ATTRIBUTE", "QUOTE"], "TERMLIST"),
    ("squote_attr", "text_expr", ["SQUOTE", "ATTRIBUTE", "SQUOTE"], "TERMLIST"),
    ("text_term", "text_expr", ["param_term"], "LOWEST"),
    ("text_affix", "text_expr", ["affix"], "PREFIX"),
    ("text_verbatim", "text_expr", ["verbatim"], "VERBATIM"),
    ("termlist_new", "termlist", ["param_term", "param_term"], "TERMLIST"),
    ("termlist_add", "termlist", ["termlist", "param_term"], "TERMLIST"),
    ("not", "expr", ["MINUS", "expr"], None),
    ("text_not", "text_expr", ["MINUS", "text_expr"], None),
    ("optional", "expr", ["TILDE", "expr"], None),
    ("text_optional", "text_expr", ["TILDE", "text_expr"], None),
    ("affix_prefix", "affix", ["PREFIX"], None),
    ("affix_suffix", "affix", ["SUFFIX"], None),
    ("affix_contains", "affix", ["CONTAINS"], None),
    ("verbatim", "verbatim", ["WILDCARD"], None),
    ("fuzzy1", "text_expr", ["PERCENT", "param_term", "PERCENT"], "PREFIX"),
    ("fuzzy2", "text_expr", ["PERCENT", "PERCENT", "param_term", "PERCENT", "PERCENT"], "PREFIX"),
    ("fuzzy3", "text_expr", ["PERCENT", "PERCENT", "PERCENT", "param_term", "PERCENT", "PERCENT", "PERCENT"], "PREFIX"),
    ("modifier", "modifier", ["MODIFIER"], None),
    ("modifierlist_new", "modifierlist", ["modifier", "OR", "term"], None),
    ("modifierlist_add", "modifierlist", ["modifierlist", "OR", "term"], None),
    ("ismissing", "expr", ["ISMISSING", "LP", "modifier", "RP"], None),
    ("tag_expr", "expr", ["modifier", "COLON", "LB", "tag_list", "RB"], None),
    ("tags_term", "tag_list", ["param_term_case"], "TAGLIST"),
    ("tags_affix", "tag_list", ["affix"], "TAGLIST"),
    ("tags_verbatim", "tag_list", ["verbatim"], "TAGLIST"),
    ("tags_termlist", "tag_list", ["termlist"], "TAGLIST"),
    ("tags_add_term", "tag_list", ["tag_list", "OR", "param_term_case"], "TAGLIST"),
    ("tags_add_affix", "tag_list", ["tag_list", "OR", "affix"], "TAGLIST"),
    ("tags_add_verbatim", "tag_list", ["tag_list", "OR", "verbatim"], "TAGLIST"),
    ("tags_add_termlist", "tag_list", ["tag_list", "OR", "termlist"], "TAGLIST"),
    ("numeric_expr", "expr", ["modifier", "COLON", "numeric_range"], None),
    ("range_incl_incl", "numeric_range", ["LSQB", "param_num", "param_num", "RSQB"], "NUMBER"),
    ("range_excl_incl", "numeric_range", ["LSQB", "exclusive_param_num", "param_num", "RSQB"], "NUMBER"),
    ("range_incl_excl", "numeric_range", ["LSQB", "param_num", "exclusive_param_num", "RSQB"], "NUMBER"),
    ("range_excl_excl", "numeric_range", ["LSQB", "exclusive_param_num", "exclusive_param_num", "RSQB"], "NUMBER"),
    ("range_single", "numeric_range", ["LSQB", "param_num", "RSQB"], "NUMBER"),
    ("num_ne", "expr", ["modifier", "NOT_EQUAL", "param_num"], None),
    ("num_eq", "expr", ["modifier", "EQUALS", "param_num"], None),
    ("num_gt", "expr", ["modifier", "GT", "param_num"], None),
    ("num_ge", "expr", ["modifier", "GE", "param_num"], None),
    ("num_lt", "expr", ["modifier", "LT", "param_num"], None),
    ("num_le", "expr", ["modifier", "LE", "param_num"], None),
    ("geo_expr", "expr", ["modifier", "COLON", "geo_filter"], None),
    ("geo_filter", "geo_filter", ["LSQB", "param_num", "param_num", "param_num", "param_term", "RSQB"], "NUMBER"),
    ("geometry_expr", "expr", ["modifier", "COLON", "geometry_query"], None),
    ("geometry_query", "geometry_query", ["LSQB", "TERM", "ATTRIBUTE", "RSQB"], None),
    ("query_knn_expr", "query", ["expr", "ARROW", "LSQB", "vector_query", "RSQB"], None),
    ("query_knn_text", "query", ["text_expr", "ARROW", "LSQB", "vector_query", "RSQB"], None),
    ("query_knn_star", "query", ["star", "ARROW", "LSQB", "vector_query", "RSQB"], None),
    ("vq_attrs_score", "vector_query", ["vector_command", "vector_attribute_list", "vector_score_field"], None),
    ("vq_score", "vector_query", ["vector_command", "vector_score_field"], None),
    ("vq_attrs", "vector_query", ["vector_command", "vector_attribute_list"], None),
    ("vq", "vector_query", ["vector_command"], None),
    ("as", "as", ["AS_T"], None),
    ("score_field", "vector_score_field", ["as", "param_term_case"], None),
    ("query_knn_expr_attrs", "query", ["expr", "ARROW", "LSQB", "vector_query", "RSQB", "ARROW", "LB", "attribute_list", "RB"], None),
    ("query_knn_text_attrs", "query", ["text_expr", "ARROW", "LSQB", "vector_query", "RSQB", "ARROW", "LB", "attribute_list", "RB"], None),
    ("query_knn_star_attrs", "query", ["star", "ARROW", "LSQB", "vector_query", "RSQB", "ARROW", "LB", "attribute_list", "RB"], None),
    ("vector_command", "vector_command", ["TERM", "param_size", "modifier", "ATTRIBUTE"], None),
    ("vector_attribute", "vector_attribute", ["TERM", "param_term"], None),
    ("vattrs_add", "vector_attribute_list", ["vector_attribute_list", "vector_attribute"], None),
    ("vattrs_one", "vector_attribute_list", ["vector_attribute"], None),
    ("vector_range_expr", "expr", ["modifier", "COLON", "LSQB", "vector_range_command", "RSQB"], None),
    ("vector_range_command", "vector_range_command", ["TERM", "param_num", "ATTRIBUTE"], None),
    ("num_size", "num", ["SIZE"], None),
    ("num", "num", ["NUMBER"], None),
    ("num_negative", "num", ["MINUS", "num"], None),
    ("term", "term", ["TERM"], None),
    ("term_number", "term", ["NUMBER"], None),
    ("term_size", "term", ["SIZE"], None),
    ("param_term", "param_term", ["term"], None),
    ("param_term_attr", "param_term", ["ATTRIBUTE"], None),
    ("param_term_case", "param_term_case", ["term"], None),
    ("param_term_case_attr", "param_term_case", ["ATTRIBUTE"], None),
    ("param_size", "param_size", ["SIZE"], None),
    ("param_size_attr", "param_size", ["ATTRIBUTE"], None),
    ("param_num_attr", "param_num", ["ATTRIBUTE"], None),
    ("param_num_neg_attr", "param_num", ["MINUS", "ATTRIBUTE"], None),
    ("param_num", "param_num", ["num"], None),
    ("excl_num", "exclusive_param_num", ["LP", "num"], None),
    ("excl_attr", "exclusive_param_num", ["LP", "ATTRIBUTE"], None),
    ("excl_neg_attr", "exclusive_param_num", ["LP", "MINUS", "ATTRIBUTE"], None),
]

if DIALECT == "2":
    PREC, RULES, FALLBACK, PREFIX = PREC2, RULES2, FALLBACK2, "v2"
else:
    PREC, RULES, FALLBACK, PREFIX = PREC1, RULES1, {}, ""

NONTERMS = []
for _, lhs, _, _ in RULES:
    if lhs not in NONTERMS:
        NONTERMS.append(lhs)
TERMS = ["$"]
for _, syms in PREC:
    for t in syms:
        if t not in TERMS:
            TERMS.append(t)
for t in list(FALLBACK) + list(FALLBACK.values()):
    if t not in TERMS:
        TERMS.append(t)
for _, _, rhs, _ in RULES:
    for s in rhs:
        if s not in NONTERMS and s not in TERMS:
            TERMS.append(s)

prec, assoc = {}, {}
for i, (a, syms) in enumerate(PREC):
    for s in syms:
        prec[s], assoc[s] = i, a


def rule_prec(r):
    if r[3]:
        return r[3]
    for s in r[2]:
        if s in prec and s not in NONTERMS:
            return s
    return None


# Augmented grammar: rule -1 is START -> query.
G = [("start", "START", ["query"], None)] + RULES

nullable = set()
changed = True
while changed:
    changed = False
    for _, lhs, rhs, _ in G:
        if lhs not in nullable and all(s in nullable for s in rhs):
            nullable.add(lhs)
            changed = True

first = {t: {t} for t in TERMS}
for n in NONTERMS + ["START"]:
    first[n] = set()
changed = True
while changed:
    changed = False
    for _, lhs, rhs, _ in G:
        for s in rhs:
            before = len(first[lhs])
            first[lhs] |= first[s]
            if len(first[lhs]) != before:
                changed = True
            if s not in nullable:
                break


def first_seq(seq, la):
    out = set()
    for s in seq:
        out |= first[s]
        if s not in nullable:
            return out
    out.add(la)
    return out


def closure(items):
    items = set(items)
    todo = list(items)
    while todo:
        r, d, la = todo.pop()
        rhs = G[r][2]
        if d < len(rhs) and rhs[d] in NONTERMS + ["START"]:
            for la2 in first_seq(rhs[d + 1:], la):
                for i, g in enumerate(G):
                    if g[1] == rhs[d]:
                        it = (i, 0, la2)
                        if it not in items:
                            items.add(it)
                            todo.append(it)
    return frozenset(items)


def goto(items, sym):
    moved = [(r, d + 1, la) for r, d, la in items if d < len(G[r][2]) and G[r][2][d] == sym]
    return closure(moved) if moved else None


start = closure([(0, 0, "$")])
states = [start]
index = {start: 0}
edges = {}
i = 0
while i < len(states):
    st = states[i]
    syms = []
    for r, d, _ in st:
        if d < len(G[r][2]) and G[r][2][d] not in syms:
            syms.append(G[r][2][d])
    for s in syms:
        nxt = goto(st, s)
        if nxt not in index:
            index[nxt] = len(states)
            states.append(nxt)
        edges[(i, s)] = index[nxt]
    i += 1

# Merge states with the same core (LALR).
core_of = [frozenset((r, d) for r, d, _ in st) for st in states]
cores = {}
merged = []
for i, c in enumerate(core_of):
    if c not in cores:
        cores[c] = len(cores)
        merged.append(set())
    merged[cores[c]] |= states[i]
remap = [cores[c] for c in core_of]
nstates = len(cores)
mgoto = {}
for (s, sym), t in edges.items():
    mgoto[(remap[s], sym)] = remap[t]

SHIFT, REDUCE, ACCEPT = 1, 2, 3
actions = [dict() for _ in range(nstates)]
conflicts = []
for s in range(nstates):
    cand = {}
    for r, d, la in merged[s]:
        rhs = G[r][2]
        if d < len(rhs):
            sym = rhs[d]
            if sym in TERMS:
                cand.setdefault(sym, set()).add((SHIFT, mgoto[(s, sym)]))
        elif r == 0:
            cand.setdefault("$", set()).add((ACCEPT, 0))
        else:
            cand.setdefault(la, set()).add((REDUCE, r))
    for la, acts in cand.items():
        shifts = [a for a in acts if a[0] in (SHIFT, ACCEPT)]
        reduces = sorted(a for a in acts if a[0] == REDUCE)
        assert len(shifts) <= 1
        # Reduce/reduce: precedence, else the earlier rule.
        while len(reduces) > 1:
            x, y = reduces[0], reduces[1]
            px, py = rule_prec(G[x[1]]), rule_prec(G[y[1]])
            if px is None or py is None or prec[px] == prec[py]:
                conflicts.append(("RR", s, la))
                reduces.pop(1)
            elif prec[px] > prec[py]:
                reduces.pop(1)
            else:
                reduces.pop(0)
        act = None
        if shifts and reduces:
            ry = rule_prec(G[reduces[0][1]])
            if ry is None or la not in prec:
                conflicts.append(("SR", s, la))
                act = shifts[0]
            elif prec[la] > prec[ry]:
                act = shifts[0]
            elif prec[la] < prec[ry]:
                act = reduces[0]
            elif assoc[la] == "right":
                act = shifts[0]
            else:
                act = reduces[0]
        elif shifts:
            act = shifts[0]
        else:
            act = reduces[0]
        actions[s][la] = act

if conflicts:
    print("// unresolved conflicts: %d" % len(conflicts), file=sys.stderr)

# Lemon makes each state's most frequent reduction its default, the action
# for any token without an entry of its own, and drops the entries that
# equal it; the start symbol's rules never become defaults.
defaults = []
for s_ in range(nstates):
    counts = {}
    for t in TERMS:
        a = actions[s_].get(t)
        if a and a[0] == REDUCE and G[a[1]][1] != "query":
            counts[a[1]] = counts.get(a[1], 0) + 1
    best = 0
    for t in TERMS:
        a = actions[s_].get(t)
        if a and a[0] == REDUCE and a[1] in counts and (best == 0 or counts[a[1]] > counts[best]):
            best = a[1]
    defaults.append(best)
    if best:
        for t in TERMS:
            a = actions[s_].get(t)
            if a and a[0] == REDUCE and a[1] == best:
                del actions[s_][t]


def camel(name):
    return "".join(x.capitalize() for x in name.split("_"))


def ident(kind, name=""):
    if PREFIX:
        return PREFIX + camel(kind) + name
    return kind + name


out = []
w = out.append
w("// Code generated by scripts/gen_search_parser.py; DO NOT EDIT.")
w("")
w("package search")
w("")
w("// The terminals of RediSearch's DIALECT %s query grammar." % DIALECT)
w("const (")
for i, t in enumerate(TERMS):
    name = ident("tok", "EOF" if t == "$" else t)
    w("\t%s = %d" % (name, i))
w(")")
w("")
w("const %s = %d" % (ident("num_tokens") if PREFIX else "numTokens", len(TERMS)))
w("")
w("// The rules, by number.")
w("const (")
for i, g in enumerate(G):
    if i == 0:
        continue
    w("\t%s = %d" % (ident("rule", camel(g[0])), i))
w(")")
w("")
w("var %s = [...]int8{%s}" % (ident("rule_l_h_s") if PREFIX else "ruleLHS", ", ".join(str(NONTERMS.index(g[1])) if g[1] in NONTERMS else "-1" for g in G)))
w("var %s = [...]int8{%s}" % (ident("rule_len") if PREFIX else "ruleLen", ", ".join(str(len(g[2])) for g in G)))
w("")
if not PREFIX:
    w("// parseActions[state][token]: 0 is no entry, n > 0 shifts to state n-1,")
    w("// n < 0 reduces by rule -n, and acceptAction accepts.")
    w("const acceptAction = %d" % 32767)
    w("")
w("var %s = [%d][%d]int16{" % (ident("actions") if PREFIX else "parseActions", nstates, len(TERMS)))
for s_ in range(nstates):
    row = []
    for t in TERMS:
        a = actions[s_].get(t)
        if a is None:
            row.append(0)
        elif a[0] == SHIFT:
            row.append(a[1] + 1)
        elif a[0] == REDUCE:
            row.append(-a[1])
        else:
            row.append(32767)
    w("\t{%s}," % ", ".join(map(str, row)))
w("}")
w("")
w("// The rule each state reduces by on a token without an entry.")
w("var %s = [%d]int8{%s}" % (ident("defaults") if PREFIX else "parseDefaults", nstates, ", ".join(map(str, defaults))))
w("")
if FALLBACK:
    w("// The token a token stands for where a state has no entry for it.")
    w("var %s = [%d]int8{%s}" % (ident("fallback"), len(TERMS), ", ".join(str(TERMS.index(FALLBACK[t])) if t in FALLBACK else "0" for t in TERMS)))
    w("")
w("// The state after a reduction, plus one, by state and nonterminal.")
w("var %s = [%d][%d]int16{" % (ident("gotos") if PREFIX else "parseGotos", nstates, len(NONTERMS)))
for s_ in range(nstates):
    w("\t{%s}," % ", ".join(str(mgoto.get((s_, n), -1) + 1) for n in NONTERMS))
w("}")
print("\n".join(out))
