"""Pipeline implementation."""

import socket
from typing import List, Any, Optional

from .exceptions import ConnectionError, TimeoutError
from .protocol import encode_resp, decode_resp


class Pipeline:
    """Command pipeline for batch execution."""

    def __init__(self, connection_pool):
        self.connection_pool = connection_pool
        self.commands: List[List[Any]] = []
        self._sock: Optional[socket.socket] = None

    def execute_command(self, *args):
        """Add a command to the pipeline."""
        self.commands.append(list(args))
        return self

    def set(self, name: str, value, **kwargs):
        """Add SET command."""
        args = ["SET", name, value]
        if "ex" in kwargs:
            args.extend(["EX", kwargs["ex"]])
        if "px" in kwargs:
            args.extend(["PX", kwargs["px"]])
        if kwargs.get("nx"):
            args.append("NX")
        if kwargs.get("xx"):
            args.append("XX")
        if "tags" in kwargs:
            args.append("TAGS")
            args.extend(kwargs["tags"])
        self.commands.append(args)
        return self

    def get(self, name: str):
        """Add GET command."""
        self.commands.append(["GET", name])
        return self

    def delete(self, *names: str):
        """Add DEL command."""
        self.commands.append(["DEL"] + list(names))
        return self

    def hset(self, name: str, key: str, value):
        """Add HSET command."""
        self.commands.append(["HSET", name, key, value])
        return self

    def hget(self, name: str, key: str):
        """Add HGET command."""
        self.commands.append(["HGET", name, key])
        return self

    def hgetall(self, name: str):
        """Add HGETALL command."""
        self.commands.append(["HGETALL", name])
        return self

    def lpush(self, name: str, *values):
        """Add LPUSH command."""
        self.commands.append(["LPUSH", name] + list(values))
        return self

    def rpush(self, name: str, *values):
        """Add RPUSH command."""
        self.commands.append(["RPUSH", name] + list(values))
        return self

    def lpop(self, name: str):
        """Add LPOP command."""
        self.commands.append(["LPOP", name])
        return self

    def rpop(self, name: str):
        """Add RPOP command."""
        self.commands.append(["RPOP", name])
        return self

    def lrange(self, name: str, start: int, end: int):
        """Add LRANGE command."""
        self.commands.append(["LRANGE", name, start, end])
        return self

    def sadd(self, name: str, *values):
        """Add SADD command."""
        self.commands.append(["SADD", name] + list(values))
        return self

    def srem(self, name: str, *values):
        """Add SREM command."""
        self.commands.append(["SREM", name] + list(values))
        return self

    def smembers(self, name: str):
        """Add SMEMBERS command."""
        self.commands.append(["SMEMBERS", name])
        return self

    def zadd(self, name: str, mapping: dict):
        """Add ZADD command."""
        args = ["ZADD", name]
        for member, score in mapping.items():
            args.extend([score, member])
        self.commands.append(args)
        return self

    def zrange(self, name: str, start: int, end: int, withscores: bool = False):
        """Add ZRANGE command."""
        args = ["ZRANGE", name, start, end]
        if withscores:
            args.append("WITHSCORES")
        self.commands.append(args)
        return self

    def expire(self, name: str, time: int):
        """Add EXPIRE command."""
        self.commands.append(["EXPIRE", name, time])
        return self

    def ttl(self, name: str):
        """Add TTL command."""
        self.commands.append(["TTL", name])
        return self

    def exists(self, *names: str):
        """Add EXISTS command."""
        self.commands.append(["EXISTS"] + list(names))
        return self

    def execute(self) -> List[Any]:
        """Execute all commands in the pipeline."""
        if not self.commands:
            return []

        # Get connection from pool
        conn = self.connection_pool.get_connection()

        try:
            # Send all commands
            for cmd in self.commands:
                data = encode_resp(cmd)
                conn.sendall(data)

            # Read all responses
            results = []
            buffer = b""

            for _ in self.commands:
                while True:
                    try:
                        chunk = conn.recv(4096)
                        if not chunk:
                            break
                        buffer += chunk

                        try:
                            response, consumed = decode_resp(buffer)
                            results.append(response)
                            buffer = buffer[consumed:]
                            break
                        except ValueError:
                            # Need more data
                            continue
                    except socket.error:
                        break

            # Pad results if needed
            while len(results) < len(self.commands):
                results.append(None)

            return results

        except socket.timeout:
            raise TimeoutError("Pipeline execution timed out")
        except socket.error as e:
            raise ConnectionError(f"Socket error: {e}")
        finally:
            self.connection_pool.release_connection(conn)

    def reset(self):
        """Clear the pipeline."""
        self.commands = []
