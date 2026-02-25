"""CacheStorm exceptions."""


class CacheStormError(Exception):
    """Base CacheStorm error."""
    pass


class ConnectionError(CacheStormError):
    """Connection error."""
    pass


class TimeoutError(CacheStormError):
    """Timeout error."""
    pass


class ResponseError(CacheStormError):
    """Response error."""
    pass


class DataError(CacheStormError):
    """Data error."""
    pass


class InvalidResponse(ResponseError):
    """Invalid response error."""
    pass
