"""HTTP client for the Go backend internal API (`/internal/v1`).

This is the single place where the AI service talks to the backend. It owns:

- base URL + service-token auth header (``X-Service-Token``)
- timeouts and light retries for idempotent (GET) requests
- mapping backend error envelopes to :class:`BackendError`

No domain logic lives here — prices, stock, shipping costs and totals always
come from the backend (BR-004). Higher phases add typed tool wrappers on top of
this thin client.
"""

from __future__ import annotations

import asyncio
from typing import Any

import httpx

from app.config import Settings
from app.platform.errors import BackendError
from app.platform.logging import get_logger, get_request_id

logger = get_logger(__name__)

# Header the Go ServiceToken middleware checks (middleware/service_token.go).
_SERVICE_TOKEN_HEADER = "X-Service-Token"
# Correlation id propagated to the backend so logs line up across services.
_REQUEST_ID_HEADER = "X-Request-ID"

_IDEMPOTENT_METHODS = {"GET", "HEAD", "OPTIONS"}


class BackendClient:
    """Thin async wrapper around ``httpx.AsyncClient`` for `/internal/v1`."""

    def __init__(self, settings: Settings, client: httpx.AsyncClient | None = None):
        self._settings = settings
        self._max_retries = max(0, settings.backend_max_retries)
        headers = {"Accept": "application/json"}
        if settings.backend_service_token:
            headers[_SERVICE_TOKEN_HEADER] = settings.backend_service_token
        self._client = client or httpx.AsyncClient(
            base_url=settings.internal_base_url,
            timeout=settings.backend_timeout_seconds,
            headers=headers,
        )

    async def aclose(self) -> None:
        await self._client.aclose()

    async def request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, Any] | None = None,
        json: Any | None = None,
    ) -> Any:
        """Perform a request and return the envelope's ``data`` field.

        Retries idempotent requests on transport errors and 5xx responses.
        Raises :class:`BackendError` on unrecoverable failures.
        """
        headers: dict[str, str] = {}
        request_id = get_request_id()
        if request_id:
            headers[_REQUEST_ID_HEADER] = request_id

        attempts = self._max_retries + 1 if method.upper() in _IDEMPOTENT_METHODS else 1
        last_exc: Exception | None = None

        for attempt in range(1, attempts + 1):
            try:
                response = await self._client.request(
                    method, path, params=params, json=json, headers=headers
                )
            except httpx.RequestError as exc:
                last_exc = exc
                logger.warning(
                    "backend_request_error",
                    extra={"method": method, "path": path, "attempt": attempt},
                )
                if attempt < attempts:
                    await asyncio.sleep(0.1 * attempt)
                    continue
                raise BackendError("backend unreachable") from exc

            if response.status_code >= 500 and attempt < attempts:
                logger.warning(
                    "backend_5xx_retry",
                    extra={
                        "method": method,
                        "path": path,
                        "status_code": response.status_code,
                        "attempt": attempt,
                    },
                )
                await asyncio.sleep(0.1 * attempt)
                continue

            return self._handle_response(response, method, path)

        # Only reached if all idempotent retries hit transport errors.
        raise BackendError("backend unreachable") from last_exc

    def _handle_response(self, response: httpx.Response, method: str, path: str) -> Any:
        try:
            body = response.json()
        except ValueError:
            body = None

        if response.is_success:
            if isinstance(body, dict) and "data" in body:
                return body["data"]
            return body

        upstream_code = None
        upstream_message = None
        if isinstance(body, dict) and isinstance(body.get("error"), dict):
            upstream_code = body["error"].get("code")
            upstream_message = body["error"].get("message")

        logger.warning(
            "backend_error_response",
            extra={
                "method": method,
                "path": path,
                "status_code": response.status_code,
                "upstream_code": upstream_code,
            },
        )

        # Preserve client-error semantics (404/409/422/...) so callers can react;
        # collapse 5xx to a generic 502 so we don't leak backend internals.
        status = response.status_code if response.status_code < 500 else 502
        message = upstream_message or "backend request failed"
        raise BackendError(
            message,
            status_code=status,
            upstream_status=response.status_code,
            upstream_code=upstream_code,
        )

    # Convenience helpers -------------------------------------------------

    async def get(self, path: str, *, params: dict[str, Any] | None = None) -> Any:
        return await self.request("GET", path, params=params)

    async def post(self, path: str, *, json: Any | None = None) -> Any:
        return await self.request("POST", path, json=json)

    async def put(self, path: str, *, json: Any | None = None) -> Any:
        return await self.request("PUT", path, json=json)

    async def delete(self, path: str, *, params: dict[str, Any] | None = None) -> Any:
        return await self.request("DELETE", path, params=params)

    async def ping(self) -> bool:
        """Check backend readiness via its `/health/ready` endpoint.

        The health route lives at the backend root, not under `/internal/v1`,
        so we build an absolute URL from the configured base.
        """
        url = f"{self._settings.backend_base_url.rstrip('/')}/health/ready"
        try:
            response = await self._client.get(url)
        except httpx.RequestError:
            return False
        return response.is_success
