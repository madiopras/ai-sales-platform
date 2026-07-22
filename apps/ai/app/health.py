"""Health endpoints: liveness and readiness.

- ``GET /health/live`` always returns ok if the process is up.
- ``GET /health/ready`` pings the Go backend and Redis; returns 503 if any
  dependency is unavailable.

Response shape matches the Go backend's health handler:
``{"status": "ok"|"unavailable", "checks": {"backend": "ok", "redis": "ok"}}``.
"""

from __future__ import annotations

import asyncio

from fastapi import APIRouter, Request

from app.clients import redis as redis_client
from app.platform.response import fail, ok

router = APIRouter(prefix="/health", tags=["health"])


@router.get("/live")
async def live():
    return ok({"status": "ok"})


@router.get("/ready")
async def ready(request: Request):
    backend = request.app.state.backend
    redis = request.app.state.redis

    backend_ok, redis_ok = await asyncio.gather(
        backend.ping(),
        redis_client.ping(redis),
    )

    checks = {
        "backend": "ok" if backend_ok else "unavailable",
        "redis": "ok" if redis_ok else "unavailable",
    }

    if backend_ok and redis_ok:
        return ok({"status": "ok", "checks": checks})

    return fail(503, "SERVICE_UNAVAILABLE", "service unavailable")
