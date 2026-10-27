import os
import pathlib
import socket
import subprocess
import tempfile
import time


def encode(*args):
    args = [arg if isinstance(arg, bytes) else arg.encode() for arg in args]
    return b"*%d\r\n" % len(args) + b"".join(b"$%d\r\n" % len(arg) + arg + b"\r\n" for arg in args)


def read_reply(reader):
    line = reader.readline()
    if not line.endswith(b"\r\n"):
        raise RuntimeError("truncated reply")
    kind, body = line[:1], line[1:-2]
    if kind == b":":
        return int(body)
    if kind == b"*":
        count = int(body)
        return None if count == -1 else [read_reply(reader) for _ in range(count)]
    if kind == b"+":
        return body
    if kind == b"-":
        raise RuntimeError(body.decode("utf-8", "replace"))
    if kind == b"$":
        size = int(body)
        if size == -1:
            return None
        value = reader.read(size)
        if len(value) != size or reader.read(2) != b"\r\n":
            raise RuntimeError("truncated bulk reply")
        return value
    raise RuntimeError(f"unexpected reply: {line!r}")


def query(port, *commands):
    with socket.create_connection(("127.0.0.1", port), timeout=2) as conn:
        conn.sendall(b"".join(encode(*command) for command in commands))
        with conn.makefile("rb") as reader:
            result = None
            for _ in commands:
                result = read_reply(reader)
            return result


def check(actual, expected):
    if actual != expected:
        raise RuntimeError(f"expected {expected!r}, got {actual!r}")


def main():
    repo = pathlib.Path(__file__).resolve().parent.parent
    with tempfile.TemporaryDirectory(prefix="cachestorm-aof-smoke-") as tmp:
        work = pathlib.Path(tmp)
        binary = pathlib.Path(os.environ.get("BIN", work / "cachestorm")).resolve()
        if "BIN" not in os.environ:
            subprocess.run(["go", "build", "-o", str(binary), "./cmd/cachestorm"], cwd=repo, check=True)
        processes = []

        def launch(directory, memory="0"):
            with socket.socket() as reservation:
                reservation.bind(("127.0.0.1", 0))
                port = reservation.getsockname()[1]
            cfg, log = directory / "config.yaml", directory / "server.log"
            cfg.write_text(
                f"server:\n  bind: 127.0.0.1\n  port: {port}\n"
                "http:\n  enabled: false\nplugins:\n  metrics:\n    enabled: false\n"
                "sentinel:\n  enabled: false\nlogging:\n  level: error\n"
                f"memory:\n  max_memory: {memory}\n  eviction_policy: noeviction\n"
                f"persistence:\n  enabled: true\n  aof: true\n  aof_sync: always\n  snapshot_interval: '0'\n  data_dir: {directory}\n"
            )
            with log.open("ab") as output:
                proc = subprocess.Popen([str(binary), "--config", str(cfg)], cwd=repo, stdout=output, stderr=output)
            processes.append(proc)
            return proc, port, log

        def ready(proc, port, log):
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                if proc.poll() is not None:
                    raise RuntimeError(f"server exited: {log.read_text()}")
                try:
                    check(query(port, ("PING",)), b"PONG")
                    return
                except OSError:
                    time.sleep(0.02)
            raise RuntimeError("server did not listen")

        try:
            prefix = encode("SET", "before", "kept")
            bad_sequences = [
                encode("UNKNOWN.REPLAY"), encode("SET", "missing-value"),
                encode("HSET", "before", "f", "v"), encode("SELECT", "999"),
                encode("MULTI") + encode("MULTI") + encode("EXEC"),
                encode("EXEC"), encode("MULTI", "invalid") + encode("EXEC"),
                encode("MULTI") + encode("INCR", "before") + encode("EXEC"),
                encode("UNKNOWN.REPLAY") + b"*3\r\n$3\r\nSET\r\n$8\r\npartial",
            ]
            for i, bad in enumerate(bad_sequences):
                directory = work / f"invalid-{i}"
                directory.mkdir()
                aof = directory / "appendonly.aof"
                source = prefix + bad
                aof.write_bytes(source)
                proc, _, log = launch(directory)
                if proc.wait(timeout=15) == 0:
                    raise RuntimeError(f"invalid AOF accepted: {log.read_text()}")
                check(aof.read_bytes(), source)

            parts = [encode("MULTI"), encode("SELECT", "3"), encode("SET", "ghost", "uncommitted"), encode("EXEC")[:-2]]
            for count in range(1, len(parts) + 1):
                directory = work / f"torn-{count}"
                directory.mkdir()
                aof = directory / "appendonly.aof"
                stable = encode("SELECT", "2") + prefix
                aof.write_bytes(stable + b"".join(parts[:count]))
                first, port, log = launch(directory)
                ready(first, port, log)
                recovered = aof.read_bytes()
                if stable not in recovered or b"MULTI" in recovered or b"ghost" in recovered:
                    raise RuntimeError("torn transaction not replaced by a clean committed snapshot")
                check(query(port, ("SELECT", "2"), ("GET", "before")), b"kept")
                check(query(port, ("SELECT", "3"), ("GET", "ghost")), None)
                check(query(port, ("SET", "acknowledged", "durable")), b"OK")
                first.kill()
                first.wait(timeout=10)
                second, port, log = launch(directory)
                ready(second, port, log)
                check(query(port, ("GET", "acknowledged")), b"durable")
                check(query(port, ("SELECT", "2"), ("GET", "before")), b"kept")
                second.terminate()
                check(second.wait(timeout=10), 0)
            directory = work / "expiry-history"
            directory.mkdir()
            first, port, log = launch(directory)
            ready(first, port, log)
            check(query(port, ("SET", "deadline", "value", "PX", "60000")), b"OK")
            deadline = query(port, ("PEXPIRETIME", "deadline"))
            check(query(port, ("PFADD", "hll", "a", "b", "")), 1)
            check(query(port, ("PFCOUNT", "hll")), 3)
            check(query(port, ("SET", "persisted", "whole", "PX", "200")), b"OK")
            check(query(port, ("APPEND", "persisted", "-value")), 11)
            check(query(port, ("PERSIST", "persisted")), 1)
            check(query(port, ("SET", "rejected", "old", "PX", "200")), b"OK")
            check(query(port, ("SET", "rejected", "ghost", "NX")), None)
            check(query(port, ("HSET", "dump-source", "f&=", "v&=\x00")), 1)
            payload = query(port, ("DUMP", "dump-source"))
            check(query(port, ("RESTORE", "restored", "200", payload)), b"OK")
            check(query(port, ("HSET", "restored", "after", "two")), 1)
            check(query(port, ("PERSIST", "restored")), 1)
            check(query(port, ("RESTORE", "restore-deadline", "60000", payload)), b"OK")
            restore_deadline = query(port, ("PEXPIRETIME", "restore-deadline"))
            check(query(port, ("FT.CREATE", "docs", "SCHEMA", "title", "TEXT")), b"OK")
            check(query(port, ("FT.ADD", "docs", "d1", "1", "FIELDS", "title", "durable search")), b"OK")
            check(query(port, ("FT.ALIASADD", "docs_alias", "docs")), b"OK")
            first.kill()
            first.wait(timeout=10)
            time.sleep(0.25)
            second, port, log = launch(directory)
            ready(second, port, log)
            check(query(port, ("PEXPIRETIME", "deadline")), deadline)
            check(query(port, ("PFCOUNT", "hll")), 3)
            if not query(port, ("GET", "hll")).startswith(b"HYLL"):
                raise RuntimeError("AOF restored HLL diagnostic text")
            check(query(port, ("GET", "persisted")), b"whole-value")
            check(query(port, ("GET", "rejected")), None)
            check(query(port, ("HGET", "restored", "f&=")), b"v&=\x00")
            check(query(port, ("HGET", "restored", "after")), b"two")
            check(query(port, ("PEXPIRETIME", "restore-deadline")), restore_deadline)
            check(query(port, ("FT.SEARCH", "docs_alias", "durable", "NOCONTENT")), [1, b"d1"])
            second.terminate()
            check(second.wait(timeout=10), 0)
            directory = work / "bitmap-memory"
            directory.mkdir()
            first, port, log = launch(directory, "4kb")
            ready(first, port, log)
            check(query(port, ("SETBIT", "bounded", "0", "1")), 0)
            check(query(port, ("PEXPIRE", "bounded", "60000")), 1)
            bitmap_deadline = query(port, ("PEXPIRETIME", "bounded"))
            before = (directory / "appendonly.aof").read_bytes()
            for command in [
                ("SETBIT", "bounded", "4294967295", "1"),
                ("BITFIELD", "bounded", "SET", "u8", "0", "7", "SET", "i64", "4294967295", "1"),
            ]:
                try:
                    query(port, command)
                except RuntimeError as error:
                    if "OOM" not in str(error):
                        raise
                else:
                    raise RuntimeError("bitmap allocation exceeded configured memory")
            check(query(port, ("GET", "bounded")), b"\x80")
            check(query(port, ("PEXPIRETIME", "bounded")), bitmap_deadline)
            check(query(port, ("BITFIELD", "missing", "GET", "u8", "100")), [0])
            check(query(port, ("BITFIELD_RO", "missing", "GET", "i64", "0")), [0])
            check(query(port, ("EXISTS", "missing")), 0)
            check((directory / "appendonly.aof").read_bytes(), before)
            first.kill()
            first.wait(timeout=10)
            second, port, log = launch(directory, "4kb")
            ready(second, port, log)
            check(query(port, ("GET", "bounded")), b"\x80")
            check(query(port, ("PEXPIRETIME", "bounded")), bitmap_deadline)
            check(query(port, ("EXISTS", "missing")), 0)
            second.terminate()
            check(second.wait(timeout=10), 0)
            print("PASS: invalid AOF stops startup unchanged; torn transactions are removed; acknowledged writes, original expiry deadlines, persisted HLL history, search indexes and bounded bitmap writes survive SIGKILL and restart in the correct database")
        finally:
            for proc in processes:
                if proc.poll() is None:
                    proc.kill()
                proc.wait(timeout=10)


if __name__ == "__main__":
    main()
