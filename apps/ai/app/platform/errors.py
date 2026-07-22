"""Internal exceptions + FastAPI exception handlers.

All errors are rendered through the shared response envelope so clients see a
consistent ``{"error": {"code", "message"}}`` shape regardless of where the
failure originated.
"""

from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from starlette.exceptions import HTTPException as StarletteHTTPException

from app.platform.logging import get_logger
from app.platform.response import fail

logger = get_logger(__name__)


class AppError(Exception):
    """Base class for expected, mapped application errors."""

    status_code = 500
    code = "INTERNAL_ERROR"

    def __init__(self, message: str, *, code: str | None = None, status_code: int | None = None):
        super().__init__(message)
        self.message = message
        if code is not None:
            self.code = code
        if status_code is not None:
            self.status_code = status_code


class BackendError(AppError):
    """Raised when the Go backend returns an error or is unreachable.

    ``upstream_code`` preserves the backend's own error code for logging/debug
    without leaking it verbatim to end users unless we choose to.
    """

    status_code = 502
    code = "BACKEND_ERROR"

    def __init__(
        self,
        message: str,
        *,
        status_code: int = 502,
        upstream_status: int | None = None,
        upstream_code: str | None = None,
    ):
        super().__init__(message, code="BACKEND_ERROR", status_code=status_code)
        self.upstream_status = upstream_status
        self.upstream_code = upstream_code


def register_exception_handlers(app: FastAPI) -> None:
    @app.exception_handler(AppError)
    async def _handle_app_error(_: Request, exc: AppError):
        # Expected errors: log at warning, no stack trace noise.
        logger.warning(
            "app_error",
            extra={"error_code": exc.code, "status_code": exc.status_code},
        )
        return fail(exc.status_code, exc.code, exc.message)

    @app.exception_handler(RequestValidationError)
    async def _handle_validation(_: Request, exc: RequestValidationError):
        return fail(422, "VALIDATION_ERROR", "request validation failed")

    @app.exception_handler(StarletteHTTPException)
    async def _handle_http(_: Request, exc: StarletteHTTPException):
        code = "NOT_FOUND" if exc.status_code == 404 else "HTTP_ERROR"
        message = exc.detail if isinstance(exc.detail, str) else "request failed"
        return fail(exc.status_code, code, message)

    @app.exception_handler(Exception)
    async def _handle_unexpected(_: Request, exc: Exception):
        # Unexpected errors: log the stack trace but return a generic message so
        # we never leak internals to clients (BR-044).
        logger.exception("unhandled_error")
        return fail(500, "INTERNAL_ERROR", "internal server error")
