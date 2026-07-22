"""Knowledge base loader + lightweight retrieval.

Loads a curated, versioned set of Q&A articles (JSON) and retrieves the best
match for a free-text query using a keyword-overlap score. This is a deliberately
simple "RAG-lite": no embeddings/vector store, so it runs offline with zero extra
deps and is fully deterministic in tests. The :meth:`KnowledgeBase.search`
interface is the seam where an embedding backend can later replace the scorer
without changing callers (the KB tool in ``llm/tools.py``).

Article shape (see ``docs/ai/knowledge-base.json``)::

    {"id", "title", "keywords": [...], "answer": "..."}

Only published content lives in the KB; the answer text is returned verbatim so
the agent never paraphrases policy into something inaccurate (BR-044). Missing or
malformed files degrade to an empty KB (the agent simply has no canned answer and
falls back to asking / handover) rather than crashing.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path

from app.platform.logging import get_logger

logger = get_logger(__name__)

# Minimum overlap score for a query to be considered a KB hit. Tuned low (one
# solid keyword match) since keywords are curated, but non-zero so unrelated
# chatter doesn't return a random article.
_MIN_SCORE = 1.0

# Tokens too generic to carry meaning; ignored when scoring so "berapa" alone
# doesn't match everything.
_STOPWORDS = frozenset(
    {
        "yang", "dan", "atau", "di", "ke", "dari", "untuk", "dengan", "ini", "itu",
        "apa", "apakah", "gimana", "bagaimana", "boleh", "bisa", "mau", "aku",
        "saya", "kamu", "kak", "min", "ada", "the", "a", "an", "is", "to", "of",
    }
)


def _tokenize(text: str) -> set[str]:
    """Lowercase word tokens with stopwords removed."""
    words = re.findall(r"\w+", text.lower())
    return {w for w in words if w not in _STOPWORDS and len(w) > 1}


@dataclass(slots=True, frozen=True)
class KnowledgeArticle:
    """A single published Q&A entry."""

    id: str
    title: str
    keywords: tuple[str, ...]
    answer: str

    def score(self, query_tokens: set[str], query_text: str) -> float:
        """Relevance of this article to a query.

        Combines two signals:
        - keyword-token overlap (each curated keyword token that appears in the
          query counts), and
        - a bonus when a multi-word keyword phrase appears verbatim in the query
          (stronger signal than scattered tokens).
        """
        if not query_tokens:
            return 0.0
        score = 0.0
        lowered = query_text.lower()
        for keyword in self.keywords:
            if " " in keyword:
                if keyword in lowered:
                    score += 2.0
                continue
            if keyword in query_tokens:
                score += 1.0
        return score


class KnowledgeBase:
    """In-memory collection of articles with keyword retrieval."""

    def __init__(self, articles: list[KnowledgeArticle], *, version: str = ""):
        self._articles = articles
        self.version = version

    def __len__(self) -> int:
        return len(self._articles)

    def search(self, query: str | None, *, min_score: float = _MIN_SCORE) -> KnowledgeArticle | None:
        """Return the best-matching article for ``query``, or None if none clears the bar."""
        if not query:
            return None
        query_tokens = _tokenize(query)
        best: KnowledgeArticle | None = None
        best_score = 0.0
        for article in self._articles:
            score = article.score(query_tokens, query)
            if score > best_score:
                best, best_score = article, score
        if best is None or best_score < min_score:
            return None
        return best


def _parse_article(raw: object) -> KnowledgeArticle | None:
    if not isinstance(raw, dict):
        return None
    article_id = str(raw.get("id") or "").strip()
    answer = str(raw.get("answer") or "").strip()
    if not article_id or not answer:
        return None
    keywords_in = raw.get("keywords") or []
    keywords = tuple(
        str(k).lower().strip() for k in keywords_in if isinstance(k, str) and k.strip()
    )
    return KnowledgeArticle(
        id=article_id,
        title=str(raw.get("title") or article_id).strip(),
        keywords=keywords,
        answer=answer,
    )


def _resolve_default() -> Path:
    """Resolve ``docs/ai/knowledge-base.json`` relative to the repo root.

    This file lives at ``apps/ai/app/knowledge/store.py``; walk up to the repo
    root (4 parents: knowledge → app → ai → apps → root) and join.
    """
    repo_root = Path(__file__).resolve().parents[4]
    return repo_root / "docs" / "ai" / "knowledge-base.json"


@lru_cache
def _load_cached(path: str) -> KnowledgeBase:
    source = Path(path) if path else _resolve_default()
    try:
        data = json.loads(source.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        # Missing/malformed KB shouldn't crash the service; degrade to empty so
        # the agent simply has no canned answer for policy questions.
        logger.warning("knowledge_base_unavailable", extra={"path": str(source)})
        return KnowledgeBase([], version="")

    articles_in = data.get("articles") if isinstance(data, dict) else None
    articles = [a for a in (_parse_article(x) for x in (articles_in or [])) if a is not None]
    version = str(data.get("version", "")) if isinstance(data, dict) else ""
    logger.info("knowledge_base_loaded", extra={"count": len(articles), "version": version})
    return KnowledgeBase(articles, version=version)


def load_knowledge_base(path: str | None = None) -> KnowledgeBase:
    """Load (and cache) the knowledge base from ``path`` or the repo default."""
    return _load_cached(path or "")
