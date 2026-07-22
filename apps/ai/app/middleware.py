"""HTTP middleware: request id + structured access logging.

Assigns a request id to each request (reusing an inbound ``X-Request-ID`` when
present), stores it in a contextvar so all logs correlate, echoes it back in the
response header, and logs one structured line per request.
"""

from __future__ import annotations

import time
import uuid

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

from app.platform.logging import get_logger, set_request_id

logger = get_logger("http.access")

_REQUEST_ID_HEADER = "X-Request-ID"


class RequestContextMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        request_id = request.headers.get(_REQUEST_ID_HEADER) or uuid.uuid4().hex
        set_request_id(request_id)
        request.state.request_id = request_id

        start = time.perf_counter()
        try:
            response: Response = await call_next(request)
        finally:
            set_request_id(None)

        elapsed_ms = round((time.perf_counter() - start) * 1000, 2)
        response.headers[_REQUEST_ID_HEADER] = request_id

        # Re-bind for the log line emitted after the response is produced.
        set_request_id(request_id)
        logger.info(
            "request",
            extra={
                "method": request.method,
                "path": request.url.path,
                "status_code": response.status_code,
                "duration_ms": elapsed_ms,
            },
        )
        set_request_id(None)
        return response
