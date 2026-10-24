"""Connection pool implementation."""

import socket
import threading
import queue
import time
from typing import Optional

from .exceptions import ConnectionError, TimeoutError
from .protocol import IncompleteResponse, encode_resp, decode_resp


class ConnectionPool:
    """Thread-safe connection pool."""

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6379,
        password: Optional[str] = None,
        db: int = 0,
        max_connections: int = 50,
        min_connections: int = 5,
        socket_timeout: float = 5.0,
        socket_connect_timeout: float = 5.0,
    ):
        self.host = host
        self.port = port
        self.password = password
        self.db = db
        self.max_connections = max_connections
        self.min_connections = min_connections
        self.socket_timeout = socket_timeout
        self.socket_connect_timeout = socket_connect_timeout

        self._pool = queue.Queue(maxsize=max_connections)
        self._lock = threading.Lock()
        self._created_connections = 0
        self._closed = False

        # Pre-populate pool
        for _ in range(min_connections):
            try:
                conn = self._try_create_connection()
            except Exception:
                break
            if conn is None:
                break
            self._pool.put(conn)

    def _create_connection(self):
        """Create a new connection and run AUTH/SELECT on it."""
        try:
            sock = socket.create_connection(
                (self.host, self.port),
                timeout=self.socket_connect_timeout,
            )
        except socket.error as e:
            raise ConnectionError(f"Failed to connect: {e}")

        try:
            sock.settimeout(self.socket_timeout)
            if self.password:
                self._handshake(sock, ["AUTH", self.password])
            if self.db != 0:
                self._handshake(sock, ["SELECT", self.db])
        except BaseException:
            sock.close()
            raise

        return sock

    def _handshake(self, sock, command):
        """Send one setup command and read its reply."""
        buffer = b""
        try:
            sock.sendall(encode_resp(command))
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    raise ConnectionError("Connection closed by server")
                buffer += chunk
                try:
                    response, consumed = decode_resp(buffer)
                except IncompleteResponse:
                    continue
                if consumed != len(buffer):
                    raise ConnectionError("Unexpected data after handshake reply")
                return response
        except socket.timeout:
            raise TimeoutError(f"{command[0]} timed out")
        except socket.error as e:
            raise ConnectionError(f"Socket error: {e}")

    def _try_create_connection(self):
        """Reserve a slot and create a connection, or return None if full."""
        with self._lock:
            if self._created_connections >= self.max_connections:
                return None
            self._created_connections += 1
        try:
            return self._create_connection()
        except BaseException:
            with self._lock:
                self._created_connections -= 1
            raise

    def _is_healthy(self, connection) -> bool:
        """Cheap liveness check: a non-blocking peek must find no data."""
        try:
            connection.setblocking(False)
            try:
                data = connection.recv(1, socket.MSG_PEEK)
            finally:
                connection.settimeout(self.socket_timeout)
        except BlockingIOError:
            return True
        except (socket.error, ValueError):
            return False
        return False

    def get_connection(self, timeout: Optional[float] = None):
        """Get a healthy connection from the pool."""
        if self._closed:
            raise ConnectionError("Pool is closed")

        wait = timeout or self.socket_timeout
        deadline = time.monotonic() + wait

        while True:
            try:
                conn = self._pool.get(block=False)
            except queue.Empty:
                break
            if self._is_healthy(conn):
                return conn
            self.discard_connection(conn)

        conn = self._try_create_connection()
        if conn is not None:
            return conn

        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("Could not get connection from pool")
            try:
                conn = self._pool.get(block=True, timeout=remaining)
            except queue.Empty:
                raise TimeoutError("Could not get connection from pool")
            if self._is_healthy(conn):
                return conn
            self.discard_connection(conn)
            conn = self._try_create_connection()
            if conn is not None:
                return conn

    def release_connection(self, connection):
        """Return a connection to the pool."""
        if self._closed:
            self.discard_connection(connection)
            return

        try:
            self._pool.put(connection, block=False)
        except queue.Full:
            self.discard_connection(connection)

    def discard_connection(self, connection):
        """Close a broken connection instead of returning it to the pool."""
        try:
            connection.close()
        except Exception:
            pass
        with self._lock:
            self._created_connections -= 1

    def close(self):
        """Close all connections in the pool."""
        self._closed = True

        while True:
            try:
                conn = self._pool.get(block=False)
            except queue.Empty:
                break
            self.discard_connection(conn)

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
