import os
import socket
import sys
import time
import unittest
from unittest.mock import patch

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from cachestorm import CacheStormClient
from cachestorm.exceptions import ConnectionError, ResponseError, TimeoutError
from cachestorm.pool import ConnectionPool
from cachestorm.pubsub import PubSub
from tests.test_pipeline_pubsub import FakePool


class PoolSocket:
    def __init__(self, replies=(), peek=None):
        self.replies = list(replies)
        self.peek = peek
        self.sent = b""
        self.closed = False
        self.blocking = True
        self.timeout = None
        self.fail_send = False

    def settimeout(self, timeout):
        self.timeout = timeout
        self.blocking = True

    def setblocking(self, flag):
        self.blocking = flag

    def sendall(self, data):
        if self.fail_send:
            raise BrokenPipeError("broken pipe")
        self.sent += data

    def recv(self, size, flags=0):
        if self.closed:
            raise OSError("closed")
        if flags & socket.MSG_PEEK:
            assert not self.blocking
            if isinstance(self.peek, BaseException):
                raise self.peek
            if self.peek is None:
                raise BlockingIOError()
            return self.peek[:size]
        if not self.replies:
            return b""
        return self.replies.pop(0)

    def close(self):
        self.closed = True


class PoolTest(unittest.TestCase):
    def patch_sockets(self, *socks):
        created = list(socks)
        patcher = patch("socket.create_connection", side_effect=lambda *a, **k: created.pop(0))
        mock = patcher.start()
        self.addCleanup(patcher.stop)
        return mock

    def test_new_connection_runs_auth_and_select(self):
        sock = PoolSocket([b"+OK\r\n", b"+O", b"K\r\n"])
        self.patch_sockets(sock)
        pool = ConnectionPool(password="pw", db=3, min_connections=0, socket_timeout=2.0)
        self.assertIs(pool.get_connection(), sock)
        self.assertEqual(
            sock.sent,
            b"*2\r\n$4\r\nAUTH\r\n$2\r\npw\r\n*2\r\n$6\r\nSELECT\r\n$1\r\n3\r\n",
        )
        self.assertEqual(sock.timeout, 2.0)

    def test_no_handshake_without_password_or_db(self):
        sock = PoolSocket()
        self.patch_sockets(sock)
        pool = ConnectionPool(min_connections=0)
        self.assertIs(pool.get_connection(), sock)
        self.assertEqual(sock.sent, b"")

    def test_auth_failure_raises_and_frees_slot(self):
        sock = PoolSocket([b"-WRONGPASS invalid\r\n"])
        self.patch_sockets(sock)
        pool = ConnectionPool(password="bad", min_connections=0)
        with self.assertRaises(ResponseError):
            pool.get_connection()
        self.assertTrue(sock.closed)
        self.assertEqual(pool._created_connections, 0)

    def test_eof_during_handshake_raises_connection_error(self):
        sock = PoolSocket([])
        self.patch_sockets(sock)
        pool = ConnectionPool(db=1, min_connections=0)
        with self.assertRaises(ConnectionError):
            pool.get_connection()
        self.assertTrue(sock.closed)

    def test_connect_failure_raises_connection_error(self):
        patcher = patch("socket.create_connection", side_effect=OSError("refused"))
        patcher.start()
        self.addCleanup(patcher.stop)
        pool = ConnectionPool(min_connections=0)
        with self.assertRaises(ConnectionError):
            pool.get_connection()
        self.assertEqual(pool._created_connections, 0)

    def test_prepopulation_failure_is_not_raised(self):
        patcher = patch("socket.create_connection", side_effect=OSError("refused"))
        patcher.start()
        self.addCleanup(patcher.stop)
        pool = ConnectionPool(min_connections=3)
        self.assertEqual(pool._created_connections, 0)

    def test_healthy_connection_is_reused(self):
        sock = PoolSocket()
        self.patch_sockets(sock)
        pool = ConnectionPool(min_connections=0, socket_timeout=4.0)
        conn = pool.get_connection()
        pool.release_connection(conn)
        self.assertIs(pool.get_connection(), sock)
        self.assertFalse(sock.closed)
        self.assertTrue(sock.blocking)
        self.assertEqual(sock.timeout, 4.0)
        self.assertEqual(pool._created_connections, 1)

    def test_dead_connections_discarded_on_checkout(self):
        cases = [b"", b"+stale\r\n", ConnectionResetError("reset")]
        for peek in cases:
            with self.subTest(peek=peek):
                dead, fresh = PoolSocket(), PoolSocket()
                create = patch("socket.create_connection", side_effect=[dead, fresh])
                create.start()
                try:
                    pool = ConnectionPool(min_connections=0)
                    pool.release_connection(pool.get_connection())
                    dead.peek = peek
                    self.assertIs(pool.get_connection(), fresh)
                    self.assertTrue(dead.closed)
                    self.assertEqual(pool._created_connections, 1)
                finally:
                    create.stop()

    def test_empty_pool_under_limit_does_not_wait(self):
        self.patch_sockets(PoolSocket())
        pool = ConnectionPool(min_connections=0, socket_timeout=5.0)
        start = time.monotonic()
        pool.get_connection()
        self.assertLess(time.monotonic() - start, 1.0)

    def test_exhausted_pool_times_out(self):
        self.patch_sockets(PoolSocket())
        pool = ConnectionPool(min_connections=0, max_connections=1)
        pool.get_connection()
        with self.assertRaises(TimeoutError):
            pool.get_connection(timeout=0.05)

    def test_client_passes_settings_to_pool_lazily(self):
        mock = self.patch_sockets()
        client = CacheStormClient(
            password="pw", db=2, socket_timeout=1.0, socket_connect_timeout=2.0
        )
        mock.assert_not_called()
        pool = client.connection_pool
        self.assertEqual(
            (pool.password, pool.db, pool.socket_timeout, pool.socket_connect_timeout),
            ("pw", 2, 1.0, 2.0),
        )

    def test_pipeline_through_auth_pool(self):
        sock = PoolSocket([b"+OK\r\n", b"+OK\r\n$1\r\nv\r\n"])
        self.patch_sockets(sock)
        client = CacheStormClient(password="pw")
        with client.pipeline() as pipe:
            pipe.set("k", "v").get("k")
            self.assertEqual(pipe.execute(), ["OK", b"v"])
        self.assertTrue(sock.sent.startswith(b"*2\r\n$4\r\nAUTH\r\n$2\r\npw\r\n"))


class PubSubSendTest(unittest.TestCase):
    def test_subscribe_send_failure_raises(self):
        sock = PoolSocket()
        pubsub = PubSub(FakePool(sock))
        pubsub._conn = sock
        sock.fail_send = True
        with self.assertRaises(ConnectionError):
            pubsub.subscribe("news")

    def test_initial_subscribe_failure_raises_and_releases(self):
        sock = PoolSocket()
        sock.fail_send = True
        pool = FakePool(sock)
        pubsub = PubSub(pool).subscribe("news")
        with self.assertRaises(ConnectionError):
            pubsub.get_message(timeout=0.01)
        self.assertFalse(pubsub._running)
        self.assertIsNone(pubsub._conn)
        self.assertEqual(pool.discarded, [sock])

    def test_close_tolerates_dead_connection(self):
        sock = PoolSocket()
        pool = FakePool(sock)
        pubsub = PubSub(pool).subscribe("news")
        pubsub._conn = sock
        sock.fail_send = True
        pubsub.close()
        self.assertIsNone(pubsub._conn)
        self.assertEqual(pool.discarded, [sock])


if __name__ == "__main__":
    unittest.main()
