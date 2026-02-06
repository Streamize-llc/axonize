"""OpenAI auto-instrumentation — wraps an OpenAI client for automatic span creation.

Usage::

    import openai
    from axonize.integrations.openai import instrument

    client = openai.OpenAI()
    client = instrument(client)

    # All chat.completions.create() calls now produce axonize spans automatically.
    response = client.chat.completions.create(model="gpt-4", messages=[...])
"""

from __future__ import annotations

import sys
from collections.abc import Iterator
from typing import Any


def instrument(client: Any) -> Any:
    """Wrap an OpenAI client so every API call produces an axonize span.

    Returns the same client object with ``chat.completions`` wrapped.
    The original client is not modified — a thin wrapper is installed.

    Raises :class:`ImportError` if the ``openai`` package is not installed.
    """
    if "openai" not in sys.modules:
        try:
            import openai as _  # noqa: F401
        except ImportError:
            msg = (
                "openai is required for this integration. "
                "Install it with: pip install axonize[openai]"
            )
            raise ImportError(msg) from None

    client.chat.completions = _InstrumentedCompletions(client.chat.completions)
    return client


class _InstrumentedCompletions:
    """Wraps ``client.chat.completions`` to auto-create LLM spans."""

    def __init__(self, original: Any) -> None:
        self._original = original

    def __getattr__(self, name: str) -> Any:
        return getattr(self._original, name)

    def create(self, **kwargs: Any) -> Any:
        """Wrap ``chat.completions.create`` with an axonize LLM span."""
        model: str = kwargs.get("model", "unknown")
        stream: bool = kwargs.get("stream", False)

        if stream:
            return self._create_streaming(model=model, kwargs=kwargs)
        return self._create_non_streaming(model=model, kwargs=kwargs)

    def _create_non_streaming(self, *, model: str, kwargs: Any) -> Any:
        import axonize

        span = axonize.llm_span("openai.chat.completions.create", model=model)
        span.__enter__()
        try:
            response = self._original.create(**kwargs)
        except BaseException as exc:
            span.__exit__(type(exc), exc, exc.__traceback__)
            raise

        # Extract token usage from response
        usage = getattr(response, "usage", None)
        if usage is not None:
            prompt_tokens = getattr(usage, "prompt_tokens", 0) or 0
            completion_tokens = getattr(usage, "completion_tokens", 0) or 0
            span.set_tokens_input(prompt_tokens)
            span.set_tokens_output(completion_tokens)

        span.__exit__(None, None, None)
        return response

    def _create_streaming(self, *, model: str, kwargs: Any) -> Iterator[Any]:
        import axonize

        span = axonize.llm_span("openai.chat.completions.create", model=model)
        span.__enter__()
        try:
            stream = self._original.create(**kwargs)
            yield from self._iterate_stream(stream, span)
        except BaseException as exc:
            span.__exit__(type(exc), exc, exc.__traceback__)
            raise

    @staticmethod
    def _iterate_stream(stream: Any, span: Any) -> Iterator[Any]:
        try:
            for chunk in stream:
                choices = getattr(chunk, "choices", [])
                if choices:
                    delta = getattr(choices[0], "delta", None)
                    content = getattr(delta, "content", None) if delta else None
                    if content:
                        span.record_token()
                yield chunk
        except BaseException as exc:
            span.__exit__(type(exc), exc, exc.__traceback__)
            raise
        else:
            span.__exit__(None, None, None)
