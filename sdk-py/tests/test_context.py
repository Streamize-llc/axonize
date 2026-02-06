"""Tests for _context module."""

import threading

from axonize._buffer import RingBuffer
from axonize._context import _current_span, get_current_span, set_current_span
from axonize._span import Span


def test_default_is_none() -> None:
    assert get_current_span() is None


def test_set_and_get() -> None:
    s = Span("test", buffer=None)
    token = set_current_span(s)
    assert get_current_span() is s
    _current_span.reset(token)
    assert get_current_span() is None


def test_token_restore() -> None:
    s1 = Span("parent", buffer=None)
    s2 = Span("child", buffer=None)

    token1 = set_current_span(s1)
    assert get_current_span() is s1

    token2 = set_current_span(s2)
    assert get_current_span() is s2

    _current_span.reset(token2)
    assert get_current_span() is s1

    _current_span.reset(token1)
    assert get_current_span() is None


def test_thread_isolation() -> None:
    buf = RingBuffer(maxsize=10)
    s = Span("main-span", buffer=buf)
    token = set_current_span(s)

    seen_in_thread: list[bool] = []

    def check() -> None:
        seen_in_thread.append(get_current_span() is None)

    t = threading.Thread(target=check)
    t.start()
    t.join()

    assert seen_in_thread == [True]
    _current_span.reset(token)
