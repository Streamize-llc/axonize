"""SDK singleton — orchestrates config, buffer, and processor lifecycle."""

from __future__ import annotations

import atexit

from axonize._buffer import RingBuffer
from axonize._config import AxonizeConfig
from axonize._exporter import OTLPExporter
from axonize._gpu import GPUProfiler, MockGPUProfiler, create_gpu_profiler
from axonize._processor import BackgroundProcessor
from axonize._span import Span
from axonize._types import SpanKind

_sdk_instance: _AxonizeSDK | None = None


class _AxonizeSDK:
    """Internal SDK singleton. Not part of the public API."""

    def __init__(self, config: AxonizeConfig) -> None:
        self.config = config
        self._buffer: RingBuffer | None = RingBuffer(config.buffer_size)
        self._processor: BackgroundProcessor | None = None
        self._exporter: OTLPExporter | None = None
        self._gpu_profiler: GPUProfiler | MockGPUProfiler | None = None

    def start(self) -> None:
        """Start the background processor with the OTLP exporter."""
        if self._buffer is None:
            return
        self._exporter = OTLPExporter(
            endpoint=self.config.endpoint,
            service_name=self.config.service_name,
            environment=self.config.environment,
        )
        self._processor = BackgroundProcessor(
            self._buffer,
            batch_size=self.config.batch_size,
            flush_interval_ms=self.config.flush_interval_ms,
            handler=self._exporter.export,
        )
        self._processor.start()

        if self.config.gpu_profiling:
            self._gpu_profiler = create_gpu_profiler(
                snapshot_interval_ms=self.config.gpu_snapshot_interval_ms,
            )
            if self._gpu_profiler is not None:
                self._gpu_profiler.start()

    def shutdown(self) -> None:
        """Stop processor and release resources."""
        if self._gpu_profiler is not None:
            self._gpu_profiler.stop()
            self._gpu_profiler = None
        if self._processor is not None:
            self._processor.stop()
            self._processor = None
        if self._exporter is not None:
            self._exporter.shutdown()
            self._exporter = None
        self._buffer = None

    def create_span(
        self,
        name: str,
        *,
        kind: SpanKind = SpanKind.INTERNAL,
    ) -> Span:
        """Create a new span wired to the internal buffer."""
        return Span(
            name,
            buffer=self._buffer,
            kind=kind,
            service_name=self.config.service_name,
            environment=self.config.environment,
        )


class _NoopSDK:
    """Fallback used when SDK is not initialized. Spans are silently discarded."""

    def create_span(
        self,
        name: str,
        *,
        kind: SpanKind = SpanKind.INTERNAL,
    ) -> Span:
        return Span(name, buffer=None, kind=kind)


_noop = _NoopSDK()


def _get_sdk() -> _AxonizeSDK | _NoopSDK:
    """Return the active SDK or a noop fallback."""
    if _sdk_instance is not None:
        return _sdk_instance
    return _noop


def init(
    *,
    endpoint: str,
    service_name: str,
    environment: str = "development",
    batch_size: int = 512,
    flush_interval_ms: int = 5000,
    buffer_size: int = 8192,
    sampling_rate: float = 1.0,
    gpu_profiling: bool = False,
) -> None:
    """Initialize the Axonize SDK.

    Must be called before creating any spans or traces.
    """
    global _sdk_instance  # noqa: PLW0603

    if _sdk_instance is not None:
        _sdk_instance.shutdown()

    config = AxonizeConfig(
        endpoint=endpoint,
        service_name=service_name,
        environment=environment,
        batch_size=batch_size,
        flush_interval_ms=flush_interval_ms,
        buffer_size=buffer_size,
        sampling_rate=sampling_rate,
        gpu_profiling=gpu_profiling,
    )
    _sdk_instance = _AxonizeSDK(config)
    _sdk_instance.start()
    atexit.register(shutdown)


def shutdown() -> None:
    """Shut down the SDK, flushing any remaining spans."""
    global _sdk_instance  # noqa: PLW0603
    if _sdk_instance is not None:
        _sdk_instance.shutdown()
        _sdk_instance = None
