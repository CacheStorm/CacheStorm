import os
import pathlib
import socket
import subprocess
import tempfile
import time


class Client:
    def __init__(self, conn):
        self.conn = conn
        self.reader = conn.makefile("rb")

    def read(self):
        line = self.reader.readline()
        if not line.endswith(b"\r\n"):
            raise RuntimeError("truncated RESP reply")
        prefix, body = line[:1], line[1:-2]
        if prefix == b"+":
            return body
        if prefix == b"-":
            raise RuntimeError(body.decode("utf-8", "replace"))
        if prefix == b":":
            return int(body)
        if prefix == b"$":
            size = int(body)
            if size == -1:
                return None
            data = self.reader.read(size)
            if len(data) != size or self.reader.read(2) != b"\r\n":
                raise RuntimeError("truncated RESP bulk string")
            return data
        if prefix == b"*":
            size = int(body)
            return None if size == -1 else [self.read() for _ in range(size)]
        raise RuntimeError(f"unknown RESP prefix {prefix!r}")

    def do(self, *args):
        args = [a if isinstance(a, bytes) else str(a).encode() for a in args]
        self.conn.sendall(
            b"*%d\r\n" % len(args)
            + b"".join(b"$%d\r\n" % len(a) + a + b"\r\n" for a in args)
        )
        return self.read()

    def close(self):
        self.reader.close()
        self.conn.close()


def check(actual, expected):
    if actual != expected:
        raise RuntimeError(f"EXPECTED {expected!r}; ACTUAL {actual!r}")


def main():
    repo = pathlib.Path(__file__).resolve().parent.parent
    with tempfile.TemporaryDirectory(prefix="cachestorm-snapshot-smoke-") as tmp:
        work = pathlib.Path(tmp)
        binary = pathlib.Path(os.environ.get("BIN", work / "cachestorm")).resolve()
        if "BIN" not in os.environ:
            subprocess.run(["go", "build", "-o", str(binary), "./cmd/cachestorm"], cwd=repo, check=True)
        cfg = work / "config.yaml"
        processes, clients = [], []
        log = work / "server.log"

        def launch():
            with socket.socket() as reservation:
                reservation.bind(("127.0.0.1", 0))
                port = reservation.getsockname()[1]
            cfg.write_text(
                f"server:\n  bind: 127.0.0.1\n  port: {port}\n"
                "http:\n  enabled: false\nplugins:\n  metrics:\n    enabled: false\n"
                "sentinel:\n  enabled: false\nlogging:\n  level: error\n"
                f"persistence:\n  enabled: true\n  aof: false\n  snapshot_interval: '0'\n  data_dir: {work}\n"
            )
            with log.open("ab") as output:
                proc = subprocess.Popen([str(binary), "-config", str(cfg)], cwd=repo, stdout=output, stderr=output)
            processes.append(proc)
            return proc, port

        def ready(proc, port):
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                if proc.poll() is not None:
                    raise RuntimeError(f"server exited: {log.read_text()}")
                try:
                    conn = socket.create_connection(("127.0.0.1", port), timeout=2)
                except OSError:
                    time.sleep(0.02)
                    continue
                client = Client(conn)
                clients.append(client)
                check(client.do("PING"), b"PONG")
                return client
            raise RuntimeError("server did not become ready")

        try:
            first, port = launch()
            c = ready(first, port)
            check(c.do("FUNCTION", "LOAD", "#!lua name=smoke\nredis.register_function('read_key', function(keys) return redis.call('GET', keys[1]) end)"), b"smoke")
            check(c.do("SET", "same", "zero", "PXAT", "16725225600000"), b"OK")
            check(c.do("PFADD", "hll", "a", "b", ""), 1)
            check(c.do("PFCOUNT", "hll"), 3)
            check(c.do("SELECT", "3"), b"OK")
            check(c.do("SET", "same", "three"), b"OK")
            check(c.do("XADD", "events", "1-0", b"\xfffield", "one", b"\xfffield", "two"), b"1-0")
            check(c.do("XGROUP", "CREATE", "events", "workers", "0"), b"OK")
            c.do("XREADGROUP", "GROUP", "workers", "consumer", "STREAMS", "events", ">")
            check(c.do("FT.CREATE", "docs", "SCHEMA", "title", "TEXT"), b"OK")
            check(c.do("FT.ADD", "docs", "d1", "1", "FIELDS", "title", "snapshot smoke"), b"OK")
            check(c.do("FT.ALIASADD", "docs_alias", "docs"), b"OK")
            check(c.do("SAVE"), b"OK")
            first.kill()
            first.wait(timeout=10)
            c.close()
            clients.remove(c)

            second, port = launch()
            r = ready(second, port)
            check(r.do("GET", "same"), b"zero")
            check(r.do("PFCOUNT", "hll"), 3)
            if not r.do("GET", "hll").startswith(b"HYLL"):
                raise RuntimeError("snapshot restored HLL diagnostic text")
            check(r.do("PEXPIRETIME", "same"), 16725225600000)
            check(r.do("SELECT", "3"), b"OK")
            check(r.do("FCALL", "read_key", "1", "same"), b"three")
            check(r.do("XPENDING", "events", "workers")[0], 1)
            check(r.do("XRANGE", "events", "-", "+"), [[b"1-0", [b"\xfffield", b"one", b"\xfffield", b"two"]]])
            check(r.do("FT.SEARCH", "docs_alias", "smoke", "NOCONTENT"), [1, b"d1"])
            second.kill()
            second.wait(timeout=10)
            r.close()
            clients.remove(r)

            snapshot = work / "dump.rdb"
            data = bytearray(snapshot.read_bytes())
            data[len(data) // 2] ^= 1
            snapshot.write_bytes(data)
            corrupt, _ = launch()
            if corrupt.wait(timeout=10) == 0 or "checksum" not in log.read_text():
                raise RuntimeError("corrupt snapshot did not stop startup")
            print("PASS: SAVE survives SIGKILL; databases, TTL, HLL registers, stream groups, functions and search indexes restored; corrupt snapshot refused")
        finally:
            for client in clients:
                client.close()
            for proc in processes:
                if proc.poll() is None:
                    proc.kill()
                proc.wait(timeout=10)


if __name__ == "__main__":
    main()
