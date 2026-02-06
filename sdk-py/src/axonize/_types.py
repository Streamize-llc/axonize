"""Core types: enums and span data structures."""

from __future__ import annotations

import enum
from dataclasses import dataclass, field


class SpanKind(enum.Enum):
    """Type of span operation."""

    INTERNAL = "internal"
    CLIENT = "client"
    SERVER = "server"


class SpanStatus(enum.Enum):
    """Status of a completed span."""

    UNSET = "unset"
    OK = "ok"
    ERROR = "error"


@dataclass(frozen=True)
class GPUAttribution:
    """Immutable GPU attribution snapshot attached to a span."""

    resource_uuid: str
    physical_gpu_uuid: str
    gpu_model: str
    node_id: str
    resource_type: str
    user_label: str
    memory_used_gb: float
    memory_total_gb: float
    utilization: float
    temperature_celsius: int
    power_watts: int
    clock_mhz: int


@dataclass(frozen=True)
class SpanData:
    """Immutable snapshot of a completed span for buffer storage."""

    span_id: str
    trace_id: str
    name: str
    kind: SpanKind
    status: SpanStatus
    start_time_ns: int
    end_time_ns: int
    duration_ms: float
    service_name: str
    attributes: dict[str, str | int | float | bool] = field(default_factory=dict)
    parent_span_id: str | None = None
    gpu_attributions: list[GPUAttribution] = field(default_factory=list)
    error_message: str | None = None
    environment: str = "development"
