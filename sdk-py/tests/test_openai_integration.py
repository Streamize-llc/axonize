"""Tests for OpenAI auto-instrumentation (uses mocks — no real API calls)."""

from __future__ import annotations

import sys
from dataclasses import dataclass
from typing import Any
from unittest.mock import MagicMock

import pytest

import axonize
import axonize._sdk as sdk_mod
from axonize.integrations.openai import instrument


@pytest.fixture(autouse=True)
def _sdk_lifecycle() -> Any:
    """Init SDK before each test, shut down after."""
    sdk_mod._sdk_instance = None
    axonize.init(endpoint="http://localhost:4317", service_name="openai-test")
    yield
    axonize.shutdown()


@pytest.fixture(autouse=True)
def _fake_openai_module(monkeypatch: Any) -> Any:
    """Inject a fake 'openai' entry into sys.modules so instrument() passes the check."""
    fake = MagicMock()
    monkeypatch.setitem(sys.modules, "openai", fake)
    yield


# --- Mock helpers ---


@dataclass
class _MockUsage:
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int


@dataclass
class _MockResponse:
    id: str = "chatcmpl-mock"
    model: str = "gpt-4"
    usage: _MockUsage | None = None


@dataclass
class _MockDelta:
    content: str | None = None


@dataclass
class _MockChoice:
    delta: _MockDelta
    index: int = 0


@dataclass
class _MockChunk:
    choices: list[_MockChoice]


def _make_mock_client(response: Any = None, stream_chunks: list[Any] | None = None) -> Any:
    """Build a mock OpenAI client."""
    client = MagicMock()

    def _create(**kwargs: Any) -> Any:
        if kwargs.get("stream"):
            if stream_chunks is not None:
                return iter(stream_chunks)
            return iter([])
        return response

    client.chat.completions.create = _create
    return client


# --- Tests ---


def test_instrument_non_streaming() -> None:
    resp = _MockResponse(
        usage=_MockUsage(prompt_tokens=100, completion_tokens=50, total_tokens=150),
    )
    client = _make_mock_client(response=resp)
    client = instrument(client)

    result = client.chat.completions.create(model="gpt-4", messages=[])

    assert result.usage is not None
    assert result.usage.prompt_tokens == 100
    assert result.usage.completion_tokens == 50

    # Verify span was enqueued
    assert sdk_mod._sdk_instance is not None
    buf = sdk_mod._sdk_instance._buffer
    assert buf is not None
    items = buf.drain(10)
    assert len(items) == 1
    sd = items[0]
    assert sd.name == "openai.chat.completions.create"
    assert sd.attributes["ai.model.name"] == "gpt-4"
    assert sd.attributes["ai.llm.tokens.input"] == 100
    assert sd.attributes["ai.llm.tokens.output"] == 50


def test_instrument_streaming() -> None:
    chunks = [
        _MockChunk(choices=[_MockChoice(delta=_MockDelta(content="Hello"))]),
        _MockChunk(choices=[_MockChoice(delta=_MockDelta(content=" world"))]),
        _MockChunk(choices=[_MockChoice(delta=_MockDelta(content="!"))]),
    ]
    client = _make_mock_client(stream_chunks=chunks)
    client = instrument(client)

    collected: list[Any] = []
    for chunk in client.chat.completions.create(model="gpt-4", messages=[], stream=True):
        collected.append(chunk)

    assert len(collected) == 3

    # Verify span was enqueued with token records
    assert sdk_mod._sdk_instance is not None
    buf = sdk_mod._sdk_instance._buffer
    assert buf is not None
    items = buf.drain(10)
    assert len(items) == 1
    sd = items[0]
    assert sd.name == "openai.chat.completions.create"
    assert sd.attributes["ai.llm.tokens.output"] == 3
    assert "ai.llm.ttft_ms" in sd.attributes


def test_instrument_error_handling() -> None:
    """API error should still produce a span with ERROR status."""
    client = MagicMock()

    def _create(**kwargs: Any) -> Any:
        raise RuntimeError("API error")

    client.chat.completions.create = _create
    client = instrument(client)

    caught = False
    try:
        client.chat.completions.create(model="gpt-4", messages=[])
    except RuntimeError:
        caught = True

    assert caught

    assert sdk_mod._sdk_instance is not None
    buf = sdk_mod._sdk_instance._buffer
    assert buf is not None
    items = buf.drain(10)
    assert len(items) == 1
    sd = items[0]
    assert sd.name == "openai.chat.completions.create"
    assert sd.error_message == "API error"


def test_instrument_without_openai(monkeypatch: Any) -> None:
    """instrument() should raise ImportError when openai is not installed."""
    import builtins

    # Remove fake openai from sys.modules
    monkeypatch.delitem(sys.modules, "openai", raising=False)

    real_import = builtins.__import__

    def _mock_import(name: str, *args: Any, **kwargs: Any) -> Any:
        if name == "openai" or name.startswith("openai."):
            raise ImportError("No module named 'openai'")
        return real_import(name, *args, **kwargs)

    monkeypatch.setattr(builtins, "__import__", _mock_import)

    with pytest.raises(ImportError, match="openai is required"):
        instrument(MagicMock())


def test_instrument_passthrough_attributes() -> None:
    """Non-create methods should pass through to the original client."""
    client = _make_mock_client(response=_MockResponse())
    original_completions = client.chat.completions
    client = instrument(client)

    # Access a non-create attribute — should delegate to original
    original_completions.some_method = lambda: "pass"
    assert client.chat.completions.some_method() == "pass"
