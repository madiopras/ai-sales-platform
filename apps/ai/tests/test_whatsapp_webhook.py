"""Tests for the WhatsApp webhook routes (GET verify + POST receive)."""

from __future__ import annotations

from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.config import Settings
from app.middleware import RequestContextMiddleware
from app.platform.errors import register_exception_handlers
from app.whatsapp.idempotency import InboundDedup
from app.whatsapp.models import InboundMessage
from app.whatsapp.outbound import OutboundSender
from app.whatsapp.provider import WebhookVerificationError
from app.whatsapp.router import router as whatsapp_router
from app.whatsapp.service import WhatsAppService
from tests.test_whatsapp_service import FakeRedis, RecordingProvider


class WebhookProvider(RecordingProvider):
    """Provider stub with controllable verify/parse for webhook tests."""

    def __init__(self, *, verify_token: str = "verify-me", signature_ok: bool = True):
        super().__init__()
        self._verify_token = verify_token
        self._signature_ok = signature_ok
        self.parsed: list[InboundMessage] = []

    def verify_subscription(self, mode, token, challenge):
        if mode == "subscribe" and token == self._verify_token and challenge:
            return challenge
        raise WebhookVerificationError("verification failed")

    def verify_signature(self, raw_body, signature_header):
        return self._signature_ok

    def parse_inbound(self, payload):
        return list(self.parsed)


def build_app(provider: WebhookProvider) -> FastAPI:
    app = FastAPI()
    app.add_middleware(RequestContextMiddleware)
    register_exception_handlers(app)
    settings = Settings(wa_echo_reply=True)
    app.state.whatsapp = WhatsAppService(
        settings=settings,
        provider=provider,
        dedup=InboundDedup(FakeRedis(), ttl_seconds=60),
        sender=OutboundSender(provider, max_retries=0),
    )
    app.include_router(whatsapp_router)
    return app


def test_verify_get_echoes_challenge():
    provider = WebhookProvider()
    client = TestClient(build_app(provider))
    resp = client.get(
        "/webhooks/whatsapp",
        params={"hub.mode": "subscribe", "hub.verify_token": "verify-me", "hub.challenge": "xyz"},
    )
    assert resp.status_code == 200
    assert resp.text == "xyz"


def test_verify_get_rejects_bad_token():
    provider = WebhookProvider()
    client = TestClient(build_app(provider))
    resp = client.get(
        "/webhooks/whatsapp",
        params={"hub.mode": "subscribe", "hub.verify_token": "nope", "hub.challenge": "xyz"},
    )
    assert resp.status_code == 403


def test_post_rejects_bad_signature():
    provider = WebhookProvider(signature_ok=False)
    client = TestClient(build_app(provider))
    resp = client.post("/webhooks/whatsapp", json={"entry": []})
    assert resp.status_code == 403
    assert provider.sent == []


def test_post_processes_messages_and_replies():
    provider = WebhookProvider()
    provider.parsed = [
        InboundMessage(wa_id="628123", message_id="wamid.1", type="text", text="halo")
    ]
    client = TestClient(build_app(provider))
    resp = client.post("/webhooks/whatsapp", json={"entry": [{"changes": []}]})
    assert resp.status_code == 200
    # Background task runs on TestClient context exit; reply was sent.
    assert len(provider.sent) == 1
    assert provider.sent[0].text == "Echo: halo"


def test_post_status_callback_returns_200_no_reply():
    provider = WebhookProvider()
    provider.parsed = []  # status callback: nothing to process
    client = TestClient(build_app(provider))
    resp = client.post("/webhooks/whatsapp", json={"entry": [{"changes": []}]})
    assert resp.status_code == 200
    assert provider.sent == []
