"""Tests for the OTLP gRPC exporter."""

from __future__ import annotations

import threading
from concurrent import futures

import grpc
import pytest
from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import (
    ExportTraceServiceRequest,
    ExportTraceServiceResponse,
)
from opentelemetry.proto.collector.trace.v1.trace_service_pb2_grpc import (
    TraceServiceServicer,
    add_TraceServiceServicer_to_server,
)
from opentelemetry.proto.trace.v1.trace_pb2 import Span as OtlpSpan
from opentelemetry.proto.trace.v1.trace_pb2 import Status as OtlpStatus

from axonize._exporter import (
    OTLPExporter,
    _build_export_request,
    _make_attribute,
    _span_data_to_otlp,
)
from axonize._types import GPUAttribution, SpanData, SpanKind, SpanStatus


def _make_span_data(**overrides: object) -> SpanData:
    """Create a SpanData with sensible defaults."""
    defaults: dict[str, object] = {
        "span_id": "abcdef0123456789",
        "trace_id": "0123456789abcdef0123456789abcdef",
        "name": "test-span",
        "kind": SpanKind.INTERNAL,
        "status": SpanStatus.OK,
        "start_time_ns": 1_000_000_000,
        "end_time_ns": 2_000_000_000,
        "duration_ms": 1000.0,
        "service_name": "test-service",
        "environment": "test",
        "attributes": {},
        "parent_span_id": None,
        "gpu_attributions": [],
        "error_message": None,
    }
    defaults.update(overrides)
    return SpanData(**defaults)  # type: ignore[arg-type]


def _make_gpu_attribution(**overrides: object) -> GPUAttribution:
    """Create a GPUAttribution with sensible defaults."""
    defaults: dict[str, object] = {
        "resource_uuid": "GPU-0000",
        "physical_gpu_uuid": "GPU-0000",
        "gpu_model": "NVIDIA H100 80GB HBM3",
        "vendor": "NVIDIA",
        "node_id": "worker-01",
        "resource_type": "full_gpu",
        "user_label": "cuda:0",
        "memory_used_gb": 42.5,
        "memory_total_gb": 80.0,
        "utilization": 85.2,
        "temperature_celsius": 72,
        "power_watts": 350,
        "clock_mhz": 1800,
    }
    defaults.update(overrides)
    return GPUAttribution(**defaults)  # type: ignore[arg-type]


class TestMakeAttribute:
    def test_string(self) -> None:
        kv = _make_attribute("key", "value")
        assert kv.key == "key"
        assert kv.value.string_value == "value"

    def test_int(self) -> None:
        kv = _make_attribute("key", 42)
        assert kv.value.int_value == 42

    def test_float(self) -> None:
        kv = _make_attribute("key", 3.14)
        assert kv.value.double_value == pytest.approx(3.14)

    def test_bool_true(self) -> None:
        kv = _make_attribute("key", True)
        assert kv.value.bool_value is True

    def test_bool_false(self) -> None:
        kv = _make_attribute("key", False)
        assert kv.value.bool_value is False

    def test_bool_is_not_int(self) -> None:
        """bool is subclass of int — ensure it's handled as bool, not int."""
        kv_true = _make_attribute("key", True)
        kv_one = _make_attribute("key", 1)
        assert kv_true.value.HasField("bool_value")
        assert kv_one.value.HasField("int_value")


class TestSpanDataToOtlp:
    def test_basic_fields(self) -> None:
        sd = _make_span_data()
        otlp = _span_data_to_otlp(sd)
        assert otlp.name == "test-span"
        assert otlp.start_time_unix_nano == 1_000_000_000
        assert otlp.end_time_unix_nano == 2_000_000_000

    def test_trace_id_bytes(self) -> None:
        sd = _make_span_data(trace_id="0123456789abcdef0123456789abcdef")
        otlp = _span_data_to_otlp(sd)
        assert otlp.trace_id == bytes.fromhex("0123456789abcdef0123456789abcdef")
        assert len(otlp.trace_id) == 16

    def test_span_id_bytes(self) -> None:
        sd = _make_span_data(span_id="abcdef0123456789")
        otlp = _span_data_to_otlp(sd)
        assert otlp.span_id == bytes.fromhex("abcdef0123456789")
        assert len(otlp.span_id) == 8

    def test_parent_span_id_present(self) -> None:
        sd = _make_span_data(parent_span_id="1234567890abcdef")
        otlp = _span_data_to_otlp(sd)
        assert otlp.parent_span_id == bytes.fromhex("1234567890abcdef")

    def test_parent_span_id_absent(self) -> None:
        sd = _make_span_data(parent_span_id=None)
        otlp = _span_data_to_otlp(sd)
        assert otlp.parent_span_id == b""

    def test_span_kind_internal(self) -> None:
        sd = _make_span_data(kind=SpanKind.INTERNAL)
        otlp = _span_data_to_otlp(sd)
        assert otlp.kind == OtlpSpan.SPAN_KIND_INTERNAL

    def test_span_kind_server(self) -> None:
        sd = _make_span_data(kind=SpanKind.SERVER)
        otlp = _span_data_to_otlp(sd)
        assert otlp.kind == OtlpSpan.SPAN_KIND_SERVER

    def test_span_kind_client(self) -> None:
        sd = _make_span_data(kind=SpanKind.CLIENT)
        otlp = _span_data_to_otlp(sd)
        assert otlp.kind == OtlpSpan.SPAN_KIND_CLIENT

    def test_status_ok(self) -> None:
        sd = _make_span_data(status=SpanStatus.OK)
        otlp = _span_data_to_otlp(sd)
        assert otlp.status.code == OtlpStatus.STATUS_CODE_OK

    def test_status_unset(self) -> None:
        sd = _make_span_data(status=SpanStatus.UNSET)
        otlp = _span_data_to_otlp(sd)
        assert otlp.status.code == OtlpStatus.STATUS_CODE_UNSET

    def test_status_error_with_message(self) -> None:
        sd = _make_span_data(status=SpanStatus.ERROR, error_message="boom")
        otlp = _span_data_to_otlp(sd)
        assert otlp.status.code == OtlpStatus.STATUS_CODE_ERROR
        assert otlp.status.message == "boom"

    def test_status_error_without_message(self) -> None:
        sd = _make_span_data(status=SpanStatus.ERROR, error_message=None)
        otlp = _span_data_to_otlp(sd)
        assert otlp.status.code == OtlpStatus.STATUS_CODE_ERROR

    def test_attributes(self) -> None:
        sd = _make_span_data(attributes={"model": "gpt-4", "tokens": 100})
        otlp = _span_data_to_otlp(sd)
        attr_dict = {a.key: a.value for a in otlp.attributes}
        assert attr_dict["model"].string_value == "gpt-4"
        assert attr_dict["tokens"].int_value == 100

    def test_gpu_attributions_single(self) -> None:
        ga = _make_gpu_attribution()
        sd = _make_span_data(gpu_attributions=[ga])
        otlp = _span_data_to_otlp(sd)
        attr_dict = {a.key: a.value for a in otlp.attributes}
        assert attr_dict["gpu.0.resource_uuid"].string_value == "GPU-0000"
        assert attr_dict["gpu.0.physical_uuid"].string_value == "GPU-0000"
        assert attr_dict["gpu.0.model"].string_value == "NVIDIA H100 80GB HBM3"
        assert attr_dict["gpu.0.vendor"].string_value == "NVIDIA"
        assert attr_dict["gpu.0.node_id"].string_value == "worker-01"
        assert attr_dict["gpu.0.resource_type"].string_value == "full_gpu"
        assert attr_dict["gpu.0.user_label"].string_value == "cuda:0"
        assert attr_dict["gpu.0.utilization"].double_value == pytest.approx(85.2)
        assert attr_dict["gpu.0.memory_used_gb"].double_value == pytest.approx(42.5)
        assert attr_dict["gpu.0.memory_total_gb"].double_value == pytest.approx(80.0)
        assert attr_dict["gpu.0.temperature_celsius"].int_value == 72
        assert attr_dict["gpu.0.power_watts"].int_value == 350
        assert attr_dict["gpu.0.clock_mhz"].int_value == 1800

    def test_gpu_attributions_multiple(self) -> None:
        ga0 = _make_gpu_attribution(user_label="cuda:0", resource_uuid="GPU-0000")
        ga1 = _make_gpu_attribution(
            user_label="cuda:1",
            resource_uuid="GPU-0001",
            physical_gpu_uuid="GPU-0001",
            utilization=92.0,
        )
        sd = _make_span_data(gpu_attributions=[ga0, ga1])
        otlp = _span_data_to_otlp(sd)
        attr_dict = {a.key: a.value for a in otlp.attributes}
        assert attr_dict["gpu.0.user_label"].string_value == "cuda:0"
        assert attr_dict["gpu.1.user_label"].string_value == "cuda:1"
        assert attr_dict["gpu.1.utilization"].double_value == pytest.approx(92.0)

    def test_no_gpu_attributions(self) -> None:
        sd = _make_span_data(gpu_attributions=[])
        otlp = _span_data_to_otlp(sd)
        attr_keys = {a.key for a in otlp.attributes}
        assert not any(k.startswith("gpu.") for k in attr_keys)

    def test_duration_ms_attribute(self) -> None:
        sd = _make_span_data(duration_ms=42.5)
        otlp = _span_data_to_otlp(sd)
        attr_dict = {a.key: a.value for a in otlp.attributes}
        assert attr_dict["axonize.duration_ms"].double_value == pytest.approx(42.5)


class TestBuildExportRequest:
    def test_single_resource_spans(self) -> None:
        spans = [_make_span_data(), _make_span_data(name="second")]
        req = _build_export_request(spans, "my-svc", "prod")
        assert len(req.resource_spans) == 1

    def test_resource_attributes(self) -> None:
        spans = [_make_span_data()]
        req = _build_export_request(spans, "my-svc", "prod")
        resource = req.resource_spans[0].resource
        attr_dict = {a.key: a.value.string_value for a in resource.attributes}
        assert attr_dict["service.name"] == "my-svc"
        assert attr_dict["deployment.environment"] == "prod"
        assert attr_dict["telemetry.sdk.name"] == "axonize"
        assert attr_dict["telemetry.sdk.version"] == "0.1.0"

    def test_scope_info(self) -> None:
        spans = [_make_span_data()]
        req = _build_export_request(spans, "svc", "dev")
        scope = req.resource_spans[0].scope_spans[0].scope
        assert scope.name == "axonize"
        assert scope.version == "0.1.0"

    def test_all_spans_in_batch(self) -> None:
        spans = [_make_span_data(name=f"span-{i}") for i in range(5)]
        req = _build_export_request(spans, "svc", "dev")
        otlp_spans = req.resource_spans[0].scope_spans[0].spans
        assert len(otlp_spans) == 5
        names = {s.name for s in otlp_spans}
        assert names == {f"span-{i}" for i in range(5)}


class TestOTLPExporter:
    def test_export_empty_batch(self) -> None:
        """Exporting an empty list should be a no-op."""
        exporter = OTLPExporter("localhost:4317", "svc", "dev")
        exporter.export([])  # Should not raise
        exporter.shutdown()

    def test_graceful_failure_bad_endpoint(self) -> None:
        """Export to unreachable endpoint should not raise."""
        exporter = OTLPExporter(
            "localhost:1",  # unlikely to be listening
            "svc",
            "dev",
            timeout_s=0.1,
        )
        sd = _make_span_data()
        exporter.export([sd])  # Must not raise
        exporter.shutdown()

    def test_shutdown_idempotent(self) -> None:
        exporter = OTLPExporter("localhost:4317", "svc", "dev")
        exporter.shutdown()
        exporter.shutdown()  # Should not raise


class _CollectorServicer(TraceServiceServicer):
    """In-process gRPC servicer that collects ExportTraceServiceRequests."""

    def __init__(self) -> None:
        self.requests: list[ExportTraceServiceRequest] = []
        self._lock = threading.Lock()

    def Export(  # noqa: N802
        self,
        request: ExportTraceServiceRequest,
        context: grpc.ServicerContext,
    ) -> ExportTraceServiceResponse:
        with self._lock:
            self.requests.append(request)
        return ExportTraceServiceResponse()


class TestWithMockGrpcServer:
    """Integration test with an in-process gRPC server."""

    def test_export_received_by_server(self) -> None:
        servicer = _CollectorServicer()
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        add_TraceServiceServicer_to_server(servicer, server)
        port = server.add_insecure_port("localhost:0")
        server.start()

        try:
            exporter = OTLPExporter(
                f"localhost:{port}", "test-svc", "staging", timeout_s=5.0
            )
            spans = [
                _make_span_data(name="op-1"),
                _make_span_data(name="op-2", span_id="1234567890abcdef"),
            ]
            exporter.export(spans)
            exporter.shutdown()

            assert len(servicer.requests) == 1
            req = servicer.requests[0]
            assert len(req.resource_spans) == 1

            resource = req.resource_spans[0].resource
            attr_dict = {a.key: a.value.string_value for a in resource.attributes}
            assert attr_dict["service.name"] == "test-svc"
            assert attr_dict["deployment.environment"] == "staging"

            otlp_spans = req.resource_spans[0].scope_spans[0].spans
            assert len(otlp_spans) == 2
            names = {s.name for s in otlp_spans}
            assert names == {"op-1", "op-2"}
        finally:
            server.stop(grace=1)
