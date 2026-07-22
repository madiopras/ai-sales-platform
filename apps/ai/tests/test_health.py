"""Tests for the health endpoints and the response envelope."""

from __future__ import annotations

from fastapi.testclient import TestClient

from tests.conftest import build_app


def test_live_returns_ok(client: TestClient):
    resp = client.get("/health/live")
    assert resp.status_code == 200
    body = resp.json()
    assert body["data"]["status"] == "ok"
    # Request id echoed in the envelope and the response header.
    assert body["request_id"]
    assert resp.headers["X-Request-ID"] == body["request_id"]


def test_ready_ok_when_dependencies_healthy(client: TestClient):
    resp = client.get("/health/ready")
    assert resp.status_code == 200
    body = resp.json()
    assert body["data"]["status"] == "ok"
    assert body["data"]["checks"] == {"backend": "ok", "redis": "ok"}


def test_ready_503_when_backend_down():
    client = TestClient(build_app(backend_healthy=False))
    resp = client.get("/health/ready")
    assert resp.status_code == 503
    assert resp.json()["error"]["code"] == "SERVICE_UNAVAILABLE"


def test_ready_503_when_redis_down():
    client = TestClient(build_app(redis_healthy=False))
    resp = client.get("/health/ready")
    assert resp.status_code == 503
    assert resp.json()["error"]["code"] == "SERVICE_UNAVAILABLE"


def test_incoming_request_id_is_preserved(client: TestClient):
    resp = client.get("/health/live", headers={"X-Request-ID": "trace-123"})
    assert resp.headers["X-Request-ID"] == "trace-123"
    assert resp.json()["request_id"] == "trace-123"


def test_unknown_route_returns_envelope(client: TestClient):
    resp = client.get("/nope")
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "NOT_FOUND"
