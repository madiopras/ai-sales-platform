"""Shared test fixtures.

Builds a FastAPI app with the health router and middleware but without the
real lifespan, so tests can inject fake backend/redis clients into app.state.
"""

from __future__ import annotations

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.health import router as health_router
from app.middleware import RequestContextMiddleware
from app.platform.errors import register_exception_handlers


class FakeBackend:
    def __init__(self, healthy: bool = True):
        self.healthy = healthy

    async def ping(self) -> bool:
        return self.healthy


class FakeRedis:
    def __init__(self, healthy: bool = True):
        self.healthy = healthy

    async def ping(self) -> bool:
        if not self.healthy:
            raise RuntimeError("redis down")
        return True


def build_app(*, backend_healthy: bool = True, redis_healthy: bool = True) -> FastAPI:
    app = FastAPI()
    app.add_middleware(RequestContextMiddleware)
    register_exception_handlers(app)
    app.include_router(health_router)
    app.state.backend = FakeBackend(backend_healthy)
    app.state.redis = FakeRedis(redis_healthy)
    return app


@pytest.fixture
def client() -> TestClient:
    return TestClient(build_app())
