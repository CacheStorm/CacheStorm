import os
import pathlib
import runpy
import socket
import subprocess
import tempfile
import time


def main():
    repo = pathlib.Path(__file__).resolve().parent.parent
    protocol = runpy.run_path(str(repo / "scripts" / "aof-recovery-smoke.py"))
    query, check = protocol["query"], protocol["check"]
    with tempfile.TemporaryDirectory(prefix="cachestorm-directory-smoke-") as tmp:
        work = pathlib.Path(tmp)
        data = work / "data"
        binary = pathlib.Path(os.environ.get("BIN", work / "cachestorm")).resolve()
        if "BIN" not in os.environ:
            subprocess.run(["go", "build", "-o", str(binary), "./cmd/cachestorm"], cwd=repo, check=True)
        processes = []

        def launch(directory):
            with socket.socket() as reservation:
                reservation.bind(("127.0.0.1", 0))
                port = reservation.getsockname()[1]
            number = len(processes)
            cfg, log = work / f"config-{number}.yaml", work / f"server-{number}.log"
            cfg.write_text(
                f"server:\n  bind: 127.0.0.1\n  port: {port}\n"
                "http:\n  enabled: false\nplugins:\n  metrics:\n    enabled: false\n"
                "sentinel:\n  enabled: false\nlogging:\n  level: error\n"
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
            first, port, log = launch(data)
            ready(first, port, log)
            check(query(port, ("SET", "owned", "durable")), b"OK")
            check(query(port, ("SAVE",)), b"OK")
            state = data / "replication.state"
            state.write_bytes(b"checkpoint-must-not-be-consumed-by-a-second-owner")
            protected = {path: path.read_bytes() for path in (data / "appendonly.aof", data / "dump.rdb", state)}
            paths = [data]
            alias = work / "alias"
            try:
                alias.symlink_to(data, target_is_directory=True)
                paths.append(alias)
            except OSError:
                pass
            for directory in paths:
                contender, _, contender_log = launch(directory)
                if contender.wait(timeout=15) == 0 or "persistent data directory is already in use" not in contender_log.read_text():
                    raise RuntimeError(f"second owner accepted: {contender_log.read_text()}")
                for path, source in protected.items():
                    check(path.read_bytes(), source)
                check(query(port, ("GET", "owned")), b"durable")
            state.unlink()
            first.terminate()
            check(first.wait(timeout=10), 0)
            if not (data / ".cachestorm.lock").exists():
                raise RuntimeError("release removed the shared lock inode")
            second, port, log = launch(data)
            ready(second, port, log)
            check(query(port, ("GET", "owned")), b"durable")
            second.kill()
            second.wait(timeout=10)
            third, port, log = launch(data)
            ready(third, port, log)
            check(query(port, ("GET", "owned")), b"durable")
            third.terminate()
            check(third.wait(timeout=10), 0)
            print("PASS: second process and symlink alias cannot read/alter owned persistence; graceful exit and SIGKILL release the lock; restart restores data")
        finally:
            for proc in processes:
                if proc.poll() is None:
                    proc.kill()
                proc.wait(timeout=10)


if __name__ == "__main__":
    main()
