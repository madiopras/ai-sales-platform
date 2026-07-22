"""JSON response envelope consistent with the Go backend.

Mirrors ``response.Envelope`` in the Go API: every response is
``{"data": ..., "error": {"code", "message"}, "request_id": ...}`` with the
unused field omitted.
"""

from __future__ import annotations

from typing import Any

from fastapi.responses import JSONResponse

from app.platform.logging import get_request_id


def _envelope(
    *,
    data: Any = None,
    error: dict[str, str] | None = None,
) -> dict[str, Any]:
    body: dict[str, Any] = {}
    if data is not None:
        body["data"] = data
    if error is not None:
        body["error"] = error
    request_id = get_request_id()
    if request_id:
        body["request_id"] = request_id
    return body


def ok(data: Any, status_code: int = 200) -> JSONResponse:
    return JSONResponse(status_code=status_code, content=_envelope(data=data))


def fail(status_code: int, code: str, message: str) -> JSONResponse:
    return JSONResponse(
        status_code=status_code,
        content=_envelope(error={"code": code, "message": message}),
    )
