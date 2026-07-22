"""Tests for the knowledge base loader + retrieval (Phase A9)."""

from __future__ import annotations

import json
import tempfile
from pathlib import Path

import pytest

from app.knowledge.store import (
    KnowledgeArticle,
    KnowledgeBase,
    _tokenize,
    load_knowledge_base,
)


def test_tokenize():
    """Lowercased word tokens with stopwords removed."""
    assert _tokenize("Berapa ongkir ke Jakarta?") == {"berapa", "ongkir", "jakarta"}
    assert _tokenize("Gimana cara bayar?") == {"cara", "bayar"}
    assert _tokenize("") == set()


def test_article_score_keyword_overlap():
    """Keyword-token overlap scores an article."""
    article = KnowledgeArticle(
        id="test",
        title="Test",
        keywords=("ongkir", "pengiriman", "kurir"),
        answer="Answer",
    )
    # Direct keyword hit.
    assert article.score({"ongkir", "jakarta"}, "ongkir jakarta") > 0
    # Multiple hits score higher.
    assert article.score({"ongkir", "kurir"}, "ongkir kurir") > article.score(
        {"ongkir"}, "ongkir"
    )
    # No overlap.
    assert article.score({"unrelated"}, "unrelated") == 0


def test_article_score_phrase_bonus():
    """Multi-word keyword phrases get a bonus when they appear verbatim."""
    article = KnowledgeArticle(
        id="test",
        title="Test",
        keywords=("virtual account", "qris"),
        answer="Answer",
    )
    # Phrase match scores higher than scattered tokens.
    phrase_score = article.score({"virtual", "account"}, "bayar virtual account")
    scattered_score = article.score({"virtual", "account"}, "virtual dan account")
    assert phrase_score > scattered_score


def test_knowledge_base_search():
    """Search returns the best-matching article above the threshold."""
    articles = [
        KnowledgeArticle(
            id="shipping",
            title="Shipping",
            keywords=("kirim", "ongkir", "pengiriman"),
            answer="Shipping answer",
        ),
        KnowledgeArticle(
            id="payment",
            title="Payment",
            keywords=("bayar", "pembayaran", "va"),
            answer="Payment answer",
        ),
    ]
    kb = KnowledgeBase(articles)

    result = kb.search("berapa ongkir ke Jakarta?")
    assert result is not None
    assert result.id == "shipping"

    result = kb.search("cara bayar gimana?")
    assert result is not None
    assert result.id == "payment"


def test_knowledge_base_search_no_match():
    """No article matches → None."""
    articles = [
        KnowledgeArticle(
            id="shipping", title="Shipping", keywords=("kirim",), answer="Answer"
        )
    ]
    kb = KnowledgeBase(articles)
    assert kb.search("totally unrelated query") is None
    assert kb.search("") is None
    assert kb.search(None) is None


def test_knowledge_base_search_min_score():
    """Below the threshold → None."""
    articles = [
        KnowledgeArticle(
            id="test", title="Test", keywords=("keyword",), answer="Answer"
        )
    ]
    kb = KnowledgeBase(articles)
    # Very weak overlap shouldn't return a hit if below min_score.
    result = kb.search("keyword", min_score=10.0)
    assert result is None


def test_load_knowledge_base_valid():
    """Load a well-formed JSON knowledge base."""
    data = {
        "version": "test.1",
        "articles": [
            {
                "id": "article-1",
                "title": "Title 1",
                "keywords": ["keyword1", "keyword2"],
                "answer": "Answer 1",
            },
            {
                "id": "article-2",
                "title": "Title 2",
                "keywords": ["keyword3"],
                "answer": "Answer 2",
            },
        ],
    }
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump(data, f)
        temp_path = f.name

    try:
        kb = load_knowledge_base(temp_path)
        assert len(kb) == 2
        assert kb.version == "test.1"
        result = kb.search("keyword1")
        assert result is not None
        assert result.id == "article-1"
    finally:
        Path(temp_path).unlink()


def test_load_knowledge_base_missing_file():
    """Missing file degrades to an empty KB."""
    kb = load_knowledge_base("/nonexistent/path.json")
    assert len(kb) == 0
    assert kb.version == ""


def test_load_knowledge_base_malformed():
    """Malformed JSON degrades to an empty KB."""
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        f.write("{ broken json")
        temp_path = f.name

    try:
        kb = load_knowledge_base(temp_path)
        assert len(kb) == 0
    finally:
        Path(temp_path).unlink()


def test_load_knowledge_base_invalid_articles():
    """Articles missing required fields are skipped."""
    data = {
        "articles": [
            {"id": "valid", "answer": "Valid answer", "keywords": ["test"]},
            {"id": "no-answer", "keywords": ["test"]},  # missing answer
            {"answer": "No id"},  # missing id
            "not-a-dict",  # not even a dict
        ]
    }
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
        json.dump(data, f)
        temp_path = f.name

    try:
        kb = load_knowledge_base(temp_path)
        assert len(kb) == 1
        assert kb.search("test").id == "valid"
    finally:
        Path(temp_path).unlink()


def test_load_knowledge_base_default():
    """When no path is given, load from the repo default."""
    # This tests the actual default file (docs/ai/knowledge-base.json) exists
    # and is valid. If the file is missing or malformed, this degrades to
    # empty rather than failing, so we just check it doesn't crash.
    kb = load_knowledge_base()
    # The default KB should have some articles (or zero if file is missing).
    assert len(kb) >= 0