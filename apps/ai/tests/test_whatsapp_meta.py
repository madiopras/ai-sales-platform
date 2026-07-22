"""Tests for the Meta WhatsApp provider: verify, signature, parse, send."""

from __future__ import annotations

import hashlib
import hmac

import httpx
import pytest
import respx

from app.config import Settings
from app.whatsapp.meta import MetaWhatsAppProvider
from app.whatsapp.models import OutboundMessage
from app.whatsapp.provider import WebhookVerificationError

WA_BASE = "https://graph.test/v20.0"


def make_provider(**overrides) -> MetaWhatsAppProvider:
    settings = Settings(
        wa_provider="meta",
        wa_verify_token="verify-me",
        wa_access_token="access-token",
        wa_phone_number_id="123456",
        wa_api_base_url=WA_BASE,
        wa_app_secret=overrides.pop("wa_app_secret", ""),
        **overrides,
    )
    return MetaWhatsAppProvider(settings)


def test_verify_subscription_returns_challenge_on_match():
    provider = make_provider()
    assert provider.verify_subscription("subscribe", "verify-me", "42") == "42"


def test_verify_subscription_rejects_bad_token():
    provider = make_provider()
    with pytest.raises(WebhookVerificationError):
        provider.verify_subscription("subscribe", "wrong", "42")


def test_verify_subscription_rejects_bad_mode():
    provider = make_provider()
    with pytest.raises(WebhookVerificationError):
        provider.verify_subscription("unsubscribe", "verify-me", "42")


def test_verify_signature_skipped_when_no_secret():
    provider = make_provider()  # no app secret
    assert provider.verify_signature(b"{}", None) is True


def test_verify_signature_valid():
    provider = make_provider(wa_app_secret="s3cr3t")
    body = b'{"hello":"world"}'
    digest = hmac.new(b"s3cr3t", body, hashlib.sha256).hexdigest()
    assert provider.verify_signature(body, f"sha256={digest}") is True


def test_verify_signature_rejects_tampered_body():
    provider = make_provider(wa_app_secret="s3cr3t")
    digest = hmac.new(b"s3cr3t", b"original", hashlib.sha256).hexdigest()
    assert provider.verify_signature(b"tampered", f"sha256={digest}") is False


def test_verify_signature_rejects_missing_header():
    provider = make_provider(wa_app_secret="s3cr3t")
    assert provider.verify_signature(b"body", None) is False


def test_parse_inbound_text_message():
    provider = make_provider()
    payload = {
        "entry": [
            {
                "changes": [
                    {
                        "value": {
                            "contacts": [
                                {"wa_id": "628123", "profile": {"name": "Budi"}}
                            ],
                            "messages": [
                                {
                                    "from": "628123",
                                    "id": "wamid.ABC",
                                    "timestamp": "1700000000",
                                    "type": "text",
                                    "text": {"body": "halo"},
                                }
                            ],
                        }
                    }
                ]
            }
        ]
    }
    messages = provider.parse_inbound(payload)
    assert len(messages) == 1
    msg = messages[0]
    assert msg.wa_id == "628123"
    assert msg.message_id == "wamid.ABC"
    assert msg.type == "text"
    assert msg.text == "halo"
    assert msg.name == "Budi"
    assert msg.timestamp == 1700000000


def test_parse_inbound_interactive_reply():
    provider = make_provider()
    payload = {
        "entry": [
            {
                "changes": [
                    {
                        "value": {
                            "messages": [
                                {
                                    "from": "628999",
                                    "id": "wamid.INT",
                                    "type": "interactive",
                                    "interactive": {
                                        "button_reply": {"id": "opt_1", "title": "Beli"}
                                    },
                                }
                            ]
                        }
                    }
                ]
            }
        ]
    }
    messages = provider.parse_inbound(payload)
    assert len(messages) == 1
    assert messages[0].text == "Beli"


def test_parse_inbound_status_callback_yields_nothing():
    provider = make_provider()
    payload = {
        "entry": [
            {"changes": [{"value": {"statuses": [{"id": "wamid.X", "status": "delivered"}]}}]}
        ]
    }
    assert provider.parse_inbound(payload) == []


@respx.mock
async def test_send_text_posts_to_messages_endpoint():
    provider = make_provider()
    route = respx.post(f"{WA_BASE}/123456/messages").mock(
        return_value=httpx.Response(200, json={"messages": [{"id": "wamid.OUT"}]})
    )
    try:
        result = await provider.send(OutboundMessage.text_message("628123", "hai"))
    finally:
        await provider.aclose()

    assert result == {"messages": [{"id": "wamid.OUT"}]}
    sent = route.calls.last.request
    assert sent.headers["Authorization"] == "Bearer access-token"
    body = sent.content.decode()
    assert '"to":"628123"' in body.replace(" ", "")
    assert '"body":"hai"' in body.replace(" ", "")


@respx.mock
async def test_send_raises_on_api_error():
    provider = make_provider()
    respx.post(f"{WA_BASE}/123456/messages").mock(
        return_value=httpx.Response(400, json={"error": {"message": "bad"}})
    )
    try:
        with pytest.raises(httpx.HTTPStatusError):
            await provider.send(OutboundMessage.text_message("628123", "hai"))
    finally:
        await provider.aclose()
