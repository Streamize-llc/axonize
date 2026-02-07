"""OTLP gRPC exporter — converts SpanData batches to protobuf and ships them."""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING

import grpc
from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import (
    ExportTraceServiceRequest,
)
from opentelemetry.proto.collector.trace.v1.trace_service_pb2_grpc import (
    TraceServiceStub,
)
from opentelemetry.proto.common.v1.common_pb2 import (
    AnyValue,
    InstrumentationScope,
    KeyValue,
)
from opentelemetry.proto.resource.v1.resource_pb2 import Resource
from opentelemetry.proto.trace.v1.trace_pb2 import (
    ResourceSpans,
    ScopeSpans,
)
from opentelemetry.proto.trace.v1.trace_pb2 import (
    Span as OtlpSpan,
)
from opentelemetry.proto.trace.v1.trace_pb2 import (
    Status as OtlpStatus,
)

from axonize._types import SpanKind, SpanStatus

if TYPE_CHECKING:
    from axonize._types import SpanData

logger = logging.getLogger("axonize.exporter")

_KIND_MAP: dict[SpanKind, int] = {
    SpanKind.INTERNAL: OtlpSpan.SPAN_KIND_INTERNAL,
    SpanKind.SERVER: OtlpSpan.SPAN_KIND_SERVER,
    SpanKind.CLIENT: OtlpSpan.SPAN_KIND_CLIENT,
}

_STATUS_MAP: dict[SpanStatus, int] = {
    SpanStatus.UNSET: OtlpStatus.STATUS_CODE_UNSET,
    SpanStatus.OK: OtlpStatus.STATUS_CODE_OK,
    SpanStatus.ERROR: OtlpStatus.STATUS_CODE_ERROR,
}


def _make_attribute(key: str, value: str | int | float | bool) -> KeyValue:
    """Convert a Python key-value pair to an OTLP KeyValue protobuf."""
    if isinstance(value, bool):
        av = AnyValue(bool_value=value)
    elif isinstance(value, int):
        av = AnyValue(int_value=value)
    elif isinstance(value, float):
        av = AnyValue(double_value=value)
    else:
        av = AnyValue(string_value=str(value))
    return KeyValue(key=key, value=av)


def _span_data_to_otlp(sd: SpanData) -> OtlpSpan:
    """Convert a single SpanData to an OTLP Span protobuf."""
    attrs = [_make_attribute(k, v) for k, v in sd.attributes.items()]

    for idx, ga in enumerate(sd.gpu_attributions):
        p = f"gpu.{idx}"
        attrs.extend([
            _make_attribute(f"{p}.resource_uuid", ga.resource_uuid),
            _make_attribute(f"{p}.physical_uuid", ga.physical_gpu_uuid),
            _make_attribute(f"{p}.model", ga.gpu_model),
            _make_attribute(f"{p}.vendor", ga.vendor),
            _make_attribute(f"{p}.node_id", ga.node_id),
            _make_attribute(f"{p}.resource_type", ga.resource_type),
            _make_attribute(f"{p}.user_label", ga.user_label),
            _make_attribute(f"{p}.utilization", ga.utilization),
            _make_attribute(f"{p}.memory_used_gb", ga.memory_used_gb),
            _make_attribute(f"{p}.memory_total_gb", ga.memory_total_gb),
            _make_attribute(f"{p}.temperature_celsius", ga.temperature_celsius),
            _make_attribute(f"{p}.power_watts", ga.power_watts),
            _make_attribute(f"{p}.clock_mhz", ga.clock_mhz),
        ])

    attrs.append(_make_attribute("axonize.duration_ms", sd.duration_ms))

    status = OtlpStatus(code=_STATUS_MAP[sd.status])  # type: ignore[arg-type]
    if sd.status == SpanStatus.ERROR and sd.error_message:
        status = OtlpStatus(code=_STATUS_MAP[sd.status], message=sd.error_message)  # type: ignore[arg-type]

    parent = bytes.fromhex(sd.parent_span_id) if sd.parent_span_id else b""

    return OtlpSpan(
        trace_id=bytes.fromhex(sd.trace_id),
        span_id=bytes.fromhex(sd.span_id),
        parent_span_id=parent,
        name=sd.name,
        kind=_KIND_MAP.get(sd.kind, OtlpSpan.SPAN_KIND_INTERNAL),  # type: ignore[arg-type]
        start_time_unix_nano=sd.start_time_ns,
        end_time_unix_nano=sd.end_time_ns,
        attributes=attrs,
        status=status,
    )


def _build_export_request(
    spans: list[SpanData],
    service_name: str,
    environment: str,
) -> ExportTraceServiceRequest:
    """Build an ExportTraceServiceRequest from a batch of SpanData."""
    resource_attrs = [
        _make_attribute("service.name", service_name),
        _make_attribute("deployment.environment", environment),
        _make_attribute("telemetry.sdk.name", "axonize"),
        _make_attribute("telemetry.sdk.version", "0.1.0"),
    ]

    resource = Resource(attributes=resource_attrs)
    scope = InstrumentationScope(name="axonize", version="0.1.0")

    otlp_spans = [_span_data_to_otlp(sd) for sd in spans]

    scope_spans = ScopeSpans(scope=scope, spans=otlp_spans)
    resource_spans = ResourceSpans(resource=resource, scope_spans=[scope_spans])

    return ExportTraceServiceRequest(resource_spans=[resource_spans])


class OTLPExporter:
    """Exports SpanData batches over gRPC using the OTLP trace protocol.

    Designed as a SpanHandler for BackgroundProcessor. Failures are logged
    but never raised — inference must not be affected by tracing issues.
    """

    def __init__(
        self,
        endpoint: str,
        service_name: str,
        environment: str,
        *,
        insecure: bool = True,
        timeout_s: float = 10.0,
        api_key: str | None = None,
    ) -> None:
        self._service_name = service_name
        self._environment = environment
        self._timeout_s = timeout_s
        self._metadata: list[tuple[str, str]] | None = None
        if api_key is not None:
            self._metadata = [("authorization", f"Bearer {api_key}")]

        if insecure:
            self._channel = grpc.insecure_channel(endpoint)
        else:
            self._channel = grpc.secure_channel(endpoint, grpc.ssl_channel_credentials())

        self._stub = TraceServiceStub(self._channel)  # type: ignore[no-untyped-call]

    def export(self, spans: list[SpanData]) -> None:
        """Export a batch of spans. Logs and swallows all errors."""
        if not spans:
            return
        try:
            request = _build_export_request(
                spans, self._service_name, self._environment
            )
            self._stub.Export(request, timeout=self._timeout_s, metadata=self._metadata)
        except Exception:  # noqa: BLE001
            logger.debug("Failed to export %d spans", len(spans), exc_info=True)

    def shutdown(self) -> None:
        """Close the gRPC channel."""
        try:
            self._channel.close()
        except Exception:  # noqa: BLE001
            pass
