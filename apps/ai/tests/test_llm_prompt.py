"""Tests for the versioned system prompt loader."""

from __future__ import annotations

from app.config import Settings
from app.llm.prompt import PROMPT_VERSION, build_system_prompt


def test_prompt_includes_preamble_hard_constraints():
    prompt = build_system_prompt(Settings())
    # The operational preamble must always be present, even if docs are missing.
    assert "source of truth" in prompt.lower()
    assert "never reveal" in prompt.lower()


def test_prompt_loads_docs_when_available():
    # Defaults resolve to docs/ai/*.md in the repo; those files exist.
    prompt = build_system_prompt(Settings())
    assert "System Prompt" in prompt
    assert "Sales Rules" in prompt


def test_prompt_override_path_missing_is_tolerated(tmp_path):
    missing = str(tmp_path / "nope.md")
    prompt = build_system_prompt(
        Settings(llm_system_prompt_path=missing, llm_sales_rules_path=missing)
    )
    # Missing docs → only the preamble, but still a non-empty prompt.
    assert prompt
    assert "source of truth" in prompt.lower()


def test_prompt_version_is_stable_string():
    assert isinstance(PROMPT_VERSION, str) and PROMPT_VERSION
