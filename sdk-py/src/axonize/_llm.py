"""LLM-specific span with token recording, TTFT, and TPOT calculation."""

from __future__ import annotations

import time
from types import TracebackType
from typing import TYPE_CHECKING

from axonize._span import Span
from axonize._types import SpanKind

if TYPE_CHECKING:
    from axonize._buffer import RingBuffer


class LLMSpan(Span):
    """Span specialized for LLM inference with streaming token support.

    Extends Span with:
      - ``record_token()`` for tracking output tokens during streaming
      - Automatic TTFT (Time To First Token) calculation
      - Automatic TPOT (Time Per Output Token) calculation
      - ``tokens_per_second`` derivation
      - Model name/version convenience setters

    Usage::

        with axonize.llm_span("generate", model="llama-3-70b") as s:
            s.set_tokens_input(128)
            for token in model.generate_stream(prompt):
                s.record_token()
                yield token
    """

    def __init__(
        self,
        name: str,
        *,
        buffer: RingBuffer | None,
        kind: SpanKind = SpanKind.SERVER,
        service_name: str = "",
        environment: str = "development",
        model: str | None = None,
        model_version: str | None = None,
        inference_type: str = "llm",
        sampling_rate: float = 1.0,
    ) -> None:
        super().__init__(
            name,
            buffer=buffer,
            kind=kind,
            service_name=service_name,
            environment=environment,
            sampling_rate=sampling_rate,
        )
        self._tokens_input: int = 0
        self._tokens_output: int = 0
        self._first_token_ns: int = 0
        self._last_token_ns: int = 0

        if model is not None:
            self._attributes["ai.model.name"] = model
        if model_version is not None:
            self._attributes["ai.model.version"] = model_version
        self._attributes["ai.inference.type"] = inference_type

    def set_tokens_input(self, count: int) -> None:
        """Set the number of input (prompt) tokens."""
        self._tokens_input = count
        self._attributes["ai.llm.tokens.input"] = count

    def set_tokens_output(self, count: int) -> None:
        """Manually set output token count (alternative to record_token())."""
        self._tokens_output = count
        self._attributes["ai.llm.tokens.output"] = count

    def record_token(self) -> None:
        """Record a single output token emission.

        Call this once per generated token during streaming. Automatically
        tracks TTFT (from span start to first token) and token count.
        """
        now = time.time_ns()
        self._tokens_output += 1
        if self._first_token_ns == 0:
            self._first_token_ns = now
        self._last_token_ns = now

    def set_model(self, name: str, version: str | None = None) -> None:
        """Set model name and optional version."""
        self._attributes["ai.model.name"] = name
        if version is not None:
            self._attributes["ai.model.version"] = version

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ) -> None:
        # Finalize token metrics before parent __exit__ builds SpanData
        self._attributes["ai.llm.tokens.output"] = self._tokens_output
        if self._tokens_input > 0:
            self._attributes["ai.llm.tokens.input"] = self._tokens_input

        # TTFT: time from span start to first token (ms)
        if self._first_token_ns > 0 and self._start_time_ns > 0:
            ttft_ms = (self._first_token_ns - self._start_time_ns) / 1_000_000
            self._attributes["ai.llm.ttft_ms"] = round(ttft_ms, 3)

        # Tokens per second: output tokens / generation duration
        end_ns = time.time_ns()
        if self._tokens_output > 0 and self._first_token_ns > 0:
            gen_duration_s = (end_ns - self._first_token_ns) / 1_000_000_000
            if gen_duration_s > 0:
                tps = self._tokens_output / gen_duration_s
                self._attributes["ai.llm.tokens_per_second"] = round(tps, 2)

        super().__exit__(exc_type, exc_val, exc_tb)
