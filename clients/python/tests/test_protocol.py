import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from cachestorm.exceptions import DataError, InvalidResponse, ResponseError
from cachestorm.protocol import IncompleteResponse, decode_resp, encode_resp


class EncodeRespTest(unittest.TestCase):
    def test_ascii(self):
        self.assertEqual(
            encode_resp(["SET", "key", "ascii"]),
            b"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nascii\r\n",
        )

    def test_turkish_uses_utf8_byte_length(self):
        self.assertEqual(
            encode_resp(["SET", "key", "ş"]),
            b"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$2\r\n\xc5\x9f\r\n",
        )

    def test_emoji_uses_utf8_byte_length(self):
        payload = "🚀".encode("utf-8")
        self.assertEqual(len(payload), 4)
        self.assertEqual(
            encode_resp(["SET", "k", "🚀"]),
            b"*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$4\r\n" + payload + b"\r\n",
        )

    def test_raw_binary_passes_through(self):
        self.assertEqual(
            encode_resp(["SET", "key", b"\xff\x00"]),
            b"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$2\r\n\xff\x00\r\n",
        )

    def test_bytearray_passes_through(self):
        self.assertEqual(encode_resp([bytearray(b"\x80")]), b"*1\r\n$1\r\n\x80\r\n")

    def test_numbers_are_bulk_strings(self):
        self.assertEqual(
            encode_resp(["EXPIRE", "k", 10, 1.5]),
            b"*4\r\n$6\r\nEXPIRE\r\n$1\r\nk\r\n$2\r\n10\r\n$3\r\n1.5\r\n",
        )

    def test_roundtrip_through_decoder(self):
        values = ["ascii", "ş", "🚀", b"\xff\x00", b""]
        encoded = encode_resp(values)
        decoded, consumed = decode_resp(encoded)
        self.assertEqual(consumed, len(encoded))
        self.assertEqual(
            decoded,
            [v if isinstance(v, bytes) else v.encode("utf-8") for v in values],
        )

    def test_none_argument_raises_data_error(self):
        with self.assertRaises(DataError):
            encode_resp(["SET", "k", None])

    def test_rejects_non_list(self):
        with self.assertRaises(ValueError):
            encode_resp(("GET", "k"))


class DecodeRespTest(unittest.TestCase):
    def test_simple_string(self):
        self.assertEqual(decode_resp(b"+OK\r\n"), ("OK", 5))

    def test_integer(self):
        self.assertEqual(decode_resp(b":-42\r\n"), (-42, 6))

    def test_bulk_string(self):
        self.assertEqual(decode_resp(b"$5\r\nhello\r\n"), (b"hello", 11))

    def test_bulk_string_by_byte_length(self):
        data = b"$2\r\n\xc5\x9f\r\n"
        self.assertEqual(decode_resp(data), ("ş".encode("utf-8"), len(data)))

    def test_bulk_string_with_embedded_crlf_and_binary(self):
        data = b"$6\r\n\xff\r\n\x00ab\r\n"
        self.assertEqual(decode_resp(data), (b"\xff\r\n\x00ab", len(data)))

    def test_empty_bulk_string(self):
        self.assertEqual(decode_resp(b"$0\r\n\r\n"), (b"", 6))

    def test_null_bulk(self):
        self.assertEqual(decode_resp(b"$-1\r\n"), (None, 5))

    def test_null_array(self):
        self.assertEqual(decode_resp(b"*-1\r\n"), (None, 5))

    def test_array(self):
        data = b"*3\r\n$3\r\nfoo\r\n:7\r\n$-1\r\n"
        self.assertEqual(decode_resp(data), ([b"foo", 7, None], len(data)))

    def test_nested_array(self):
        data = b"*2\r\n*1\r\n+a\r\n*0\r\n"
        self.assertEqual(decode_resp(data), ([["a"], []], len(data)))

    def test_consumed_excludes_trailing_data(self):
        self.assertEqual(decode_resp(b":1\r\n:2\r\n"), (1, 4))

    def test_error_reply_raises(self):
        with self.assertRaises(ResponseError) as cm:
            decode_resp(b"-ERR unknown command\r\n")
        self.assertIn("ERR unknown command", str(cm.exception))

    def test_error_inside_array_raises(self):
        with self.assertRaises(ResponseError):
            decode_resp(b"*2\r\n:1\r\n-WRONGTYPE bad\r\n")

    def test_error_returned_when_not_raising(self):
        value, consumed = decode_resp(b"-ERR boom\r\n:1\r\n", raise_errors=False)
        self.assertIsInstance(value, ResponseError)
        self.assertIn("ERR boom", str(value))
        self.assertEqual(consumed, 11)

    def test_nested_error_returned_in_position_when_not_raising(self):
        data = b"*3\r\n:1\r\n-WRONGTYPE bad\r\n+OK\r\n"
        value, consumed = decode_resp(data, raise_errors=False)
        self.assertEqual(consumed, len(data))
        self.assertEqual(value[0], 1)
        self.assertIsInstance(value[1], ResponseError)
        self.assertEqual(value[2], "OK")

    def test_every_fragment_is_incomplete(self):
        samples = [
            b"+OK\r\n",
            b":12345\r\n",
            b"$5\r\nhello\r\n",
            b"$2\r\n\xc5\x9f\r\n",
            b"*3\r\n$3\r\nfoo\r\n:7\r\n$-1\r\n",
            b"*2\r\n*1\r\n$4\r\n\xf0\x9f\x9a\x80\r\n$0\r\n\r\n",
            b"-ERR boom\r\n",
        ]
        for full in samples:
            for cut in range(len(full)):
                with self.subTest(full=full, cut=cut):
                    with self.assertRaises(IncompleteResponse):
                        decode_resp(full[:cut])

    def test_incremental_feed_yields_value_once_complete(self):
        full = b"*2\r\n$4\r\n\xf0\x9f\x9a\x80\r\n:9\r\n"
        buffer = b""
        result = None
        for i in range(len(full)):
            buffer += full[i:i + 1]
            try:
                result = decode_resp(buffer)
            except IncompleteResponse:
                continue
        self.assertEqual(result, ([b"\xf0\x9f\x9a\x80", 9], len(full)))

    def test_incomplete_is_value_error(self):
        self.assertTrue(issubclass(IncompleteResponse, ValueError))

    def test_unknown_prefix_is_invalid_not_incomplete(self):
        with self.assertRaises(InvalidResponse):
            decode_resp(b"!x\r\n")

    def test_bad_integer_is_invalid_not_incomplete(self):
        with self.assertRaises(InvalidResponse):
            decode_resp(b":abc\r\n")

    def test_bulk_without_crlf_terminator_is_invalid(self):
        with self.assertRaises(InvalidResponse):
            decode_resp(b"$2\r\nabXY")

    def test_non_utf8_simple_string_does_not_hang(self):
        self.assertEqual(decode_resp(b"+\xff\r\n")[1], 4)


if __name__ == "__main__":
    unittest.main()
