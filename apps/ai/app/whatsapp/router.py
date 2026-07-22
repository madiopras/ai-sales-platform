"""WhatsApp webhook routes.

- ``GET /webhooks/whatsapp``  — provider subscription verification (echo the
  challenge). Meta calls this once when you register the webhook.
- ``POST /webhooks/whatsapp`` — inbound messages + status callbacks. We verify
  the payload signature, parse messages, schedule background processing, and
  return 200 fast so the provider doesn't retry on our latency.

Security: the GET handshake validates ``WA_VERIFY_TOKEN``; the POST verifies the
``X-Hub-Signature-256`` HMAC when ``WA_APP_SECRET`` is set. Without a secret,
signature checks are skipped (development only) — flag this before going live.
"""

from __future__ import annotations

from fastapi import APIRouter, BackgroundTasks, Query, Request, Response

from app.platform.logging import get_logger
from app.whatsapp.provider import WebhookVerificationError
from app.whatsapp.service import WhatsAppService

logger = get_logger(__name__)

router = APIRouter(prefix="/webhooks", tags=["whatsapp"])

_SIGNATURE_HEADER = "X-Hub-Signature-256"


def _service(request: Request) -> WhatsAppService:
    return request.app.state.whatsapp


@router.get("/whatsapp")
async def verify_webhook(
    request: Request,
    hub_mode: str | None = Query(default=None, alias="hub.mode"),
    hub_verify_token: str | None = Query(default=None, alias="hub.verify_token"),
    hub_challenge: str | None = Query(default=None, alias="hub.challenge"),
):
    """Provider subscription handshake: echo the challenge on success."""
    provider = _service(request).provider
    try:
        challenge = provider.verify_subscription(hub_mode, hub_verify_token, hub_challenge)
    except WebhookVerificationError:
        logger.warning("wa_webhook_verify_failed")
        return Response(status_code=403, content="verification failed")
    # Meta expects the raw challenge string echoed back, not a JSON envelope.
    return Response(status_code=200, content=challenge, media_type="text/plain")


@router.post("/whatsapp")
async def receive_webhook(request: Request, background_tasks: BackgroundTasks):
    """Receive inbound messages: verify signature, parse, schedule processing."""
    service = _service(request)
    provider = service.provider

    raw_body = await request.body()
    signature = request.headers.get(_SIGNATURE_HEADER)
    if not provider.verify_signature(raw_body, signature):
        logger.warning("wa_webhook_bad_signature")
        return Response(status_code=403, content="invalid signature")

    try:
        payload = await request.json()
    except ValueError:
        payload = {}

    messages = provider.parse_inbound(payload) if isinstance(payload, dict) else []
    for message in messages:
        # Process out-of-band so we can ACK the provider immediately.
        background_tasks.add_task(service.handle_inbound, message)

    # Always 200 for well-formed calls (even status callbacks with no messages)
    # so the provider stops retrying.
    return Response(status_code=200, content="")
