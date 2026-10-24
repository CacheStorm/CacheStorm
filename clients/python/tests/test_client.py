import asyncio
import os
import sys
import unittest
from unittest.mock import patch

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from cachestorm import AsyncCacheStormClient, CacheStormClient
from cachestorm.exceptions import ConnectionError, DataError, ResponseError


class FakeSocket:
    def __init__(self, chunks):
        self.chunks = list(chunks)
        self.sent = b""
        self.timeout = None
        self.closed = False

    def settimeout(self, timeout):
        self.timeout = timeout

    def sendall(self, data):
        self.sent += data

    def recv(self, size):
        if not self.chunks:
            return b""
        return self.chunks.pop(0)

    def close(self):
        self.closed = True


class FakeReader:
    def __init__(self, chunks):
        self.chunks = list(chunks)

    async def read(self, size):
        if not self.chunks:
            return b""
        chunk = self.chunks.pop(0)
        if isinstance(chunk, BaseException):
            raise chunk
        return chunk


class FakeWriter:
    def __init__(self):
        self.written = b""
        self.closed = False

    def write(self, data):
        self.written += data

    async def drain(self):
        pass

    def close(self):
        self.closed = True

    async def wait_closed(self):
        pass


class SyncClientTest(unittest.TestCase):
    def make_client(self, chunks, **kwargs):
        sock = FakeSocket(chunks)
        client = CacheStormClient(**kwargs)
        patcher = patch("socket.create_connection", return_value=sock)
        self.create_connection = patcher.start()
        self.addCleanup(patcher.stop)
        return client, sock

    def test_connect_uses_timeouts(self):
        client, sock = self.make_client([], socket_timeout=2.0, socket_connect_timeout=3.0)
        client.connect()
        self.create_connection.assert_called_once_with(("localhost", 6379), timeout=3.0)
        self.assertEqual(sock.timeout, 2.0)

    def test_set_sends_utf8_and_parses_ok(self):
        client, sock = self.make_client([b"+OK\r\n"])
        self.assertTrue(client.set("k", "ş"))
        self.assertEqual(sock.sent, b"*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$2\r\n\xc5\x9f\r\n")

    def test_get_binary_fragmented_reply(self):
        client, sock = self.make_client([b"$", b"2\r", b"\n\xff", b"\x00\r", b"\n"])
        self.assertEqual(client.get("k"), b"\xff\x00")

    def test_error_reply_raises(self):
        client, _ = self.make_client([b"-ERR wrong\r\n"])
        with self.assertRaises(ResponseError):
            client.get("k")

    def test_eof_raises_connection_error_and_resets(self):
        client, sock = self.make_client([b"$5\r\nhel"])
        with self.assertRaises(ConnectionError):
            client.get("k")
        self.assertTrue(sock.closed)
        self.assertIsNone(client._sock)
        self.assertEqual(client._buffer, b"")

    def test_eof_before_any_reply_raises_connection_error(self):
        client, _ = self.make_client([])
        with self.assertRaises(ConnectionError):
            client.get("k")

    def test_leftover_bytes_kept_for_next_reply(self):
        client, _ = self.make_client([b"$1\r\na\r\n$1\r", b"\nb\r\n:3\r\n"])
        self.assertEqual(client.get("x"), b"a")
        self.assertEqual(client.get("y"), b"b")
        self.assertEqual(client.ttl("z"), 3)

    def test_error_reply_does_not_poison_next_reply(self):
        client, _ = self.make_client([b"-ERR bad\r\n+OK\r\n"])
        with self.assertRaises(ResponseError):
            client.get("k")
        self.assertTrue(client.set("k", "v"))

    def test_none_argument_raises_before_sending(self):
        client, sock = self.make_client([])
        with self.assertRaises(DataError):
            client.set("k", None)
        self.assertEqual(sock.sent, b"")
        self.create_connection.assert_not_called()

    def test_auth_failure_resets_socket(self):
        client, sock = self.make_client([b"-WRONGPASS invalid\r\n"], password="bad")
        with self.assertRaises(ResponseError):
            client.connect()
        self.assertTrue(sock.closed)
        self.assertIsNone(client._sock)

    def test_select_failure_resets_socket(self):
        client, sock = self.make_client([b"-ERR DB index is out of range\r\n"], db=99)
        with self.assertRaises(ResponseError):
            client.connect()
        self.assertTrue(sock.closed)
        self.assertIsNone(client._sock)


class AsyncClientTest(unittest.TestCase):
    def run_async(self, coro):
        return asyncio.run(coro)

    def patch_open(self, reader, writer, calls):
        async def fake_open_connection(host, port):
            calls.append((host, port))
            return reader, writer

        patcher = patch.object(asyncio, "open_connection", fake_open_connection)
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_construction_defaults(self):
        client = AsyncCacheStormClient()
        self.assertEqual(client.socket_timeout, 5.0)
        self.assertEqual(client.socket_connect_timeout, 5.0)
        custom = AsyncCacheStormClient(socket_connect_timeout=1.5)
        self.assertEqual(custom.socket_connect_timeout, 1.5)

    def test_connect_via_fake(self):
        calls = []
        reader, writer = FakeReader([]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            client = AsyncCacheStormClient(host="example", port=7000)
            await client.connect()
            return client

        client = self.run_async(scenario())
        self.assertEqual(calls, [("example", 7000)])
        self.assertIs(client._writer, writer)

    def test_connect_authenticates_and_selects(self):
        calls = []
        reader = FakeReader([b"+OK\r\n", b"+OK\r\n"])
        writer = FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            client = AsyncCacheStormClient(password="pw", db=2)
            await client.connect()

        self.run_async(scenario())
        self.assertEqual(
            writer.written,
            b"*2\r\n$4\r\nAUTH\r\n$2\r\npw\r\n*2\r\n$6\r\nSELECT\r\n$1\r\n2\r\n",
        )

    def test_set_get_roundtrip_with_fragments(self):
        calls = []
        reader = FakeReader([b"+O", b"K\r\n", b"$4\r\n\xf0\x9f", b"\x9a\x80\r\n"])
        writer = FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            client = AsyncCacheStormClient()
            ok = await client.set("k", "🚀")
            value = await client.get("k")
            await client.close()
            return client, ok, value

        client, ok, value = self.run_async(scenario())
        self.assertTrue(ok)
        self.assertEqual(value, "🚀".encode("utf-8"))
        self.assertTrue(writer.closed)
        self.assertIsNone(client._writer)
        self.assertIn(b"$4\r\n\xf0\x9f\x9a\x80\r\n", writer.written)

    def test_error_reply_raises(self):
        calls = []
        reader, writer = FakeReader([b"-WRONGTYPE nope\r\n"]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            client = AsyncCacheStormClient()
            await client.get("k")

        with self.assertRaises(ResponseError):
            self.run_async(scenario())

    def test_auth_error_is_not_masked_as_connection_error(self):
        calls = []
        reader, writer = FakeReader([b"-WRONGPASS invalid\r\n"]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            await AsyncCacheStormClient(password="bad").connect()

        with self.assertRaises(ResponseError):
            self.run_async(scenario())

    def test_reader_failure_raises_connection_error(self):
        calls = []
        reader, writer = FakeReader([ConnectionResetError("reset")]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            await AsyncCacheStormClient().get("k")

        with self.assertRaises(ConnectionError):
            self.run_async(scenario())

    def test_eof_raises_connection_error_and_resets(self):
        calls = []
        reader, writer = FakeReader([b"$3\r\nab"]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def run():
            client = AsyncCacheStormClient()
            with self.assertRaises(ConnectionError):
                await client.get("k")
            return client

        client = self.run_async(run())
        self.assertIsNone(client._writer)
        self.assertTrue(writer.closed)

    def test_leftover_bytes_kept_for_next_reply(self):
        calls = []
        reader, writer = FakeReader([b"$1\r\na\r\n$1\r\nb", b"\r\n"]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def scenario():
            client = AsyncCacheStormClient()
            return await client.get("x"), await client.get("y")

        self.assertEqual(self.run_async(scenario()), (b"a", b"b"))

    def test_none_argument_raises_before_connecting(self):
        calls = []
        self.patch_open(FakeReader([]), FakeWriter(), calls)

        async def scenario():
            await AsyncCacheStormClient().set("k", None)

        with self.assertRaises(DataError):
            self.run_async(scenario())
        self.assertEqual(calls, [])

    def test_auth_failure_closes_connection(self):
        calls = []
        reader, writer = FakeReader([b"-WRONGPASS invalid\r\n"]), FakeWriter()
        self.patch_open(reader, writer, calls)

        async def run():
            client = AsyncCacheStormClient(password="bad")
            with self.assertRaises(ResponseError):
                await client.connect()
            return client

        client = self.run_async(run())
        self.assertIsNone(client._writer)
        self.assertTrue(writer.closed)

    def test_connect_failure_raises_connection_error(self):
        async def failing_open_connection(host, port):
            raise OSError("refused")

        async def scenario():
            with patch.object(asyncio, "open_connection", failing_open_connection):
                await AsyncCacheStormClient().connect()

        with self.assertRaises(ConnectionError):
            self.run_async(scenario())


if __name__ == "__main__":
    unittest.main()
