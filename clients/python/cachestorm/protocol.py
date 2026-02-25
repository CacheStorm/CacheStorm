"""RESP protocol implementation."""

from typing import Any, List, Tuple, Union


def encode_resp(data: List[Any]) -> bytes:
    """Encode data to RESP format."""
    if not isinstance(data, list):
        raise ValueError("Data must be a list")

    parts = [f"*{len(data)}\r\n"]

    for item in data:
        if item is None:
            parts.append("$-1\r\n")
        elif isinstance(item, bytes):
            parts.append(f"${len(item)}\r\n")
            parts.append(item.decode('utf-8', errors='replace'))
            parts.append("\r\n")
        elif isinstance(item, int):
            parts.append(f":{item}\r\n")
        else:
            s = str(item)
            parts.append(f"${len(s)}\r\n")
            parts.append(s)
            parts.append("\r\n")

    return "".join(parts).encode('utf-8')


def decode_resp(data: bytes) -> Tuple[Any, int]:
    """Decode RESP data.

    Returns (decoded_value, bytes_consumed)
    """
    if not data:
        raise ValueError("Empty data")

    prefix = data[0:1]

    if prefix == b'+':
        # Simple string
        end = data.find(b'\r\n')
        if end == -1:
            raise ValueError("Incomplete data")
        return data[1:end].decode('utf-8'), end + 2

    elif prefix == b'-':
        # Error
        end = data.find(b'\r\n')
        if end == -1:
            raise ValueError("Incomplete data")
        error_msg = data[1:end].decode('utf-8')
        raise Exception(f"Redis error: {error_msg}")

    elif prefix == b':':
        # Integer
        end = data.find(b'\r\n')
        if end == -1:
            raise ValueError("Incomplete data")
        return int(data[1:end]), end + 2

    elif prefix == b'$':
        # Bulk string
        end = data.find(b'\r\n')
        if end == -1:
            raise ValueError("Incomplete data")

        length = int(data[1:end])
        if length == -1:
            return None, end + 2

        start = end + 2
        end_pos = start + length

        if len(data) < end_pos + 2:
            raise ValueError("Incomplete data")

        return data[start:end_pos], end_pos + 2

    elif prefix == b'*':
        # Array
        end = data.find(b'\r\n')
        if end == -1:
            raise ValueError("Incomplete data")

        count = int(data[1:end])
        if count == -1:
            return None, end + 2

        elements = []
        pos = end + 2

        for _ in range(count):
            elem, consumed = decode_resp(data[pos:])
            elements.append(elem)
            pos += consumed

        return elements, pos

    else:
        raise ValueError(f"Unknown RESP prefix: {prefix}")
