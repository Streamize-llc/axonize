# Axonize Architecture Document

> **Version**: 0.1.1 (Draft)
> **Last Updated**: 2025-12-29
> **Status**: Design Phase

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [Architecture Overview](#3-architecture-overview)
4. [Data Model](#4-data-model)
5. [SDK Design](#5-sdk-design)
6. [Server Architecture](#6-server-architecture)
7. [Design Decisions](#7-design-decisions)
8. [MVP Scope](#8-mvp-scope)
9. [Future Considerations](#9-future-considerations)

---

## 1. Executive Summary

### 1.1 What is Axonize?

Axonize는 AI 추론(Inference) 워크로드를 위한 **옵저버빌리티(Observability) 플랫폼**입니다.

프로젝트 이름은 신경세포의 신호 전달 통로인 **Axon(축삭)**에서 유래했습니다. 데이터가 입력(Prompt)되어 모델 레이어를 거쳐 출력(Inference)되기까지의 전 과정을 추적합니다.

### 1.2 Core Value Proposition

```
┌─────────────────────────────────────────────────────────────────┐
│                      Market Positioning                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Grafana/Prometheus        [Axonize]         Langfuse/LangSmith│
│   ─────────────────        ──────────         ──────────────────│
│   Infrastructure            Inference          LLM Service      │
│   Monitoring               Infrastructure      Tracing          │
│                            Optimization                          │
│                                                                  │
│   "서버가 살아있나?"      "모델이 최적으로     "API 호출이       │
│                           돌아가나?"           성공했나?"        │
│                                                                  │
│   CPU, Memory, GPU        TTFT, TPOT,         Prompt, Token,    │
│   Utilization             KV Cache,           Completion        │
│                           Step Latency                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 1.3 Target Users

| User Type | Use Case | Pain Point |
|-----------|----------|------------|
| ML Engineer | 추론 서버 운영 | 어떤 요청이 느린지 모름 |
| Platform Team | GPU 클러스터 관리 | GPU 효율성 측정 어려움 |
| Finance/Ops | 비용 관리 | 추론당 비용 산정 불가 |

---

## 2. Problem Statement

### 2.1 Current Landscape Gaps

**Gap 1: 인프라 ↔ 모델 메트릭 단절**
```
현재 상황:
- Grafana: GPU utilization 87% ← 숫자만 보임
- 실제 필요: 이 87% 중 몇 %가 실제 추론이고, 몇 %가 대기인가?
```

**Gap 2: 멀티모달 통합 부재**
```
현재 상황:
- LLM 모니터링: Langfuse (토큰 기반)
- Image 모니터링: ??? (표준 없음)
- Audio 모니터링: ??? (표준 없음)

필요: 모든 모달리티를 하나의 규격으로 통합
```

**Gap 3: 계층적 추론 추적 부재**
```
이미지 생성 파이프라인 예시:

Request ──→ VIT Embedding ──→ Diffusion (20 steps) ──→ VAE Decode ──→ Response
              120ms              3200ms                   180ms
              GPU:0              GPU:0,1,2,3              GPU:0

현재: 전체 시간만 알 수 있음 (3500ms)
필요: 각 단계별 breakdown + GPU 귀속
```

### 2.2 Why Existing Tools Fall Short

| Tool | Limitation for Inference Monitoring |
|------|-------------------------------------|
| Prometheus/Grafana | 모델 내부 메트릭 수집 불가, Pull 모델의 한계 |
| Langfuse/LangSmith | LLM API 전용, Self-hosted 추론 서버 미지원 |
| Weights & Biases | 학습(Training) 중심, 추론 최적화 기능 부족 |
| Jaeger/Zipkin | 범용 트레이싱, AI/GPU 특화 메트릭 없음 |

---

## 3. Architecture Overview

### 3.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Axonize Platform                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐        │
│  │   Dashboard    │  │  OTLP Server   │  │  GPU Registry  │        │
│  │   (React)      │  │  (gRPC/HTTP)   │  │  (Device DB)   │        │
│  └───────┬────────┘  └───────┬────────┘  └───────┬────────┘        │
│          │                   │                   │                  │
│          └───────────────────┼───────────────────┘                  │
│                              │                                       │
│                    ┌─────────┴─────────┐                            │
│                    │    ClickHouse     │                            │
│                    │  (Time-series DB) │                            │
│                    └───────────────────┘                            │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
                               ▲
                               │ OTLP (gRPC)
                               │ < 1% overhead
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
┌───────┴───────┐    ┌─────────┴────────┐   ┌────────┴────────┐
│  axonize-py   │    │  Direct OTLP     │   │  Future SDKs    │
│               │    │  (Jaeger 호환)    │   │                 │
│ - PyTorch     │    │                  │   │ - axonize-js    │
│ - HuggingFace │    │                  │   │ - axonize-go    │
│ - Diffusers   │    │                  │   │                 │
│ - vLLM / TGI  │    │                  │   │                 │
└───────────────┘    └──────────────────┘   └─────────────────┘
```

### 3.2 Component Responsibilities

| Component | Responsibility | Technology |
|-----------|---------------|------------|
| **axonize-py** | Python SDK, 메트릭 수집 및 전송 | Python, pynvml/IOKit, OTel SDK |
| **OTLP Server** | Span 수신, 검증, 저장 | Go or Rust, gRPC |
| **GPU Registry** | GPU 디바이스 정보 관리 | PostgreSQL |
| **ClickHouse** | 시계열 Span 데이터 저장 | ClickHouse |
| **Dashboard** | 시각화, 분석 UI | React, TypeScript |

### 3.3 Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         Data Flow                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Inference Request                                            │
│     │                                                            │
│     ▼                                                            │
│  2. SDK creates Trace                                            │
│     │  - trace_id generated                                      │
│     │  - start_time timestamp                                    │
│     │                                                            │
│     ▼                                                            │
│  3. SDK creates Span for each sub-operation                      │
│     │  - VIT embedding span                                      │
│     │  - Diffusion span (with child spans per step)              │
│     │  - VAE decode span                                         │
│     │                                                            │
│     ▼                                                            │
│  4. GPU Profiler attaches resource info                          │
│     │  - resource_uuid (PK)                                      │
│     │  - physical_gpu_uuid                                       │
│     │  - runtime metrics (util, memory, power)                   │
│     │                                                            │
│     ▼                                                            │
│  5. Async export to ring buffer                                  │
│     │  (< 100ns, non-blocking)                                   │
│     │                                                            │
│     ▼                                                            │
│  6. Background thread batches & sends                            │
│     │  - Batch size: 100 spans                                   │
│     │  - Flush interval: 1 second                                │
│     │  - gRPC with compression                                   │
│     │                                                            │
│     ▼                                                            │
│  7. Server receives, validates, stores                           │
│     │                                                            │
│     ▼                                                            │
│  8. Dashboard queries & visualizes                               │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. Data Model

### 4.1 Core Concepts

Axonize의 데이터 모델은 **OpenTelemetry 표준을 기반**으로 하되, AI 추론에 특화된 확장을 추가합니다.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Core Concepts                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Trace                                                          │
│   └── 하나의 추론 요청 전체                                       │
│       │                                                          │
│       ├── Span (root)                                            │
│       │   └── "image_generation"                                 │
│       │       │                                                  │
│       │       ├── Span (child)                                   │
│       │       │   └── "vit_embedding"                            │
│       │       │       └── GPUAttribution                         │
│       │       │                                                  │
│       │       ├── Span (child)                                   │
│       │       │   └── "diffusion"                                │
│       │       │       ├── GPUAttribution                         │
│       │       │       │                                          │
│       │       │       ├── Span (grandchild)                      │
│       │       │       │   └── "unet_step_0"                      │
│       │       │       ├── Span (grandchild)                      │
│       │       │       │   └── "unet_step_1"                      │
│       │       │       └── ...                                    │
│       │       │                                                  │
│       │       └── Span (child)                                   │
│       │           └── "vae_decode"                               │
│       │               └── GPUAttribution                         │
│       │                                                          │
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 Trace

하나의 추론 요청 전체를 나타냅니다.

```python
@dataclass
class Trace:
    """하나의 추론 요청 전체"""

    # === 식별자 ===
    trace_id: str               # UUID v4, 전역 고유

    # === 타이밍 ===
    start_time: datetime        # 추론 시작 시간
    end_time: datetime          # 추론 완료 시간

    # === 계산 필드 ===
    @property
    def duration_ms(self) -> float:
        """전체 소요 시간"""
        return (self.end_time - self.start_time).total_seconds() * 1000

    # === 관계 ===
    spans: List[Span]           # 하위 작업들

    # === 메타데이터 ===
    service_name: str           # "image-gen-service"
    environment: str            # "production", "staging"
    version: str                # "1.2.3"

    # === 비용 ===
    @property
    def total_cost_usd(self) -> float:
        """전체 추론 비용 (모든 span 합산)"""
        return sum(s.cost_usd for s in self.spans if s.cost_usd)
```

**설계 근거:**
- `service_name`, `environment`: 멀티 서비스 환경에서 필터링 지원

### 4.3 Span

추론 내의 개별 작업 단위입니다.

```python
@dataclass
class Span:
    """추론 내의 하위 작업"""

    # === 식별자 ===
    span_id: str                # UUID v4
    trace_id: str               # FK → Trace
    parent_span_id: Optional[str]  # 부모 span (계층 구조)

    # === 기본 정보 ===
    name: str                   # "vit_embedding", "unet_step_0"
    kind: SpanKind              # INTERNAL, CLIENT, SERVER

    # === 타이밍 ===
    start_time: datetime
    end_time: datetime
    duration_ms: float          # 편의를 위한 중복 저장

    # === AI 모델 정보 ===
    model_name: Optional[str]   # "ViT-L/14", "SDXL-UNet"
    model_version: Optional[str]
    inference_type: Optional[str]  # "embedding", "generation", "decode"

    # === GPU 귀속 ===
    gpu_attributions: List[GPUAttribution]  # 사용된 GPU들

    # === 모달리티별 메트릭 ===
    # LLM
    tokens_input: Optional[int]
    tokens_output: Optional[int]
    tokens_per_second: Optional[float]
    ttft_ms: Optional[float]    # Time To First Token

    # Diffusion
    diffusion_steps: Optional[int]
    step_latencies_ms: Optional[List[float]]
    cfg_scale: Optional[float]

    # 공통
    batch_size: Optional[int]

    # === 상태 ===
    status: SpanStatus          # OK, ERROR
    error_message: Optional[str]

    # === 비용 ===
    cost_usd: Optional[float]   # 이 span의 추정 비용

    # === 자식 span ===
    children: List[Span]


class SpanKind(Enum):
    INTERNAL = "internal"       # 내부 처리
    CLIENT = "client"           # 외부 서비스 호출
    SERVER = "server"           # 요청 수신


class SpanStatus(Enum):
    UNSET = "unset"
    OK = "ok"
    ERROR = "error"
```

**설계 근거:**
- `parent_span_id`: 계층적 추론 파이프라인 표현 (VIT → Diffusion → VAE)
- `gpu_attributions` List: 하나의 span이 여러 GPU 사용 가능 (Tensor Parallelism)
- 모달리티별 필드 분리: LLM과 Diffusion의 메트릭이 완전히 다름

### 4.4 GPU Identity Model (3-Layer)

GPU 리소스를 정확히 식별하기 위한 3계층 모델입니다.

```
┌─────────────────────────────────────────────────────────────────┐
│                    GPU Identity Model                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Context Layer (사용자 인터페이스)                        │    │
│  │  ─────────────────────────────────────                   │    │
│  │  "사용자가 코드에서 보는 것"                               │    │
│  │                                                          │    │
│  │  user_label: "cuda:0"                                   │    │
│  │  pod_id: "inference-pod-abc123"                         │    │
│  │  container_id: "docker-xyz"                             │    │
│  │  process_id: 12345                                      │    │
│  │                                                          │    │
│  │  특징: Pod 재시작 시 변경됨, 동적                         │    │
│  └──────────────────────────┬──────────────────────────────┘    │
│                             │                                    │
│                             ▼                                    │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Resource Layer (논리적 연산 단위) ← Axonize PK          │    │
│  │  ─────────────────────────────────────                   │    │
│  │  "실제로 연산을 수행하는 단위"                             │    │
│  │                                                          │    │
│  │  resource_uuid: "MIG-xxxx" or "GPU-82f7..."             │    │
│  │  resource_type: "mig_1g_10gb" or "full_gpu"             │    │
│  │  memory_gb: 10.0                                        │    │
│  │  sm_count: 14                                           │    │
│  │                                                          │    │
│  │  특징: MIG 설정 변경 전까지 고정, 핵심 PK                 │    │
│  └──────────────────────────┬──────────────────────────────┘    │
│                             │                                    │
│                             ▼                                    │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Physical Layer (물리적 하드웨어)                         │    │
│  │  ─────────────────────────────────────                   │    │
│  │  "서버에 실제로 꽂혀있는 GPU 카드"                        │    │
│  │                                                          │    │
│  │  physical_gpu_uuid: "GPU-82f7-..."                      │    │
│  │  gpu_model: "NVIDIA H100 80GB HBM3"                     │    │
│  │  architecture: "Hopper"                                 │    │
│  │  node_id: "gpu-node-01"                                 │    │
│  │  pcie_bus_id: "0000:3B:00.0"                           │    │
│  │                                                          │    │
│  │  특징: 하드웨어 교체 전까지 불변                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**왜 3계층인가?**

```
문제 상황: MIG 환경에서의 GPU 식별

┌─────────────────────────────────────────────────┐
│  Physical GPU: GPU-82f7... (H100 80GB)          │
├─────────┬─────────┬─────────┬───────────────────┤
│ MIG-aaa │ MIG-bbb │ MIG-ccc │ MIG-ddd           │
│ 10GB    │ 10GB    │ 20GB    │ 40GB              │
│ Pod-A   │ Pod-B   │ Pod-C   │ Pod-D             │
│ cuda:0  │ cuda:0  │ cuda:0  │ cuda:0  ← 전부 동일!│
└─────────┴─────────┴─────────┴───────────────────┘

1계층만 있으면: cuda:0이 4개 → 구분 불가
2계층만 있으면: 물리 GPU 집계 불가
3계층 있으면: 모든 레벨에서 정확한 식별 및 집계 가능
```

### 4.5 GPU Data Structures

```python
# ============================================
# Physical Layer
# ============================================
@dataclass
class PhysicalGPU:
    """물리적 GPU 카드 - 서버에 꽂혀있는 실체"""

    # === 식별자 ===
    uuid: str                   # "GPU-82f7-..." (nvidia-smi UUID)

    # === 하드웨어 스펙 ===
    model: str                  # "NVIDIA H100 80GB HBM3"
    vendor: str                 # "NVIDIA"
    architecture: str           # "Hopper", "Ampere", "Ada Lovelace"
    compute_capability: str     # "9.0"

    # === 메모리 ===
    memory_total_gb: float      # 80.0
    memory_bandwidth_gbps: float # 3350

    # === 컴퓨팅 파워 ===
    fp16_tflops: float          # 1979
    fp32_tflops: float          # 989
    tensor_core_tflops: float   # 3958
    sm_count: int               # 132 (H100)

    # === 전력 ===
    tdp_watts: int              # 700

    # === 위치 정보 ===
    node_id: str                # K8s node name or hostname
    pcie_bus_id: str            # "0000:3B:00.0"
    numa_node: int              # NUMA 노드 (메모리 지역성)

    # === 토폴로지 ===
    nvlink_peers: List[str]     # NVLink로 연결된 다른 GPU UUID

    # === 런타임 정보 ===
    driver_version: str         # "535.104.12"
    cuda_version: str           # "12.2"

    # === 클라우드 환경 (optional) ===
    cloud_provider: Optional[str]     # "aws", "gcp", "azure"
    cloud_instance_id: Optional[str]  # "i-1234567890abcdef0"
    cloud_zone: Optional[str]         # "us-east-1a"

    # === 비용 설정 ===
    cost_per_hour_usd: Optional[float]  # 사용자 설정


# ============================================
# Resource Layer
# ============================================
@dataclass
class ComputeResource:
    """논리적 연산 단위 - Axonize의 핵심 PK"""

    # === 식별자 (PK) ===
    resource_uuid: str          # MIG UUID 또는 GPU UUID

    # === 타입 ===
    resource_type: ResourceType

    # === 물리 GPU 참조 ===
    physical_gpu_uuid: str      # FK → PhysicalGPU

    # === 이 리소스의 스펙 ===
    memory_gb: float            # 할당된 메모리 (MIG면 부분)
    sm_count: int               # 할당된 SM 개수

    # === MIG 전용 필드 ===
    mig_profile: Optional[str]  # "1g.10gb", "3g.40gb"
    gi_id: Optional[int]        # GPU Instance ID
    ci_id: Optional[int]        # Compute Instance ID

    # === 공유 여부 ===
    is_shared: bool             # time-slicing 등으로 공유되는지

    # === 메타데이터 ===
    created_at: datetime
    last_seen_at: datetime


class ResourceType(Enum):
    FULL_GPU = "full_gpu"
    MIG_1G_10GB = "mig_1g_10gb"
    MIG_2G_20GB = "mig_2g_20gb"
    MIG_3G_40GB = "mig_3g_40gb"
    MIG_4G_40GB = "mig_4g_40gb"
    MIG_7G_80GB = "mig_7g_80gb"
    VGPU = "vgpu"
    TIME_SLICED = "time_sliced"


# ============================================
# Context Layer
# ============================================
@dataclass
class ResourceContext:
    """런타임 매핑 - 누가 이 리소스를 쓰고 있는지"""

    # === 식별자 ===
    context_id: str             # Axonize가 부여

    # === 리소스 참조 ===
    resource_uuid: str          # FK → ComputeResource

    # === 사용자 관점 ===
    user_label: str             # "cuda:0", "cuda:1"

    # === 프로세스 정보 ===
    hostname: str               # 머신 이름
    process_id: int
    process_name: str           # "python"

    # === 환경별 메타데이터 (범용) ===
    labels: Dict[str, str]      # 오케스트레이터/환경별 자유롭게 확장
    # K8s:        {"k8s.pod.name": "...", "k8s.namespace": "...", "k8s.node.name": "..."}
    # Docker:     {"docker.container.id": "...", "docker.container.name": "..."}
    # Bare Metal: {"datacenter": "...", "rack": "..."}
    # AWS ECS:    {"aws.ecs.cluster": "...", "aws.ecs.task.id": "..."}

    # === 시간 ===
    attached_at: datetime
    detached_at: Optional[datetime]


# ============================================
# GPU Attribution (Span에 첨부)
# ============================================
@dataclass
class GPUAttribution:
    """Span에 첨부되는 GPU 정보 - 역정규화된 스냅샷"""

    # === 핵심 식별자 ===
    resource_uuid: str          # PK - 어떤 연산 단위를 썼는지

    # === 집계/분석용 역정규화 ===
    physical_gpu_uuid: str      # 물리 GPU (집계용)
    gpu_model: str              # "H100" (빠른 필터링용)
    node_id: str                # 서버 식별
    resource_type: str          # "full_gpu", "mig_1g_10gb"

    # === 사용자가 인식하는 이름 ===
    user_label: str             # "cuda:0"

    # === 런타임 상태 스냅샷 ===
    memory_used_gb: float
    memory_total_gb: float
    utilization_percent: float
    temperature_celsius: int
    power_watts: int
    clock_mhz: int

    # === 비용 계산 ===
    duration_ms: float          # 이 GPU를 사용한 시간
    energy_wh: float            # 소비 에너지
    cost_usd: float             # 추정 비용
```

**설계 근거:**

| 결정 | 근거 |
|------|------|
| `resource_uuid`를 PK로 | MIG, vGPU, 일반 GPU 모두 일관된 방식으로 처리 |
| `physical_gpu_uuid` 별도 유지 | 물리 GPU 기준 집계 필요 (같은 카드의 MIG들 합산) |
| `GPUAttribution`에 역정규화 | JOIN 없이 빠른 쿼리, 시계열 분석 최적화 |
| `user_label` 보존 | 사용자 디버깅 시 친숙한 이름 필요 |

---

## 5. SDK Design

### 5.1 Design Principles

```
┌─────────────────────────────────────────────────────────────────┐
│                    SDK Design Principles                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Zero-Overhead First                                          │
│     "추론 성능에 1% 이상 영향을 주면 실패"                        │
│                                                                  │
│  2. OpenTelemetry Compatible                                     │
│     "독자 규격 X, OTel 확장으로 도입 장벽 낮춤"                   │
│                                                                  │
│  3. Graceful Degradation                                         │
│     "서버 장애 시에도 추론은 계속되어야 함"                       │
│                                                                  │
│  4. Developer Ergonomics                                         │
│     "pip install axonize 한 줄로 시작"                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 Overhead Budget

```
목표: 추론 스레드에 < 1μs 오버헤드

┌─────────────────────────────────────────────────────────────────┐
│                     Overhead Breakdown                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Operation                          Target        Actual         │
│  ─────────────────────────────────  ──────        ──────         │
│  span.start()                       < 500ns       TBD            │
│  span.set_attribute() x 10          < 200ns       TBD            │
│  GPU snapshot (pynvml)              < 50μs        (async)        │
│  span.end()                         < 200ns       TBD            │
│  Buffer enqueue                     < 100ns       TBD            │
│  ─────────────────────────────────  ──────                       │
│  Total (excluding async)            < 1μs                        │
│                                                                  │
│  Async operations (별도 스레드):                                  │
│  - GPU profiling                                                 │
│  - Network transmission                                          │
│  - Compression                                                   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.3 Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     SDK Architecture                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Inference Thread (Critical Path - 절대 블로킹 금지)            │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                                                           │  │
│   │  model.forward(input)                                     │  │
│   │      │                                                    │  │
│   │      ▼                                                    │  │
│   │  with axonize.span("forward"):                           │  │
│   │      │                                                    │  │
│   │      ├──▶ span_data = create_span()     [< 500ns]        │  │
│   │      │                                                    │  │
│   │      ├──▶ ring_buffer.enqueue(span_data) [< 100ns]       │  │
│   │      │        │                                           │  │
│   │      │        │  Lock-free, non-blocking                  │  │
│   │      │        ▼                                           │  │
│   │      │   ┌─────────────────┐                              │  │
│   │      │   │   Ring Buffer   │  (size: 10,000)              │  │
│   │      │   │   [][][][][]... │                              │  │
│   │      │   └────────┬────────┘                              │  │
│   │      │            │                                       │  │
│   │      ▼            │                                       │  │
│   │  continue execution                                       │  │
│   │                   │                                       │  │
│   └───────────────────│───────────────────────────────────────┘  │
│                       │                                          │
│                       │ (별도 스레드)                             │
│                       ▼                                          │
│   Background Thread (Non-critical)                               │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                                                           │  │
│   │  while True:                                              │  │
│   │      sleep(flush_interval)  # 1초                         │  │
│   │      │                                                    │  │
│   │      ▼                                                    │  │
│   │  batch = ring_buffer.drain(max=100)                       │  │
│   │      │                                                    │  │
│   │      ▼                                                    │  │
│   │  enrich_with_gpu_metrics(batch)  # pynvml 호출            │  │
│   │      │                                                    │  │
│   │      ▼                                                    │  │
│   │  compressed = gzip(batch)                                 │  │
│   │      │                                                    │  │
│   │      ▼                                                    │  │
│   │  try:                                                     │  │
│   │      grpc_send(compressed)                                │  │
│   │  except NetworkError:                                     │  │
│   │      disk_buffer.write(batch)  # 나중에 재전송             │  │
│   │                                                           │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.4 API Design

```python
# ============================================
# 초기화
# ============================================
import axonize

axonize.init(
    endpoint="http://axonize-server:4317",
    service_name="image-gen-service",
    environment="production",

    # 성능 튜닝
    batch_size=100,
    flush_interval_ms=1000,
    buffer_size=10000,

    # 샘플링
    sampling_rate=1.0,          # 1.0 = 100%
    always_sample_errors=True,
    always_sample_slow=True,
    slow_threshold_ms=5000,

    # GPU 프로파일링
    gpu_profiling=True,
    gpu_snapshot_interval_ms=100,
)


# ============================================
# 기본 사용법
# ============================================
@axonize.trace(name="image_generation")
def generate_image(prompt: str, reference_image: bytes):
    """데코레이터로 전체 추론을 trace로 감싸기"""

    # 1단계: VIT 임베딩
    with axonize.span("vit_embedding", model="ViT-L/14") as span:
        span.set_gpus(["cuda:0"])
        embedding = vit_model.encode(reference_image)
        span.set_attribute("embedding_dim", embedding.shape[-1])

    # 2단계: Diffusion
    with axonize.span("diffusion", model="SDXL") as span:
        span.set_gpus(["cuda:0", "cuda:1", "cuda:2", "cuda:3"])
        span.set_attribute("steps", 20)
        span.set_attribute("cfg_scale", 7.5)

        # 각 스텝을 하위 span으로
        for i in range(20):
            with axonize.span(f"unet_step_{i}") as step_span:
                step_span.set_gpus(["cuda:0", "cuda:1"])
                latent = unet(latent, timestep=i)

    # 3단계: VAE Decode
    with axonize.span("vae_decode", model="SDXL-VAE") as span:
        span.set_gpus(["cuda:0"])
        image = vae.decode(latent)

    return image


# ============================================
# LLM 특화 API
# ============================================
with axonize.llm_span("generation", model="llama-70b") as span:
    span.set_gpus(["cuda:0", "cuda:1", "cuda:2", "cuda:3"])

    # 스트리밍 토큰 추적
    for token in model.generate_stream(prompt):
        span.record_token()  # TTFT, TPOT 자동 계산
        yield token
```

### 5.5 OpenTelemetry Compatibility

```python
# Axonize는 OTel Semantic Conventions를 확장

# === 표준 OTel 속성 ===
span.set_attribute("service.name", "inference-server")
span.set_attribute("service.version", "1.2.3")

# === Axonize AI 확장 속성 ===
# Prefix: ai.* (AI/ML 관련)
span.set_attribute("ai.model.name", "SDXL")
span.set_attribute("ai.model.version", "1.0")
span.set_attribute("ai.inference.type", "image_generation")
span.set_attribute("ai.inference.batch_size", 4)

# Prefix: ai.llm.* (LLM 특화)
span.set_attribute("ai.llm.tokens.input", 150)
span.set_attribute("ai.llm.tokens.output", 500)
span.set_attribute("ai.llm.tokens_per_second", 45.2)
span.set_attribute("ai.llm.ttft_ms", 120.5)

# Prefix: ai.diffusion.* (Diffusion 특화)
span.set_attribute("ai.diffusion.steps", 20)
span.set_attribute("ai.diffusion.cfg_scale", 7.5)
span.set_attribute("ai.diffusion.scheduler", "euler_a")

# Prefix: gpu.* (GPU 리소스)
span.set_attribute("gpu.0.resource_uuid", "MIG-xxxx")
span.set_attribute("gpu.0.physical_uuid", "GPU-82f7...")
span.set_attribute("gpu.0.model", "NVIDIA H100 80GB")
span.set_attribute("gpu.0.memory.used_gb", 45.2)
span.set_attribute("gpu.0.utilization", 87.5)
span.set_attribute("gpu.0.power_watts", 650)

# Prefix: cost.* (비용)
span.set_attribute("cost.energy_wh", 0.18)
span.set_attribute("cost.estimated_usd", 0.0023)
```

**호환성 이점:**
```
┌─────────────────────────────────────────────────────────────────┐
│                  OTel Compatibility Benefits                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. 기존 인프라 재사용                                            │
│     - Jaeger, Tempo, Zipkin으로 바로 export 가능                 │
│     - Grafana에서 trace 시각화 가능                               │
│                                                                  │
│  2. 도입 장벽 최소화                                              │
│     - 이미 OTel 쓰는 조직은 익숙한 개념                           │
│     - 기존 instrumentation과 공존 가능                           │
│                                                                  │
│  3. 생태계 활용                                                   │
│     - OTel Collector 사용 가능                                   │
│     - 다양한 exporter 활용                                       │
│                                                                  │
│  4. 미래 호환성                                                   │
│     - OTel이 업계 표준으로 자리잡는 중                            │
│     - vendor lock-in 방지                                        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 6. Server Architecture

### 6.1 Components

```
┌─────────────────────────────────────────────────────────────────┐
│                     Server Architecture                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    API Gateway                           │    │
│  │                   (nginx/envoy)                          │    │
│  │                                                          │    │
│  │  - TLS termination                                       │    │
│  │  - Rate limiting                                         │    │
│  │  - Authentication                                        │    │
│  └─────────────────────────┬───────────────────────────────┘    │
│                            │                                     │
│            ┌───────────────┼───────────────┐                    │
│            │               │               │                    │
│            ▼               ▼               ▼                    │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │   Ingest    │ │    Query    │ │     API     │               │
│  │   Service   │ │   Service   │ │   Service   │               │
│  │             │ │             │ │             │               │
│  │ - OTLP recv │ │ - ClickHouse│ │ - REST API  │               │
│  │ - Validate  │ │   queries   │ │ - WebSocket │               │
│  │ - Enrich    │ │ - Aggregate │ │ - Settings  │               │
│  │ - Buffer    │ │ - Cache     │ │             │               │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘               │
│         │               │               │                       │
│         │               │               │                       │
│         ▼               ▼               ▼                       │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                     Data Layer                           │    │
│  ├─────────────────────────────────────────────────────────┤    │
│  │                                                          │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │    │
│  │  │ ClickHouse  │  │ PostgreSQL  │  │    Redis    │      │    │
│  │  │             │  │             │  │             │      │    │
│  │  │ - Spans     │  │ - GPU Reg   │  │ - Cache     │      │    │
│  │  │ - Traces    │  │ - Users     │  │ - Sessions  │      │    │
│  │  │ - Metrics   │  │ - Settings  │  │ - Real-time │      │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘      │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 6.2 ClickHouse Schema

```sql
-- ============================================
-- Spans 테이블 (메인 시계열 데이터)
-- ============================================
CREATE TABLE spans (
    -- 식별자
    trace_id String,
    span_id String,
    parent_span_id Nullable(String),

    -- 기본 정보
    name String,
    service_name String,
    environment String,

    -- 타이밍
    start_time DateTime64(3),
    end_time DateTime64(3),
    duration_ms Float64,

    -- AI 모델 정보
    model_name Nullable(String),
    model_version Nullable(String),
    inference_type Nullable(String),

    -- LLM 메트릭
    tokens_input Nullable(UInt32),
    tokens_output Nullable(UInt32),
    tokens_per_second Nullable(Float32),
    ttft_ms Nullable(Float32),

    -- Diffusion 메트릭
    diffusion_steps Nullable(UInt16),
    cfg_scale Nullable(Float32),

    -- GPU 정보 (역정규화)
    gpu_resource_uuids Array(String),
    gpu_physical_uuids Array(String),
    gpu_models Array(String),
    gpu_node_ids Array(String),
    gpu_memory_used_gb Array(Float32),
    gpu_utilization Array(Float32),
    gpu_power_watts Array(UInt16),

    -- 비용
    cost_usd Nullable(Float64),

    -- 상태
    status String,
    error_message Nullable(String),

    -- 속성 (유연한 확장)
    attributes Map(String, String),

    -- 파티셔닝/정렬 키
    INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 1,
    INDEX idx_model_name model_name TYPE bloom_filter GRANULARITY 1,
    INDEX idx_service_name service_name TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (service_name, start_time, trace_id)
TTL start_time + INTERVAL 30 DAY;


-- ============================================
-- Traces 테이블 (집계된 trace 정보)
-- ============================================
CREATE TABLE traces (
    trace_id String,

    -- 타이밍
    start_time DateTime64(3),
    end_time DateTime64(3),
    duration_ms Float64,

    -- 메타데이터
    service_name String,
    environment String,
    root_span_name String,

    -- 집계
    span_count UInt32,
    error_count UInt32,

    -- 비용
    total_cost_usd Float64,

    -- GPU 사용 요약
    gpu_count UInt8,
    total_gpu_time_ms Float64,

    INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(start_time)
ORDER BY (service_name, start_time)
TTL start_time + INTERVAL 90 DAY;


-- ============================================
-- GPU Metrics 테이블 (시계열 GPU 상태)
-- ============================================
CREATE TABLE gpu_metrics (
    timestamp DateTime64(3),

    -- GPU 식별
    resource_uuid String,
    physical_gpu_uuid String,
    node_id String,

    -- 상태
    utilization Float32,
    memory_used_gb Float32,
    memory_total_gb Float32,
    temperature_celsius UInt8,
    power_watts UInt16,
    clock_mhz UInt16,

    -- 추론 활동
    active_spans UInt16,

    INDEX idx_resource_uuid resource_uuid TYPE bloom_filter GRANULARITY 1
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (node_id, resource_uuid, timestamp)
TTL timestamp + INTERVAL 7 DAY;
```

### 6.3 PostgreSQL Schema (GPU Registry)

```sql
-- ============================================
-- Physical GPUs (불변 하드웨어 정보)
-- ============================================
CREATE TABLE physical_gpus (
    uuid VARCHAR(64) PRIMARY KEY,  -- GPU-xxxx

    -- 하드웨어 스펙
    model VARCHAR(128) NOT NULL,
    vendor VARCHAR(32) NOT NULL,
    architecture VARCHAR(32),
    compute_capability VARCHAR(8),

    -- 메모리
    memory_total_gb DECIMAL(5,1) NOT NULL,
    memory_bandwidth_gbps INTEGER,

    -- 컴퓨팅
    sm_count INTEGER,
    fp16_tflops DECIMAL(6,1),
    fp32_tflops DECIMAL(6,1),
    tdp_watts INTEGER,

    -- 위치
    node_id VARCHAR(128) NOT NULL,
    pcie_bus_id VARCHAR(16),
    numa_node SMALLINT,

    -- 런타임
    driver_version VARCHAR(32),
    cuda_version VARCHAR(16),

    -- 클라우드
    cloud_provider VARCHAR(16),
    cloud_instance_id VARCHAR(64),
    cloud_zone VARCHAR(32),

    -- 비용
    cost_per_hour_usd DECIMAL(10,4),

    -- 메타
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_seen_at TIMESTAMP
);


-- ============================================
-- Compute Resources (논리적 연산 단위)
-- ============================================
CREATE TABLE compute_resources (
    resource_uuid VARCHAR(64) PRIMARY KEY,  -- MIG-xxxx or GPU-xxxx

    -- 타입
    resource_type VARCHAR(32) NOT NULL,  -- full_gpu, mig_1g_10gb, etc.

    -- 물리 GPU 참조
    physical_gpu_uuid VARCHAR(64) NOT NULL REFERENCES physical_gpus(uuid),

    -- 스펙
    memory_gb DECIMAL(5,1) NOT NULL,
    sm_count INTEGER,

    -- MIG 정보
    mig_profile VARCHAR(16),
    gi_id SMALLINT,
    ci_id SMALLINT,

    -- 공유 여부
    is_shared BOOLEAN DEFAULT FALSE,

    -- 메타
    created_at TIMESTAMP DEFAULT NOW(),
    last_seen_at TIMESTAMP
);


-- ============================================
-- Resource Contexts (런타임 매핑)
-- ============================================
CREATE TABLE resource_contexts (
    context_id VARCHAR(64) PRIMARY KEY,

    -- 리소스 참조
    resource_uuid VARCHAR(64) NOT NULL REFERENCES compute_resources(resource_uuid),

    -- 사용자 관점
    user_label VARCHAR(16) NOT NULL,  -- cuda:0

    -- 프로세스 정보
    hostname VARCHAR(128) NOT NULL,
    process_id INTEGER,
    process_name VARCHAR(64),

    -- 환경별 메타데이터 (범용)
    -- K8s: {"k8s.pod.name": "...", "k8s.namespace": "..."}
    -- Docker: {"docker.container.id": "..."}
    -- Bare Metal: {"datacenter": "...", "rack": "..."}
    labels JSONB DEFAULT '{}',

    -- 시간
    attached_at TIMESTAMP NOT NULL,
    detached_at TIMESTAMP,

    INDEX idx_resource_uuid (resource_uuid),
    INDEX idx_hostname (hostname)
);

-- labels 내 특정 키로 검색 가능
CREATE INDEX idx_labels ON resource_contexts USING GIN (labels);
```

---

## 7. Design Decisions

### 7.1 Why OpenTelemetry Compatible?

| 대안 | 장점 | 단점 | 결정 |
|------|------|------|------|
| **독자 규격** | 완전한 자유도 | 도입 장벽 높음, 생태계 부재 | ❌ |
| **OTel 완전 준수** | 호환성 최대 | AI 특화 메트릭 제약 | ❌ |
| **OTel 확장** | 호환성 + 유연성 | 복잡도 약간 증가 | ✅ 선택 |

**결정 근거:**
- 빅테크들이 OTel로 수렴하는 추세
- 기존 Jaeger/Grafana 사용자 즉시 활용 가능
- AI 특화 semantic conventions 추가로 요구사항 충족

### 7.2 Why 3-Layer GPU Identity?

| 대안 | 문제점 |
|------|--------|
| **cuda:0만 사용** | MIG 환경에서 모든 Pod이 cuda:0 → 구분 불가 |
| **UUID만 사용** | MIG UUID vs GPU UUID 혼재, 물리 집계 어려움 |
| **2-Layer (Physical + Context)** | MIG 인스턴스를 별도 엔티티로 관리 불가 |

**3-Layer 결정 근거:**
```
Physical: 하드웨어 집계 (같은 카드의 MIG들 합산)
Resource: 실제 PK (MIG든 Full GPU든 일관된 식별)
Context: 동적 매핑 (Pod 재시작에도 Resource 추적 유지)
```

### 7.3 Why ClickHouse?

| DB | 장점 | 단점 | 적합성 |
|----|------|------|--------|
| PostgreSQL | 범용, 익숙함 | 시계열 쿼리 느림 | ❌ |
| TimescaleDB | 시계열 최적화 | 대규모 쓰기 한계 | △ |
| InfluxDB | 시계열 전용 | 복잡한 쿼리 제약 | △ |
| **ClickHouse** | 대용량 쓰기, 빠른 집계 | 운영 복잡도 | ✅ |

**결정 근거:**
- 일일 수십억 row 처리 가능
- 열 기반 압축으로 저장 효율적
- 실시간 집계 쿼리 빠름
- Grafana 플러그인 존재

### 7.4 Why Ring Buffer for SDK?

```
목표: 추론 스레드 블로킹 제로

대안 1: 직접 전송
─────────────────
span.end() → HTTP POST → 서버
문제: 네트워크 지연이 추론에 영향

대안 2: Queue (뮤텍스)
─────────────────
span.end() → mutex.lock() → queue.push() → mutex.unlock()
문제: 락 경합 시 대기 발생

대안 3: Ring Buffer (Lock-free) ✅
─────────────────
span.end() → atomic CAS → buffer[idx] = span
장점: < 100ns, 절대 블로킹 없음
```

---

## 8. MVP Scope

### 8.1 In Scope (v0.1)

```
┌─────────────────────────────────────────────────────────────────┐
│                      MVP Scope (v0.1)                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ✅ Included                                                     │
│  ───────────                                                     │
│  • Python SDK (axonize-py)                                      │
│  • 단일 노드, 단일/멀티 GPU                                       │
│  • NVIDIA GPU (pynvml) + Apple Silicon (IOKit)                  │
│  • Full GPU + MIG 지원                                           │
│  • 환경 무관 (Bare Metal, Docker, K8s, Cloud 등)                 │
│  • 기본 대시보드 (Trace 뷰, GPU 뷰)                               │
│  • OTel 호환 export                                              │
│  • 중소 규모 (일일 1000만 추론 이하)                              │
│                                                                  │
│  Target Users:                                                   │
│  • Self-hosted 추론 서버 운영하는 스타트업/중소기업               │
│  • vLLM, TGI, Ollama 사용자                                     │
│  • Diffusers로 이미지 생성 서비스 운영                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 8.2 Out of Scope (Future)

```
┌─────────────────────────────────────────────────────────────────┐
│                   Out of Scope (v0.2+)                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ❌ Excluded from MVP                                            │
│  ────────────────────                                            │
│  • 멀티 노드 분산 추론 (Tensor Parallelism)                       │
│  • AMD ROCm, Google TPU, AWS Inferentia (GPUBackend 확장)       │
│  • vGPU (VMware/Citrix 가상화)                                   │
│  • 대규모 스케일 (일일 1억+ 추론)                                 │
│  • Adaptive sampling                                             │
│  • SOC2/HIPAA 인증                                               │
│  • 자동 최적화 추천                                               │
│  • 알림 시스템                                                    │
│                                                                  │
│  Why Excluded:                                                   │
│  • 분산 추론: 근본적 아키텍처 변경 필요                           │
│  • 비-NVIDIA: 각 벤더별 SDK 통합 공수                            │
│  • 대규모: 샤딩/파티셔닝 전략 추가 필요                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 8.3 Success Criteria

| Metric | Target | Measurement |
|--------|--------|-------------|
| SDK Overhead | < 1% | 벤치마크 스크립트 |
| Span Ingestion | > 10K/sec | 부하 테스트 |
| Query Latency (P95) | < 500ms | 대시보드 응답 시간 |
| Data Loss | 0% (정상 운영) | E2E 테스트 |
| Setup Time | < 30분 | 문서 따라 설치 |

### 8.4 Known Limitations (MVP)

MVP에서 의도적으로 미해결로 남겨둔 사항들입니다. 구현 진행하면서 구체화합니다.

#### 1. SDK 오버헤드 목표 검증 필요
```
목표: < 1μs (추론 스레드 영향)
예상: Python 구현 시 10-50μs 가능성
조치: 구현 후 벤치마크로 측정, 필요시 Cython/C extension 검토
```

#### 2. GPU Array 컬럼 쿼리 성능
```
현재: spans 테이블에 GPU 정보를 Array로 저장
문제: GPU별 집계 쿼리 시 ARRAY JOIN 필요 (비용 높음)
조치: MVP 규모에서 모니터링, 병목 발생 시 별도 테이블로 분리
```

#### 3. 보안 정책 미정의
```
미정의 항목:
- SDK ↔ Server 인증 방식 (API Key, mTLS 등)
- TLS 설정
- 민감 데이터 (Prompt 내용 등) 처리 정책
- Multi-tenant 데이터 격리

조치: MVP는 내부망/신뢰 환경 가정, v0.2에서 보안 레이어 추가
```

#### 4. 비용 계산 정책 미정의
```
미정의 항목:
- MIG 인스턴스 비용 분배 (메모리 비율? SM 비율?)
- 여러 Span이 동시에 GPU 사용 시 비용 분배
- Idle 시간 비용 처리

조치: cost_usd 필드는 Optional, 정책 정의 후 구현
```

#### 5. Queue 모니터링 미지원
```
현재: E2E duration만 측정, Queue 대기 시간 별도 측정 안 함
이유:
- 회사마다 Queue 시스템이 다름 (Kafka, Redis, Celery 등)
- 범용 표준화 어려움

조치: v0.2+에서 인기 Queue 시스템별 Integration 제공 검토
      (Celery, Redis Queue 등)
```

---

## 9. Future Considerations

### 9.1 Distributed Tracing (v0.2)

멀티 노드 Tensor Parallelism 지원을 위한 설계 방향:

```python
# 문제: 4개 GPU에서 동시에 실행되는 forward pass
# Process 0 (GPU 0): layers 0-7
# Process 1 (GPU 1): layers 8-15
# ...

# 해결: Distributed Context Propagation
class DistributedTraceContext:
    """NCCL 통신 시 trace context 전파"""

    def inject_to_tensor(self, tensor, trace_context):
        """텐서 메타데이터에 context 첨부"""
        pass

    def extract_from_tensor(self, tensor):
        """텐서에서 context 추출"""
        pass
```

### 9.2 Multi-Vendor GPU Support

GPU 백엔드는 `GPUBackend` Protocol로 추상화되어 있으며, 벤더별 구현이 자동 선택됩니다.

**현재 지원:**

| 벤더 | 백엔드 모듈 | 메트릭 소스 | 디바이스 라벨 |
|------|------------|-----------|-------------|
| **NVIDIA** | `_gpu_nvml.py` (NvmlBackend) | pynvml | `cuda:N` |
| **Apple** | `_gpu_apple.py` (AppleSiliconBackend) | IOKit (ctypes) | `mps:0` |

**백엔드 추상화:**

```python
@runtime_checkable
class GPUBackend(Protocol):
    vendor: str
    def discover(self) -> list[DiscoveredGPU]: ...
    def collect(self, handle: Any) -> _GPUSnapshot: ...
    def shutdown(self) -> None: ...
```

**벤더 전달:**
- SDK가 `gpu.N.vendor` OTLP 속성으로 벤더 정보 전송
- Server는 이 속성을 파싱하여 GPU 레지스트리에 저장
- 이전 SDK 호환: vendor 속성 없으면 "NVIDIA" fallback

**Apple Silicon 특성:**
- 칩당 GPU 1개 (MIG 없음, Multi-GPU 없음)
- UUID: `APPLE-{sha256(chip+hostname)[:12]}` (하드웨어 UUID 없으므로 deterministic hash)
- 통합 메모리: `memory_total_gb` = 전체 시스템 메모리 (CPU/GPU 공유)
- 일부 메트릭 unavailable → 0으로 보고 (temperature, clock)

**미래 벤더:**

```python
class AMDBackend(GPUBackend):
    """pyrsmi 기반 — 미구현"""
    pass

class TPUBackend(GPUBackend):
    """Cloud TPU API 기반 — 미구현"""
    pass
```

### 9.3 Auto-Optimization Recommendations (v0.4)

```python
# 자동 분석 및 추천
class OptimizationEngine:
    def analyze(self, traces: List[Trace]) -> List[Recommendation]:
        recommendations = []

        # 1. Batch size 최적화
        if avg_gpu_util < 50:
            recommendations.append(
                Recommendation(
                    type="increase_batch_size",
                    reason="GPU utilization is low",
                    expected_improvement="30% throughput increase"
                )
            )

        # 2. 모델 양자화 제안
        if avg_memory_usage > 80:
            recommendations.append(
                Recommendation(
                    type="quantization",
                    reason="Memory pressure detected",
                    expected_improvement="50% memory reduction"
                )
            )

        return recommendations
```

---

## Appendix

### A. Glossary

| Term | Definition |
|------|------------|
| **Trace** | 하나의 추론 요청 전체를 나타내는 컨테이너 |
| **Span** | 추론 내의 개별 작업 단위 |
| **TTFT** | Time To First Token - 첫 토큰 생성까지 시간 |
| **TPOT** | Time Per Output Token - 토큰당 생성 시간 |
| **MIG** | Multi-Instance GPU - NVIDIA GPU 분할 기술 |
| **OTel** | OpenTelemetry - 분산 추적 표준 |

### B. References

- [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/)
- [NVIDIA NVML API](https://developer.nvidia.com/nvidia-management-library-nvml)
- [ClickHouse Documentation](https://clickhouse.com/docs)
- [vLLM Project](https://github.com/vllm-project/vllm)

---

*This document is a living artifact and will be updated as the project evolves.*
