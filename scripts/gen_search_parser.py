#!/usr/bin/env python3
"""Generates internal/search/dialect1_tables.go: the LALR(1) tables of
RediSearch's DIALECT 1 query grammar (src/query_parser/v1/parser.y), with
conflicts resolved the way lemon resolves them, so queries parse as Redis
parses them.

    python3 scripts/gen_search_parser.py > internal/search/dialect1_tables.go
"""

# Precedence, lowest first, as parser.y declares it.
PREC = [
    ("left", ["LOWEST"]), ("left", ["TILDE"]), ("left", ["TAGLIST"]), ("left", ["QUOTE"]),
    ("left", ["COLON"]), ("left", ["MINUS"]), ("left", ["NUMBER"]), ("left", ["STOPWORD"]),
    ("left", ["TERMLIST"]), ("left", ["TERM"]), ("left", ["PREFIX", "SUFFIX", "CONTAINS"]),
    ("left", ["PERCENT"]), ("left", ["ATTRIBUTE"]), ("right", ["LP"]), ("left", ["RP"]),
    ("left", ["MODIFIER"]), ("left", ["AND"]), ("left", ["OR"]), ("left", ["ORX"]), ("left", ["ARROW"]),
]

# Rules in parser.y order: (name, lhs, rhs, explicit precedence).
RULES = [
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

NONTERMS = []
for _, lhs, _, _ in RULES:
    if lhs not in NONTERMS:
        NONTERMS.append(lhs)
TERMS = ["$"]
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

import sys
if conflicts:
    print("// unresolved conflicts: %d" % len(conflicts), file=sys.stderr)

out = []
w = out.append
w("// Code generated by scripts/gen_search_parser.py; DO NOT EDIT.")
w("")
w("package search")
w("")
w("// The terminals of RediSearch's DIALECT 1 query grammar.")
w("const (")
for i, t in enumerate(TERMS):
    name = "tokEOF" if t == "$" else "tok" + t
    w("\t%s = %d" % (name, i) if i else "\t%s = iota" % name)
w(")")
w("")
w("const numTokens = %d" % len(TERMS))
w("")
w("// The rules, by number: the nonterminal they make and their length.")
w("const (")
for i, g in enumerate(G):
    if i == 0:
        continue
    w("\trule%s = %d" % ("".join(x.capitalize() for x in g[0].split("_")), i))
w(")")
w("")
w("var ruleLHS = [...]int8{%s}" % ", ".join(str(NONTERMS.index(g[1])) if g[1] in NONTERMS else "-1" for g in G))
w("var ruleLen = [...]int8{%s}" % ", ".join(str(len(g[2])) for g in G))
w("")
w("// parseActions[state][token]: 0 is an error, n > 0 shifts to state n-1,")
w("// n < 0 reduces by rule -n, and acceptAction accepts.")
w("const acceptAction = %d" % 32767)
w("")
w("var parseActions = [%d][numTokens]int16{" % nstates)
for s in range(nstates):
    row = []
    for t in TERMS:
        a = actions[s].get(t)
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
# Lemon makes each state's most frequent reduction its default: the
# action for any token the state has no entry for.
w("// parseDefaults[state] is the rule a state reduces by on any other token.")
defaults = []
for s_ in range(nstates):
    counts = {}
    for t in TERMS:
        a = actions[s_].get(t)
        if a and a[0] == REDUCE:
            counts[a[1]] = counts.get(a[1], 0) + 1
    best = 0
    for t in TERMS:
        a = actions[s_].get(t)
        if a and a[0] == REDUCE and (best == 0 or counts[a[1]] > counts[best]):
            best = a[1]
    defaults.append(best)
w("var parseDefaults = [%d]int8{%s}" % (nstates, ", ".join(map(str, defaults))))
w("")
w("// parseGotos[state][nonterminal] is the state after a reduction, plus one.")
w("var parseGotos = [%d][%d]int16{" % (nstates, len(NONTERMS)))
for s in range(nstates):
    w("\t{%s}," % ", ".join(str(mgoto.get((s, n), -1) + 1) for n in NONTERMS))
w("}")
print("\n".join(out))
