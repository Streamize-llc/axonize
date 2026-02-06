"""Tests for LLM-specific span (llm_span, record_token, TTFT/TPOT)."""

from __future__ import annotations

import time

from axonize._buffer import RingBuffer
from axonize._llm import LLMSpan
from axonize._types import SpanKind, SpanStatus


def _make_llm_span(
    name: str = "generate",
    model: str | None = "test-model",
    **kwargs: object,
) -> tuple[LLMSpan, RingBuffer]:
    buf = RingBuffer(maxsize=100)
    span = LLMSpan(
        name,
        buffer=buf,
        model=model,  # type: ignore[arg-type]
        **kwargs,  # type: ignore[arg-type]
    )
    return span, buf


class TestLLMSpanBasic:
    def test_model_attribute_set(self) -> None:
        span, buf = _make_llm_span(model="llama-3-70b", model_version="v1")
        with span:
            pass
        data = buf.drain(1)[0]
        assert data.attributes["ai.model.name"] == "llama-3-70b"
        assert data.attributes["ai.model.version"] == "v1"
        assert data.attributes["ai.inference.type"] == "llm"

    def test_default_kind_is_server(self) -> None:
        span, _ = _make_llm_span()
        assert span.kind == SpanKind.SERVER

    def test_no_model_no_attribute(self) -> None:
        span, buf = _make_llm_span(model=None)
        with span:
            pass
        data = buf.drain(1)[0]
        assert "ai.model.name" not in data.attributes

    def test_set_model_after_creation(self) -> None:
        span, buf = _make_llm_span(model=None)
        with span as s:
            s.set_model("gpt-4", version="turbo")
        data = buf.drain(1)[0]
        assert data.attributes["ai.model.name"] == "gpt-4"
        assert data.attributes["ai.model.version"] == "turbo"


class TestTokenRecording:
    def test_record_token_increments(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            s.set_tokens_input(50)
            for _ in range(10):
                s.record_token()
        data = buf.drain(1)[0]
        assert data.attributes["ai.llm.tokens.input"] == 50
        assert data.attributes["ai.llm.tokens.output"] == 10

    def test_manual_set_tokens_output(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            s.set_tokens_input(100)
            s.set_tokens_output(200)
        data = buf.drain(1)[0]
        assert data.attributes["ai.llm.tokens.input"] == 100
        assert data.attributes["ai.llm.tokens.output"] == 200

    def test_zero_tokens_when_none_recorded(self) -> None:
        span, buf = _make_llm_span()
        with span:
            pass
        data = buf.drain(1)[0]
        assert data.attributes["ai.llm.tokens.output"] == 0


class TestTTFT:
    def test_ttft_calculated(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            time.sleep(0.01)  # 10ms before first token
            s.record_token()
            s.record_token()
        data = buf.drain(1)[0]
        ttft = data.attributes["ai.llm.ttft_ms"]
        assert isinstance(ttft, float)
        assert ttft >= 5.0  # At least some delay

    def test_no_ttft_without_tokens(self) -> None:
        span, buf = _make_llm_span()
        with span:
            pass
        data = buf.drain(1)[0]
        assert "ai.llm.ttft_ms" not in data.attributes


class TestTokensPerSecond:
    def test_tps_calculated(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            for _ in range(20):
                s.record_token()
                time.sleep(0.001)  # ~1ms per token
        data = buf.drain(1)[0]
        tps = data.attributes["ai.llm.tokens_per_second"]
        assert isinstance(tps, float)
        assert tps > 0

    def test_no_tps_without_tokens(self) -> None:
        span, buf = _make_llm_span()
        with span:
            pass
        data = buf.drain(1)[0]
        assert "ai.llm.tokens_per_second" not in data.attributes


class TestLLMSpanStatus:
    def test_error_captured(self) -> None:
        span, buf = _make_llm_span()
        try:
            with span as s:
                s.record_token()
                raise RuntimeError("OOM")
        except RuntimeError:
            pass
        data = buf.drain(1)[0]
        assert data.status == SpanStatus.ERROR
        assert data.error_message == "OOM"
        # Tokens still recorded despite error
        assert data.attributes["ai.llm.tokens.output"] == 1

    def test_ok_status_on_success(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            s.record_token()
        data = buf.drain(1)[0]
        assert data.status == SpanStatus.OK


class TestLLMSpanInheritance:
    def test_set_attribute_works(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            s.set_attribute("custom", "value")
        data = buf.drain(1)[0]
        assert data.attributes["custom"] == "value"

    def test_set_gpus_works(self) -> None:
        span, buf = _make_llm_span()
        with span as s:
            s.set_gpus(["cuda:0"])
        data = buf.drain(1)[0]
        # No profiler active, so attributions empty but no error
        assert data.gpu_attributions == []

    def test_inference_type_custom(self) -> None:
        span, buf = _make_llm_span(inference_type="diffusion")
        with span:
            pass
        data = buf.drain(1)[0]
        assert data.attributes["ai.inference.type"] == "diffusion"
