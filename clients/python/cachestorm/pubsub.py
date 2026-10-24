"""Pub/Sub implementation."""

import socket
import threading
import queue
from typing import Callable, Optional, Dict, Set, Any

from .exceptions import ConnectionError, InvalidResponse


def _to_str(value: Any) -> Any:
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return value
from .protocol import IncompleteResponse, encode_resp, decode_resp


class PubSub:
    """Pub/Sub client."""

    def __init__(self, connection_pool):
        self.connection_pool = connection_pool
        self.channels: Set[str] = set()
        self.patterns: Set[str] = set()
        self._running = False
        self._thread: Optional[threading.Thread] = None
        self._callbacks: Dict[str, Callable] = {}
        self._pattern_callbacks: Dict[str, Callable] = {}
        self._message_queue: queue.Queue = queue.Queue()
        self._conn: Optional[socket.socket] = None
        self._lock = threading.Lock()

    def subscribe(self, *channels: str, **kwargs):
        """Subscribe to channels."""
        handler = kwargs.get("handler")

        for channel in channels:
            self.channels.add(channel)
            if handler:
                self._callbacks[channel] = handler

        # Send SUBSCRIBE command
        if self._conn:
            self._send_command(["SUBSCRIBE"] + list(channels))

        return self

    def psubscribe(self, *patterns: str, **kwargs):
        """Subscribe to patterns."""
        handler = kwargs.get("handler")

        for pattern in patterns:
            self.patterns.add(pattern)
            if handler:
                self._pattern_callbacks[pattern] = handler

        # Send PSUBSCRIBE command
        if self._conn:
            self._send_command(["PSUBSCRIBE"] + list(patterns))

        return self

    def unsubscribe(self, *channels: str):
        """Unsubscribe from channels."""
        if not channels:
            # Unsubscribe from all
            channels = tuple(self.channels)

        for channel in channels:
            self.channels.discard(channel)
            self._callbacks.pop(channel, None)

        if self._conn and channels:
            self._send_command(["UNSUBSCRIBE"] + list(channels))

        return self

    def punsubscribe(self, *patterns: str):
        """Unsubscribe from patterns."""
        if not patterns:
            # Unsubscribe from all
            patterns = tuple(self.patterns)

        for pattern in patterns:
            self.patterns.discard(pattern)
            self._pattern_callbacks.pop(pattern, None)

        if self._conn and patterns:
            self._send_command(["PUNSUBSCRIBE"] + list(patterns))

        return self

    def _send_command(self, cmd: list):
        """Send command to server."""
        if self._conn:
            data = encode_resp(cmd)
            try:
                self._conn.sendall(data)
            except socket.error as e:
                raise ConnectionError(f"Failed to send {cmd[0]}: {e}")

    def listen(self, timeout: Optional[float] = None):
        """Listen for messages (blocking generator)."""
        import time

        if not self._running:
            self._start_listener()

        start_time = time.time()

        while self._running or not self._message_queue.empty():
            try:
                # Use timeout to allow checking _running periodically
                wait_time = 0.1 if timeout is None else max(0.1, timeout / 10)
                message = self._message_queue.get(timeout=wait_time)

                # Call handler if registered
                if message:
                    msg_type = message.get("type")
                    channel = message.get("channel", "")

                    if msg_type == "message" and channel in self._callbacks:
                        self._callbacks[channel](message)
                    elif msg_type == "pmessage":
                        pattern = message.get("pattern", "")
                        if pattern in self._pattern_callbacks:
                            self._pattern_callbacks[pattern](message)

                yield message

                # Check timeout
                if timeout and (time.time() - start_time) >= timeout:
                    break

            except queue.Empty:
                if timeout and (time.time() - start_time) >= timeout:
                    break
                continue

    def get_message(self, timeout: Optional[float] = None) -> Optional[Dict[str, Any]]:
        """Get a message (non-blocking)."""
        if not self._running:
            self._start_listener()

        try:
            return self._message_queue.get(timeout=timeout)
        except queue.Empty:
            return None

    def _start_listener(self):
        """Start the listener thread."""
        with self._lock:
            if self._running:
                return

            self._running = True

            # Get dedicated connection
            self._conn = self.connection_pool.get_connection()

            try:
                if self.channels:
                    self._send_command(["SUBSCRIBE"] + list(self.channels))
                if self.patterns:
                    self._send_command(["PSUBSCRIBE"] + list(self.patterns))
            except BaseException:
                self._running = False
                self._drop_connection()
                raise

            # Start listener thread
            self._thread = threading.Thread(target=self._listener_loop, daemon=True)
            self._thread.start()

    def _listener_loop(self):
        """Background listener loop."""
        buffer = b""

        while self._running:
            try:
                chunk = self._conn.recv(4096)
                if not chunk:
                    break

                buffer += chunk

                # Try to parse messages
                while buffer:
                    try:
                        response, consumed = decode_resp(buffer, raise_errors=False)
                    except IncompleteResponse:
                        break
                    except InvalidResponse:
                        self._running = False
                        break
                    buffer = buffer[consumed:]

                    message = self._parse_message(response)
                    if message:
                        self._message_queue.put(message)

            except socket.timeout:
                continue
            except socket.error:
                break

        self._running = False

    def _parse_message(self, response) -> Optional[Dict[str, Any]]:
        """Parse a pub/sub message from response."""
        if not isinstance(response, list) or len(response) < 3:
            return None

        response = [_to_str(item) for item in response[:-1]] + [response[-1]]
        msg_type = response[0]

        if msg_type == "message":
            return {
                "type": "message",
                "channel": response[1],
                "data": response[2],
            }
        elif msg_type == "pmessage" and len(response) >= 4:
            return {
                "type": "pmessage",
                "pattern": response[1],
                "channel": response[2],
                "data": response[3],
            }
        elif msg_type == "subscribe":
            return {
                "type": "subscribe",
                "channel": response[1],
                "subscribed_count": response[2],
            }
        elif msg_type == "psubscribe":
            return {
                "type": "psubscribe",
                "pattern": response[1],
                "subscribed_count": response[2],
            }
        elif msg_type == "unsubscribe":
            return {
                "type": "unsubscribe",
                "channel": response[1],
                "subscribed_count": response[2],
            }
        elif msg_type == "punsubscribe":
            return {
                "type": "punsubscribe",
                "pattern": response[1],
                "subscribed_count": response[2],
            }

        return None

    def close(self):
        """Close pub/sub connection."""
        self._running = False

        if self._thread:
            self._thread.join(timeout=1.0)

        try:
            self.unsubscribe()
            self.punsubscribe()
        except ConnectionError:
            pass

        self._drop_connection()

    def _drop_connection(self):
        """Close the dedicated connection and give its pool slot back."""
        conn, self._conn = self._conn, None
        if conn is None:
            return
        discard = getattr(self.connection_pool, "discard_connection", None)
        if discard is not None:
            discard(conn)
            return
        try:
            conn.close()
        except Exception:
            pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()
