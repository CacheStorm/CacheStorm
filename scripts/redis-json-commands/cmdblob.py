"""Build Redis COMMAND INFO / COMMAND DOCS RESP3 blobs from a Redis source
tree's src/commands/*.json, following server.c's addReplyCommandInfo and
addReplyCommandDocs (Redis 8.10.3).

REDIS_COMMANDS_DIR names the directory holding the JSON files. Checked
against the entries scripts/gen-command-specs.go took from live Redis 7.4 and
8.2 servers: they match byte for byte where the specs did not change."""
import json, glob, os, sys

D = os.environ['REDIS_COMMANDS_DIR']
CMDS = {}
for f in glob.glob(D + '/*.json'):
    for name, spec in json.load(open(f)).items():
        spec['_name'] = name
        spec['_file'] = os.path.basename(f)[:-5]
        CMDS.setdefault(spec.get('container', ''), []).append(spec)

CMD_FLAGS = ['WRITE', 'READONLY', 'DENYOOM', 'MODULE', 'ADMIN', 'PUBSUB', 'NOSCRIPT', 'BLOCKING', 'LOADING', 'STALE',
             'SKIP_MONITOR', 'SKIP_SLOWLOG', 'ASKING', 'FAST', 'NO_AUTH', 'NO_MANDATORY_KEYS', 'NO_ASYNC_LOADING',
             'NO_MULTI', 'MOVABLE_KEYS', 'ALLOW_BUSY', 'SCRIPT_RUNNER']
FLAG_NAME = {'MOVABLE_KEYS': 'movablekeys'}
ACL = ['keyspace', 'read', 'write', 'set', 'sortedset', 'list', 'hash', 'string', 'array', 'bitmap', 'hyperloglog', 'geo',
       'stream', 'pubsub', 'admin', 'fast', 'slow', 'blocking', 'dangerous', 'connection', 'transaction', 'scripting']
KEY_FLAGS = ['RO', 'RW', 'OW', 'RM', 'ACCESS', 'UPDATE', 'INSERT', 'DELETE', 'NOT_KEY', 'INCOMPLETE', 'VARIABLE_FLAGS']
KEY_FLAG_NAME = {'ACCESS': 'access', 'UPDATE': 'update', 'INSERT': 'insert', 'DELETE': 'delete', 'NOT_KEY': 'not_key',
                 'INCOMPLETE': 'incomplete', 'VARIABLE_FLAGS': 'variable_flags'}


def bulk(s):
    b = s.encode()
    return b'$%d\r\n%s\r\n' % (len(b), b)


def integer(n): return b':%d\r\n' % n
def aset(items): return b'~%d\r\n' % len(items) + b''.join(items)
def arr(items): return b'*%d\r\n' % len(items) + b''.join(items)
def amap(pairs): return b'%%%d\r\n' % len(pairs) + b''.join(bulk(k) + v for k, v in pairs)
def status(s): return b'+' + s.encode() + b'\r\n'


def flags_of(spec):
    fl = set(spec.get('command_flags', []))
    ks = spec.get('key_specs', [])
    # populateCommandLegacyRangeSpec
    first, last, step = 0, 0, 0
    if ks:
        if len(ks) == 1 and 'index' in ks[0]['begin_search'] and 'range' in ks[0]['find_keys']:
            k = ks[0]
            first = k['begin_search']['index']['pos']
            r = k['find_keys']['range']
            last = r['lastkey'] + first if r['lastkey'] >= 0 else r['lastkey']
            step = r['step']
            if 'INCOMPLETE' in k.get('flags', []):
                fl.add('MOVABLE_KEYS')
        else:
            fk, lk, prev = 1 << 31, 0, 0
            for k in ks:
                if 'index' not in k['begin_search'] or 'range' not in k['find_keys']:
                    fl.add('MOVABLE_KEYS'); continue
                pos = k['begin_search']['index']['pos']
                r = k['find_keys']['range']
                if r['step'] != 1 or (prev and prev != pos - 1):
                    fl.add('MOVABLE_KEYS'); continue
                if 'INCOMPLETE' in k.get('flags', []):
                    fl.add('MOVABLE_KEYS')
                fk = min(fk, pos)
                la = r['lastkey']
                if la >= 0:
                    la += pos
                lk = max(lk & 0xffffffff, la & 0xffffffff)
                if lk >= 1 << 31:
                    lk -= 1 << 32
                prev = lk
            if fk == 1 << 31:
                fl.add('MOVABLE_KEYS')
            else:
                rel = lk if lk < 0 else lk - fk
                first, step = fk, 1
                last = rel + first if rel >= 0 else rel
    return fl, first, last, step


def acl_of(spec, fl):
    cats = set(c.lower() for c in spec.get('acl_categories', []))
    if 'WRITE' in fl: cats.add('write')
    if 'READONLY' in fl and 'scripting' not in cats: cats.add('read')
    if 'ADMIN' in fl: cats |= {'admin', 'dangerous'}
    if 'PUBSUB' in fl: cats.add('pubsub')
    if 'FAST' in fl: cats.add('fast')
    if 'BLOCKING' in fl: cats.add('blocking')
    if 'fast' not in cats: cats.add('slow')
    return [c for c in ACL if c in cats]


def key_specs(spec):
    out = []
    for k in spec.get('key_specs', []):
        pairs = []
        if k.get('notes'): pairs.append(('notes', bulk(k['notes'])))
        kf = k.get('flags', [])
        pairs.append(('flags', aset([status(KEY_FLAG_NAME.get(f, f)) for f in KEY_FLAGS if f in kf])))
        bs = k['begin_search']
        if 'index' in bs:
            b = amap([('type', bulk('index')), ('spec', amap([('index', integer(bs['index']['pos']))]))])
        elif 'keyword' in bs:
            b = amap([('type', bulk('keyword')), ('spec', amap([('keyword', bulk(bs['keyword']['keyword'])), ('startfrom', integer(bs['keyword']['startfrom']))]))])
        else:
            b = amap([('type', bulk('unknown')), ('spec', amap([]))])
        pairs.append(('begin_search', b))
        fk = k['find_keys']
        if 'range' in fk:
            r = fk['range']
            f = amap([('type', bulk('range')), ('spec', amap([('lastkey', integer(r['lastkey'])), ('keystep', integer(r['step'])), ('limit', integer(r['limit']))]))])
        elif 'keynum' in fk:
            r = fk['keynum']
            f = amap([('type', bulk('keynum')), ('spec', amap([('keynumidx', integer(r['keynumidx'])), ('firstkey', integer(r['firstkey'])), ('keystep', integer(r['step']))]))])
        else:
            f = amap([('type', bulk('unknown')), ('spec', amap([]))])
        pairs.append(('find_keys', f))
        out.append(amap(pairs))
    return aset(out)


def fullname(spec):
    if spec.get('container'):
        return spec['container'].lower() + '|' + spec['_name'].lower()
    return spec['_name'].lower()


def subs_of(spec, order=None):
    if spec.get('container'):
        return []
    subs = CMDS.get(spec['_name'], [])
    if order:
        rank = {n: i for i, n in enumerate(order)}
        subs = sorted(subs, key=lambda s: rank.get(fullname(s), 1 << 30))
    return subs


def info(spec, order=None):
    fl, first, last, step = flags_of(spec)
    subs = subs_of(spec, order)
    tips = [bulk(t.lower()) for t in spec.get('command_tips', [])]
    return arr([bulk(fullname(spec)), integer(spec['arity']),
                aset([status(FLAG_NAME.get(f, f.lower())) for f in CMD_FLAGS if f in fl]),
                integer(first), integer(last), integer(step),
                aset([status('@' + c) for c in acl_of(spec, fl)]),
                aset(tips), key_specs(spec),
                (arr([info(s) for s in subs]) if subs else aset([]))])


def args_list(args):
    out = []
    for a in args:
        pairs = [('name', bulk(a['name'])), ('type', bulk(a['type']))]
        block = a['type'] in ('oneof', 'block')
        if not block:
            pairs.append(('display_text', bulk(a.get('display', a['name']))))
        if 'key_spec_index' in a: pairs.append(('key_spec_index', integer(a['key_spec_index'])))
        if 'token' in a: pairs.append(('token', bulk(a['token'])))
        if 'summary' in a: pairs.append(('summary', bulk(a['summary'])))
        if 'since' in a: pairs.append(('since', bulk(a['since'])))
        if 'deprecated_since' in a: pairs.append(('deprecated_since', bulk(a['deprecated_since'])))
        fl = [f for f, k in (('optional', 'optional'), ('multiple', 'multiple'), ('multiple_token', 'multiple_token')) if a.get(k)]
        if fl: pairs.append(('flags', aset([status(f) for f in fl])))
        if block: pairs.append(('arguments', args_list(a.get('arguments', []))))
        out.append(amap(pairs))
    return arr(out)


def docs(spec, order=None):
    pairs = []
    if spec.get('summary'): pairs.append(('summary', bulk(spec['summary'])))
    if spec.get('since'): pairs.append(('since', bulk(spec['since'])))
    pairs.append(('group', bulk(spec['group'].replace('_', '-'))))
    if spec.get('complexity'): pairs.append(('complexity', bulk(spec['complexity'])))
    if spec.get('doc_flags'):
        pairs.append(('doc_flags', aset([status(f.lower()) for f in ('DEPRECATED', 'SYSCMD') if f in spec['doc_flags']])))
    if spec.get('deprecated_since'): pairs.append(('deprecated_since', bulk(spec['deprecated_since'])))
    if spec.get('replaced_by'): pairs.append(('replaced_by', bulk(spec['replaced_by'])))
    if spec.get('history'):
        pairs.append(('history', aset([arr([bulk(h[0]), bulk(h[1])]) for h in spec['history']])))
    if spec.get('arguments'): pairs.append(('arguments', args_list(spec['arguments'])))
    subs = subs_of(spec, order)
    if subs:
        pairs.append(('subcommands', b'%%%d\r\n' % len(subs) + b''.join(bulk(fullname(s)) + docs(s) for s in subs)))
    return amap(pairs)


def top(name):
    for s in CMDS['']:
        if s['_name'].lower() == name.lower():
            return s
    raise KeyError(name)


if __name__ == '__main__':
    kind, name = sys.argv[1], sys.argv[2]
    s = top(name)
    sys.stdout.buffer.write(info(s) if kind == 'info' else docs(s))
