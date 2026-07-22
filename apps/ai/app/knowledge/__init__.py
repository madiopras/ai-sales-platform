"""Knowledge base (RAG-lite) for non-product questions (Phase A9).

Customers ask things the product catalog can't answer: shipping coverage,
payment methods, return policy, operating hours, size guidance. This package
provides a small retrieval layer over a curated, versioned knowledge base so the
agent answers those from published content instead of inventing policy.

- :mod:`app.knowledge.store` loads the KB articles (from ``docs/ai`` by default)
  and retrieves the best-matching article for a query with a lightweight keyword
  overlap score. It intentionally avoids heavyweight vector deps so the service
  stays lean and fully offline-testable; the scoring interface is the seam where
  a pgvector/embedding backend can drop in later without touching callers.

The KB never carries prices/stock/order data — those are always fetched live
from the backend (BR-004). Policy answers here are informational only; anything
requiring action (refund, negotiation) still routes to human handover (BR-003).
"""

from app.knowledge.store import KnowledgeArticle, KnowledgeBase, load_knowledge_base

__all__ = ["KnowledgeArticle", "KnowledgeBase", "load_knowledge_base"]
