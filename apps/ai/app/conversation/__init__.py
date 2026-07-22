"""Conversation & session state (Phase A3).

Redis-backed session state that survives across inbound WhatsApp messages so
the agent can hold multi-turn conversations. This package owns:

- :mod:`app.conversation.models` — the session/context/state shapes
- :mod:`app.conversation.store`  — Redis persistence (serialize + TTL)
- :mod:`app.conversation.manager` — the conversation state machine

Phase A3 only maintains state; the LLM that *decides* what to do with it lands
in Phase A4. Prices/stock/totals still come from the backend, never from state.
"""
