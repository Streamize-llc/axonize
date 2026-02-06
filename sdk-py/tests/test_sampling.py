"""Tests for trace-level sampling."""

from __future__ import annotations

from axonize._buffer import RingBuffer
from axonize._span import Span


def test_sampling_rate_1_keeps_all() -> None:
    """rate=1.0 keeps every span."""
    buf = RingBuffer(maxsize=100)
    for _ in range(20):
        with Span("kept", buffer=buf, sampling_rate=1.0):
            pass
    assert len(buf) == 20


def test_sampling_rate_0_drops_all() -> None:
    """rate=0.0 drops every span."""
    buf = RingBuffer(maxsize=100)
    for _ in range(20):
        with Span("dropped", buffer=buf, sampling_rate=0.0):
            pass
    assert len(buf) == 0


def test_sampling_children_inherit() -> None:
    """Child spans inherit the parent's sampling decision."""
    buf = RingBuffer(maxsize=100)

    # Sampled parent → sampled children
    with Span("parent", buffer=buf, sampling_rate=1.0) as parent:
        assert parent._sampled is True
        with Span("child", buffer=buf, sampling_rate=0.0) as child:
            # Child inherits parent's decision, ignores its own sampling_rate
            assert child._sampled is True

    assert len(buf) == 2

    buf2 = RingBuffer(maxsize=100)
    # Unsampled parent → unsampled children
    with Span("parent", buffer=buf2, sampling_rate=0.0) as parent:
        assert parent._sampled is False
        with Span("child", buffer=buf2, sampling_rate=1.0) as child:
            assert child._sampled is False

    assert len(buf2) == 0


def test_sampling_rate_approximate() -> None:
    """rate=0.5 keeps roughly half the spans (±15%)."""
    buf = RingBuffer(maxsize=2000)
    n = 1000
    for _ in range(n):
        with Span("coin-flip", buffer=buf, sampling_rate=0.5):
            pass
    kept = len(buf)
    # Should be ~500, allow 350-650
    assert 350 <= kept <= 650, f"Expected ~500, got {kept}"
