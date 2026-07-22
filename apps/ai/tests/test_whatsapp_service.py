"""Tests for the WhatsApp service: idempotency + echo reply behavior."""

from __future__ import annotations

import pytest

from app.config import Settings
from app.whatsapp.idempotency import InboundDedup
from app.whatsapp.models import InboundMessage, OutboundMessage
from app.whatsapp.outbound import OutboundSender
from app.whatsapp.service import WhatsAppService


class FakeRedis:
    """Minimal in-memory Redis stub supporting SET NX EX."""

    def __init__(self):
        self.store: dict[str, str] = {}

    async def set(self, key, value, nx=False, ex=None):  # noqa: ARG002
        if nx and key in self.store:
            return None
        self.store[key] = value
        return True


class RecordingProvider:
    """Provider stub that records outbound sends."""

    name = "fake"

    def __init__(self, fail_times: int = 0):
        self.sent: list[OutboundMessage] = []
        self._fail_times = fail_times

    def verify_subscription(self, mode, token, challenge):  # pragma: no cover
        return challenge or ""

    def verify_signature(self, raw_body, signature_header):  # pragma: no cover
        return True

    def parse_inbound(self, payload):  # pragma: no cover
        return []

    async def send(self, message: OutboundMessage):
        if self._fail_times > 0:
            self._fail_times -= 1
            raise RuntimeError("transient")
        self.sent.append(message)
        return {"messages": [{"id": "wamid.OUT"}]}

    async def aclose(self):  # pragma: no cover
        return None


def make_service(provider: RecordingProvider, *, echo: bool = True, send_retries: int = 3):
    settings = Settings(wa_echo_reply=echo, wa_send_max_retries=send_retries)
    dedup = InboundDedup(FakeRedis(), ttl_seconds=60)
    sender = OutboundSender(provider, max_retries=send_retries)
    return WhatsAppService(settings=settings, provider=provider, dedup=dedup, sender=sender)


def inbound(text: str = "halo", message_id: str = "wamid.1") -> InboundMessage:
    return InboundMessage(wa_id="628123", message_id=message_id, type="text", text=text)


async def test_handle_inbound_echoes_text():
    provider = RecordingProvider()
    service = make_service(provider)
    await service.handle_inbound(inbound("kaos hitam"))
    assert len(provider.sent) == 1
    assert provider.sent[0].to == "628123"
    assert provider.sent[0].text == "Echo: kaos hitam"


async def test_handle_inbound_duplicate_is_skipped():
    provider = RecordingProvider()
    service = make_service(provider)
    msg = inbound(message_id="wamid.dup")
    await service.handle_inbound(msg)
    await service.handle_inbound(msg)  # retry with same id
    assert len(provider.sent) == 1  # only processed once


async def test_handle_inbound_no_echo_when_disabled():
    provider = RecordingProvider()
    service = make_service(provider, echo=False)
    await service.handle_inbound(inbound())
    assert provider.sent == []


async def test_outbound_retries_then_succeeds():
    provider = RecordingProvider(fail_times=2)
    sender = OutboundSender(provider, max_retries=3)
    ok = await sender.send(OutboundMessage.text_message("628123", "hai"))
    assert ok is True
    assert len(provider.sent) == 1


async def test_outbound_gives_up_after_retries():
    provider = RecordingProvider(fail_times=99)
    sender = OutboundSender(provider, max_retries=2)
    ok = await sender.send(OutboundMessage.text_message("628123", "hai"))
    assert ok is False
    assert provider.sent == []


@pytest.mark.parametrize("text", ["", None])
async def test_handle_inbound_non_text_gets_greeting(text):
    provider = RecordingProvider()
    service = make_service(provider)
    msg = InboundMessage(wa_id="628123", message_id="wamid.x", type="image", text=text)
    await service.handle_inbound(msg)
    assert len(provider.sent) == 1
    assert "terima" in provider.sent[0].text.lower()
