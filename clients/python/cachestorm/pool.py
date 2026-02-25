"""Connection pool implementation."""

import threading
import queue
from typing import Optional

from .exceptions import ConnectionError, TimeoutError


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
                conn = self._create_connection()
                self._pool.put(conn)
            except Exception:
                break

    def _create_connection(self):
        """Create a new connection."""
        import socket

        sock = socket.create_connection(
            (self.host, self.port),
            timeout=self.socket_connect_timeout,
        )
        sock.settimeout(self.socket_timeout)

        self._created_connections += 1
        return sock

    def get_connection(self, timeout: Optional[float] = None):
        """Get a connection from the pool."""
        if self._closed:
            raise ConnectionError("Pool is closed")

        try:
            return self._pool.get(block=True, timeout=timeout or self.socket_timeout)
        except queue.Empty:
            # Create new connection if under limit
            with self._lock:
                if self._created_connections < self.max_connections:
                    return self._create_connection()
            raise TimeoutError("Could not get connection from pool")

    def release_connection(self, connection):
        """Return a connection to the pool."""
        if self._closed:
            connection.close()
            return

        try:
            self._pool.put(connection, block=False)
        except queue.Full:
            connection.close()
            with self._lock:
                self._created_connections -= 1

    def close(self):
        """Close all connections in the pool."""
        self._closed = True

        while not self._pool.empty():
            try:
                conn = self._pool.get(block=False)
                conn.close()
            except Exception:
                pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
