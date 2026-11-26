"""Insert or replace commands in internal/command's generated tables with
entries built from Redis' src/commands JSON (see cmdblob.py), for the
commands of Redis releases there is no server image of to run
scripts/gen-command-specs.go against.

usage: REDIS_COMMANDS_DIR=<redis>/src/commands patchtables.py <repo> name...
Run gofmt on the two table files afterwards."""
import re, sys, os
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import cmdblob

repo, names = sys.argv[1], sys.argv[2:]


def goquote(b):
    out = []
    for ch in b:
        c = chr(ch)
        if c == '\r': out.append('\\r')
        elif c == '\n': out.append('\\n')
        elif c == '"': out.append('\\"')
        elif c == '\\': out.append('\\\\')
        elif 32 <= ch < 127: out.append(c)
        else: out.append('\\x%02x' % ch)
    return '"' + ''.join(out) + '"'


def upsert(text, var, key, value):
    start = text.index('var %s = map[string]' % var)
    end = text.index('\n}\n', start)
    body = text[start:end]
    lines = body.split('\n')
    pat = re.compile(r'^\t"([^"]+)":')
    newline = '\t%s: %s,' % (goquote(key.encode()), value)
    for i, l in enumerate(lines):
        m = pat.match(l)
        if m and m.group(1) == key:
            lines[i] = newline
            break
    else:
        idx = [i for i, l in enumerate(lines) if pat.match(l)]
        pos = next((i for i in idx if pat.match(lines[i]).group(1) > key), idx[-1] + 1)
        lines.insert(pos, newline)
    return text[:start] + '\n'.join(lines) + text[end:]


data = os.path.join(repo, 'internal/command/redis_command_data.go')
specs = os.path.join(repo, 'internal/command/redis_command_specs.go')
dt, st = open(data).read(), open(specs).read()
for n in names:
    s = cmdblob.top(n)
    dt = upsert(dt, 'redisCommandInfoRESP3', n.lower(), goquote(cmdblob.info(s)))
    dt = upsert(dt, 'redisCommandDocsRESP3', n.lower(), goquote(cmdblob.docs(s)))
    fl, first, last, step = cmdblob.flags_of(s)
    flags = [cmdblob.FLAG_NAME.get(f, f.lower()) for f in cmdblob.CMD_FLAGS if f in fl]
    cats = cmdblob.acl_of(s, fl)
    q = lambda xs: '[]string{' + ', '.join('"%s"' % x for x in xs) + '}'
    st = upsert(st, 'redisNativeCommandSpecs', n.upper(),
                '{Arity: %d, Flags: %s, FirstKey: %d, LastKey: %d, KeyStep: %d, Categories: %s}' % (s['arity'], q(flags), first, last, step, q(cats)))
open(data, 'w').write(dt)
open(specs, 'w').write(st)
