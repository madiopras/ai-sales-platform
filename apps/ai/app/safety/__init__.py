"""Human handover & safety guardrails (Phase A8).

Two concerns, kept separate so each is testable in isolation:

- :mod:`app.safety.injection` — a prompt-injection guard that refuses attempts
  to extract the system prompt / rules / secrets or to make the agent "forget"
  its instructions (BR-044, ai-system-prompt "Security"). Injection is refused
  in-place; it does not by itself escalate to a human.
- :mod:`app.safety.handover` — detection of when a conversation should leave the
  bot and go to a human (BR-003): explicit request, complaint/refund/return,
  price negotiation, payment/shipping trouble, bulk order, plus runtime signals
  like low confidence / unavailable data. Also the admin-notification hook fired
  when a session enters handover.

The bot never claims domain facts here; handover pauses automated replies so a
human can take over (the WhatsApp service goes quiet while ``session.handover``).
"""
