"""CacheStorm client implementation."""

import socket
import threading
from typing import Any, Dict, List, Optional, Union, Callable
from contextlib import contextmanager

from .exceptions import ConnectionError, TimeoutError, ResponseError
from .pool import ConnectionPool
from .pipeline import Pipeline
from .pubsub import PubSub
from .protocol import encode_resp, decode_resp


class CacheStormClient:
    """Synchronous CacheStorm client."""

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6379,
        password: Optional[str] = None,
        db: int = 0,
        socket_timeout: float = 5.0,
        socket_connect_timeout: float = 5.0,
        connection_pool: Optional[ConnectionPool] = None,
    ):
        self.host = host
        self.port = port
        self.password = password
        self.db = db
        self.socket_timeout = socket_timeout
        self.socket_connect_timeout = socket_connect_timeout
        self._sock: Optional[socket.socket] = None
        self._lock = threading.Lock()

        if connection_pool:
            self.connection_pool = connection_pool
        else:
            self.connection_pool = ConnectionPool(
                host=host,
                port=port,
                password=password,
                db=db,
                max_connections=10,
            )

    def connect(self) -> None:
        """Connect to the server."""
        if self._sock is None:
            try:
                self._sock = socket.create_connection(
                    (self.host, self.port),
                    timeout=self.socket_connect_timeout,
                )
                self._sock.settimeout(self.socket_timeout)

                if self.password:
                    self.execute_command("AUTH", self.password)

                if self.db != 0:
                    self.execute_command("SELECT", self.db)

            except socket.error as e:
                raise ConnectionError(f"Failed to connect: {e}")

    def close(self) -> None:
        """Close the connection."""
        if self._sock:
            try:
                self._sock.close()
            except socket.error:
                pass
            finally:
                self._sock = None

    def __enter__(self):
        self.connect()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    def execute_command(self, *args) -> Any:
        """Execute a command."""
        self.connect()

        with self._lock:
            try:
                # Send command
                data = encode_resp(list(args))
                self._sock.sendall(data)

                # Receive response
                response = self._read_response()
                return response

            except socket.timeout:
                raise TimeoutError("Command timed out")
            except socket.error as e:
                self.close()
                raise ConnectionError(f"Socket error: {e}")

    def _read_response(self) -> Any:
        """Read response from server."""
        buffer = b""
        while True:
            try:
                chunk = self._sock.recv(4096)
                if not chunk:
                    break
                buffer += chunk

                # Try to decode
                try:
                    response, _ = decode_resp(buffer)
                    return response
                except ValueError:
                    # Need more data
                    continue

            except socket.error:
                break

        return None

    # String commands
    def set(
        self,
        name: str,
        value: Union[str, bytes, int],
        ex: Optional[int] = None,
        px: Optional[int] = None,
        nx: bool = False,
        xx: bool = False,
        tags: Optional[List[str]] = None,
    ) -> bool:
        """Set key to value."""
        args = ["SET", name, value]

        if ex is not None:
            args.extend(["EX", ex])
        if px is not None:
            args.extend(["PX", px])
        if nx:
            args.append("NX")
        if xx:
            args.append("XX")
        if tags:
            args.append("TAGS")
            args.extend(tags)

        return self.execute_command(*args) == "OK"

    def set_with_tags(
        self,
        name: str,
        value: Union[str, bytes, int],
        tags: List[str],
    ) -> bool:
        """Set key with tags."""
        return self.set(name, value, tags=tags)

    def get(self, name: str) -> Optional[bytes]:
        """Get value of key."""
        return self.execute_command("GET", name)

    def delete(self, *names: str) -> int:
        """Delete one or more keys."""
        return self.execute_command("DEL", *names)

    def exists(self, *names: str) -> int:
        """Check if keys exist."""
        return self.execute_command("EXISTS", *names)

    def expire(self, name: str, time: int) -> bool:
        """Set expiration on key."""
        return self.execute_command("EXPIRE", name, time) == 1

    def ttl(self, name: str) -> int:
        """Get time to live of key."""
        return self.execute_command("TTL", name)

    # Hash commands
    def hset(self, name: str, key: str, value: Union[str, bytes, int]) -> int:
        """Set hash field to value."""
        return self.execute_command("HSET", name, key, value)

    def hget(self, name: str, key: str) -> Optional[bytes]:
        """Get hash field value."""
        return self.execute_command("HGET", name, key)

    def hgetall(self, name: str) -> Dict[bytes, bytes]:
        """Get all hash fields and values."""
        result = self.execute_command("HGETALL", name)
        if result and len(result) % 2 == 0:
            return {result[i]: result[i + 1] for i in range(0, len(result), 2)}
        return {}

    def hdel(self, name: str, *keys: str) -> int:
        """Delete hash fields."""
        return self.execute_command("HDEL", name, *keys)

    # List commands
    def lpush(self, name: str, *values: Union[str, bytes, int]) -> int:
        """Push values to left of list."""
        return self.execute_command("LPUSH", name, *values)

    def rpush(self, name: str, *values: Union[str, bytes, int]) -> int:
        """Push values to right of list."""
        return self.execute_command("RPUSH", name, *values)

    def lpop(self, name: str) -> Optional[bytes]:
        """Pop value from left of list."""
        return self.execute_command("LPOP", name)

    def rpop(self, name: str) -> Optional[bytes]:
        """Pop value from right of list."""
        return self.execute_command("RPOP", name)

    def lrange(self, name: str, start: int, end: int) -> List[bytes]:
        """Get list range."""
        return self.execute_command("LRANGE", name, start, end)

    # Set commands
    def sadd(self, name: str, *values: Union[str, bytes, int]) -> int:
        """Add values to set."""
        return self.execute_command("SADD", name, *values)

    def srem(self, name: str, *values: Union[str, bytes, int]) -> int:
        """Remove values from set."""
        return self.execute_command("SREM", name, *values)

    def smembers(self, name: str) -> List[bytes]:
        """Get all set members."""
        return self.execute_command("SMEMBERS", name)

    def sismember(self, name: str, value: Union[str, bytes, int]) -> bool:
        """Check if value is in set."""
        return self.execute_command("SISMEMBER", name, value) == 1

    # Sorted set commands
    def zadd(self, name: str, mapping: Dict[Union[str, bytes], float]) -> int:
        """Add to sorted set."""
        args = ["ZADD", name]
        for member, score in mapping.items():
            args.extend([score, member])
        return self.execute_command(*args)

    def zrange(
        self, name: str, start: int, end: int, withscores: bool = False
    ) -> List[bytes]:
        """Get sorted set range."""
        args = ["ZRANGE", name, start, end]
        if withscores:
            args.append("WITHSCORES")
        return self.execute_command(*args)

    def zrem(self, name: str, *values: Union[str, bytes]) -> int:
        """Remove from sorted set."""
        return self.execute_command("ZREM", name, *values)

    # CacheStorm-specific commands
    def invalidate(self, tag: str) -> int:
        """Invalidate keys by tag."""
        return self.execute_command("INVALIDATE", tag)

    def tag_keys(self, tag: str) -> List[str]:
        """Get keys by tag."""
        return self.execute_command("TAGKEYS", tag)

    def tags(self, key: str) -> List[str]:
        """Get tags of key."""
        return self.execute_command("TAGS", key)

    # Pub/Sub
    def pubsub(self) -> PubSub:
        """Create pub/sub instance."""
        return PubSub(self.connection_pool)

    def publish(self, channel: str, message: Union[str, bytes]) -> int:
        """Publish message to channel."""
        return self.execute_command("PUBLISH", channel, message)

    # Pipeline
    @contextmanager
    def pipeline(self):
        """Create pipeline context manager."""
        pipe = Pipeline(self.connection_pool)
        try:
            yield pipe
        finally:
            pipe.reset()

    def pipeline_execute(self, commands: List[List]) -> List[Any]:
        """Execute pipeline."""
        with self.pipeline() as pipe:
            for cmd in commands:
                pipe.execute_command(*cmd)
            return pipe.execute()


class AsyncCacheStormClient:
    """Asynchronous CacheStorm client."""

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6379,
        password: Optional[str] = None,
        db: int = 0,
        socket_timeout: float = 5.0,
    ):
        self.host = host
        self.port = port
        self.password = password
        self.db = db
        self.socket_timeout = socket_timeout
        self._reader = None
        self._writer = None

    async def connect(self) -> None:
        """Connect to the server."""
        import asyncio

        try:
            self._reader, self._writer = await asyncio.wait_for(
                asyncio.open_connection(self.host, self.port),
                timeout=self.socket_connect_timeout,
            )

            if self.password:
                await self.execute_command("AUTH", self.password)

            if self.db != 0:
                await self.execute_command("SELECT", self.db)

        except asyncio.TimeoutError:
            raise TimeoutError("Connection timed out")
        except Exception as e:
            raise ConnectionError(f"Failed to connect: {e}")

    async def close(self) -> None:
        """Close the connection."""
        if self._writer:
            self._writer.close()
            await self._writer.wait_closed()

    async def execute_command(self, *args) -> Any:
        """Execute a command."""
        import asyncio

        if not self._writer:
            await self.connect()

        try:
            # Send command
            data = encode_resp(list(args))
            self._writer.write(data)
            await self._writer.drain()

            # Receive response
            response = await asyncio.wait_for(
                self._read_response(),
                timeout=self.socket_timeout,
            )
            return response

        except asyncio.TimeoutError:
            raise TimeoutError("Command timed out")

    async def _read_response(self) -> Any:
        """Read response from server."""
        buffer = b""
        while True:
            try:
                chunk = await self._reader.read(4096)
                if not chunk:
                    break
                buffer += chunk

                try:
                    response, consumed = decode_resp(buffer)
                    return response
                except ValueError:
                    continue

            except Exception:
                break

        return None

    # Async versions of commands
    async def set(self, name: str, value: Union[str, bytes, int], **kwargs) -> bool:
        """Set key to value."""
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

        return await self.execute_command(*args) == "OK"

    async def get(self, name: str) -> Optional[bytes]:
        """Get value of key."""
        return await self.execute_command("GET", name)

    async def delete(self, *names: str) -> int:
        """Delete keys."""
        return await self.execute_command("DEL", *names)
