"""Context propagation for span parent-child relationships."""

from __future__ import annotations

from contextvars import ContextVar, Token
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from axonize._span import Span

_current_span: ContextVar[Span | None] = ContextVar("_current_span", default=None)


def get_current_span() -> Span | None:
    """Return the active span in the current context, or None."""
    return _current_span.get()


def set_current_span(span: Span | None) -> Token[Span | None]:
    """Set the active span and return a token for later restoration."""
    return _current_span.set(span)
