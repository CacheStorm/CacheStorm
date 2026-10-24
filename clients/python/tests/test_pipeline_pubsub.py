import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from cachestorm.exceptions import ConnectionError, DataError, ResponseError
from cachestorm.pipeline import Pipeline
from cachestorm.protocol import decode_resp
from cachestorm.pubsub import PubSub
from tests.test_client import FakeSocket


class FakePool:
    def __init__(self, sock):
        self.sock = sock
        self.handed_out = 0
        self.released = []
        self.discarded = []

    def get_connection(self, timeout=None):
        self.handed_out += 1
        return self.sock

    def release_connection(self, conn):
        self.released.append(conn)

    def discard_connection(self, conn):
        conn.close()
        self.discarded.append(conn)


class PipelineTest(unittest.TestCase):
    def make(self, chunks):
        sock = FakeSocket(chunks)
        pool = FakePool(sock)
        return Pipeline(pool), pool, sock

    def test_all_replies_fragmented(self):
        pipe, pool, sock = self.make([b"+O", b"K\r\n$2\r", b"\n\xc5\x9f\r\n:1", b"\r\n"])
        pipe.set("k", "ş").get("k").exists("k")
        self.assertEqual(pipe.execute(), ["OK", b"\xc5\x9f", 1])
        self.assertEqual(pool.released, [sock])
        self.assertIn(b"$2\r\n\xc5\x9f\r\n", sock.sent)

    def test_error_raised_only_after_all_replies_read(self):
        pipe, pool, sock = self.make([b"+OK\r\n-WRONGTYPE bad\r\n:5\r\n"])
        pipe.set("k", "v").get("h").exists("k")
        with self.assertRaises(ResponseError) as cm:
            pipe.execute()
        self.assertIn("WRONGTYPE", str(cm.exception))
        self.assertEqual(sock.chunks, [])
        self.assertEqual(pool.released, [sock])
        self.assertEqual(pool.discarded, [])

    def test_errors_kept_in_position_without_raise(self):
        pipe, _, _ = self.make([b"+OK\r\n-ERR one\r\n:5\r\n"])
        pipe.set("k", "v").get("h").exists("k")
        results = pipe.execute(raise_on_error=False)
        self.assertEqual(results[0], "OK")
        self.assertIsInstance(results[1], ResponseError)
        self.assertEqual(results[2], 5)

    def test_eof_mid_pipeline_raises_connection_error(self):
        pipe, pool, sock = self.make([b"+OK\r\n$5\r\nab"])
        pipe.set("k", "v").get("k")
        with self.assertRaises(ConnectionError):
            pipe.execute()
        self.assertEqual(pool.released, [])
        self.assertEqual(pool.discarded, [sock])
        self.assertTrue(sock.closed)

    def test_none_argument_raises_before_sending(self):
        pipe, pool, sock = self.make([])
        pipe.set("k", "v").execute_command("SET", "x", None)
        with self.assertRaises(DataError):
            pipe.execute()
        self.assertEqual(pool.handed_out, 0)
        self.assertEqual(sock.sent, b"")

    def test_empty_pipeline(self):
        pipe, pool, _ = self.make([])
        self.assertEqual(pipe.execute(), [])
        self.assertEqual(pool.handed_out, 0)


class PubSubTest(unittest.TestCase):
    def test_parse_decoded_message_frame(self):
        frame, _ = decode_resp(b"*3\r\n$7\r\nmessage\r\n$4\r\nnews\r\n$2\r\n\xff\x00\r\n")
        self.assertEqual(
            PubSub(None)._parse_message(frame),
            {"type": "message", "channel": "news", "data": b"\xff\x00"},
        )

    def test_parse_decoded_pmessage_frame(self):
        frame, _ = decode_resp(
            b"*4\r\n$8\r\npmessage\r\n$2\r\nn*\r\n$4\r\nnews\r\n$2\r\nhi\r\n"
        )
        self.assertEqual(
            PubSub(None)._parse_message(frame),
            {"type": "pmessage", "pattern": "n*", "channel": "news", "data": b"hi"},
        )

    def test_parse_decoded_subscribe_frame(self):
        frame, _ = decode_resp(b"*3\r\n$9\r\nsubscribe\r\n$4\r\nnews\r\n:1\r\n")
        self.assertEqual(
            PubSub(None)._parse_message(frame),
            {"type": "subscribe", "channel": "news", "subscribed_count": 1},
        )

    def test_short_pmessage_ignored(self):
        frame, _ = decode_resp(b"*3\r\n$8\r\npmessage\r\n$2\r\nn*\r\n$4\r\nnews\r\n")
        self.assertIsNone(PubSub(None)._parse_message(frame))

    def test_listen_delivers_fragmented_frames_and_calls_handler(self):
        stream = (
            b"*3\r\n$9\r\nsubscribe\r\n$4\r\nnews\r\n:1\r\n"
            b"*3\r\n$7\r\nmessage\r\n$4\r\nnews\r\n$5\r\nhello\r\n"
        )
        sock = FakeSocket([stream[i:i + 7] for i in range(0, len(stream), 7)])
        pubsub = PubSub(FakePool(sock))
        received = []
        pubsub.subscribe("news", handler=received.append)

        messages = list(pubsub.listen(timeout=2.0))

        self.assertIn(b"$9\r\nSUBSCRIBE\r\n$4\r\nnews\r\n", sock.sent)
        self.assertEqual(
            messages,
            [
                {"type": "subscribe", "channel": "news", "subscribed_count": 1},
                {"type": "message", "channel": "news", "data": b"hello"},
            ],
        )
        self.assertEqual(received, [messages[1]])


if __name__ == "__main__":
    unittest.main()
