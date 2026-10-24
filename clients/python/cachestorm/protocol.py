"""RESP protocol implementation."""

from typing import Any, List, Tuple

from .exceptions import DataError, InvalidResponse, ResponseError


class IncompleteResponse(ValueError):
    """Raised when the buffer does not yet hold a complete RESP value."""
    pass


def _encode_arg(item: Any) -> bytes:
    if isinstance(item, (bytes, bytearray, memoryview)):
        return bytes(item)
    if isinstance(item, str):
        return item.encode('utf-8')
    return str(item).encode('utf-8')


def encode_resp(data: List[Any]) -> bytes:
    """Encode data to RESP format."""
    if not isinstance(data, list):
        raise ValueError("Data must be a list")

    parts = [b"*%d\r\n" % len(data)]

    for item in data:
        if item is None:
            raise DataError("Invalid argument: None cannot be sent as a command argument")
        payload = _encode_arg(item)
        parts.append(b"$%d\r\n" % len(payload))
        parts.append(payload)
        parts.append(b"\r\n")

    return b"".join(parts)


def _read_line(data: bytes, pos: int) -> Tuple[bytes, int]:
    end = data.find(b'\r\n', pos)
    if end == -1:
        raise IncompleteResponse("Incomplete data")
    return data[pos:end], end + 2


def _parse_int(raw: bytes) -> int:
    try:
        return int(raw)
    except ValueError:
        raise InvalidResponse(f"Invalid integer in response: {raw!r}")


def _decode_at(data: bytes, pos: int, raise_errors: bool) -> Tuple[Any, int]:
    if pos >= len(data):
        raise IncompleteResponse("Incomplete data")

    prefix = data[pos:pos + 1]
    line, next_pos = _read_line(data, pos + 1)

    if prefix == b'+':
        return line.decode('utf-8', errors='replace'), next_pos

    if prefix == b'-':
        error_msg = line.decode('utf-8', errors='replace')
        error = ResponseError(f"Redis error: {error_msg}")
        if raise_errors:
            raise error
        return error, next_pos

    if prefix == b':':
        return _parse_int(line), next_pos

    if prefix == b'$':
        length = _parse_int(line)
        if length == -1:
            return None, next_pos
        if length < 0:
            raise InvalidResponse(f"Invalid bulk length: {length}")
        end_pos = next_pos + length
        if len(data) < end_pos + 2:
            raise IncompleteResponse("Incomplete data")
        if data[end_pos:end_pos + 2] != b'\r\n':
            raise InvalidResponse("Bulk string not terminated by CRLF")
        return data[next_pos:end_pos], end_pos + 2

    if prefix == b'*':
        count = _parse_int(line)
        if count == -1:
            return None, next_pos
        if count < 0:
            raise InvalidResponse(f"Invalid array length: {count}")
        elements = []
        for _ in range(count):
            elem, next_pos = _decode_at(data, next_pos, raise_errors)
            elements.append(elem)
        return elements, next_pos

    raise InvalidResponse(f"Unknown RESP prefix: {prefix!r}")


def decode_resp(data: bytes, raise_errors: bool = True) -> Tuple[Any, int]:
    """Decode RESP data.

    Returns (decoded_value, bytes_consumed). Raises IncompleteResponse
    (a ValueError) when more data is needed and InvalidResponse for
    malformed input. Error replies raise ResponseError, or are returned
    as ResponseError instances when raise_errors is False.
    """
    if not data:
        raise IncompleteResponse("Empty data")
    return _decode_at(data, 0, raise_errors)
