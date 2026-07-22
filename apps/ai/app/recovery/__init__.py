"""Abandoned-cart recovery (Phase A9).

A session whose cart has items but that has gone idle is a lost sale we can win
back with a gentle, well-timed reminder (sales-rules "Abandoned Cart"). This
package provides the recovery sweep:

- :mod:`app.recovery.service` scans candidate sessions, decides which qualify
  for a reminder (cart present, idle past a threshold, not already reminded, not
  in handover, within quiet-hours and a max-reminders cap), sends one friendly
  WhatsApp nudge, and marks the session so it isn't nagged again.

The sweep is idempotent and rate-limited per customer so we never spam (a hard
rule from sales-rules). It reads cart *presence* from the session's cached
reference but always fetches live cart contents from the backend before
messaging, so a cart emptied/checked-out elsewhere doesn't trigger a stale
reminder (BR-004). Emits analytics for reminders sent (BR-050).
"""

from app.recovery.service import CartRecoveryService

__all__ = ["CartRecoveryService"]
