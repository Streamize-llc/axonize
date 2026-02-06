"""@trace decorator for wrapping functions in spans."""

from __future__ import annotations

import functools
from collections.abc import Callable
from typing import Any, TypeVar, overload

from axonize._types import SpanKind

F = TypeVar("F", bound=Callable[..., Any])


@overload
def trace(func: F) -> F: ...


@overload
def trace(
    *,
    name: str | None = None,
    kind: SpanKind = SpanKind.SERVER,
) -> Callable[[F], F]: ...


def trace(
    func: F | None = None,
    *,
    name: str | None = None,
    kind: SpanKind = SpanKind.SERVER,
) -> F | Callable[[F], F]:
    """Decorator that wraps a function call in a span.

    Can be used with or without arguments::

        @trace
        def handle_request(): ...

        @trace(name="custom", kind=SpanKind.CLIENT)
        def call_service(): ...
    """

    def decorator(fn: F) -> F:
        span_name = name or fn.__qualname__

        @functools.wraps(fn)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            from axonize._sdk import _get_sdk

            sdk = _get_sdk()
            with sdk.create_span(span_name, kind=kind):
                return fn(*args, **kwargs)

        return wrapper  # type: ignore[return-value]

    if func is not None:
        return decorator(func)
    return decorator
