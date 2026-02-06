# Axonize Milestone Roadmap

> **Version**: 0.1.0
> **Last Updated**: 2026-02-06
> **Strategy**: Personal Edition 먼저 → Datacenter Edition 확장 (Bottom-up)

---

## Overview

```
Part 1: Personal Edition (M0 ~ M5)
──────────────────────────────────────────────────────────

  M0          M1          M2          M3          M4          M5
  Foundation  SDK Core    Pipeline    GPU Prof.   Dashboard   Launch
  ─────────  ─────────   ─────────   ─────────   ─────────   ─────────
  모노레포    trace/span  E2E 파이프  GPU 메트릭  UI v1       pip install
  셋업       생성        라인 완성   수집        시각화      axonize

                              ↓ v0.1 Release

Part 2: Datacenter Edition (M6 ~ M9)
──────────────────────────────────────────────────────────

  M6          M7          M8          M9
  Multi-Node  Orchestr.   Enterprise  Scale
  ─────────   ─────────   ─────────   ─────────
  노드 에이전트 K8s/SLURM  RBAC/SSO   샤딩/1억+
  클러스터 뷰  분산 추론   멀티테넌시  Adaptive
```

---

## Part 1: Personal Edition

> 목표: 단일 노드에서 `pip install axonize` 한 줄로 AI 추론 옵저버빌리티 시작
> 대상: Self-hosted 추론 서버를 운영하는 개인/스타트업 ML 엔지니어

---

### M0: Foundation

**핵심 목표**: 모노레포 셋업, 기술 스택 확정, 개발 환경 구축

#### 완료 기준 (Definition of Done)

- [ ] 모노레포 구조가 확정되고 각 패키지 디렉토리가 생성됨
- [ ] 언어/프레임워크가 결정되고 문서화됨
- [ ] `docker compose up`으로 ClickHouse + PostgreSQL이 실행됨
- [ ] DB 스키마 마이그레이션이 자동 실행됨
- [ ] `make dev` / `make test` 명령이 동작함
- [ ] CI (GitHub Actions)에서 lint/test가 통과함

#### 주요 작업 항목

| 작업 | TODO 파일 매핑 |
|------|---------------|
| 모노레포 구조 결정 (`packages/sdk-py`, `packages/server`, `packages/dashboard`) | - |
| 서버 언어 결정 (Go vs Rust) | TODO-SERVER §1 |
| Python SDK 패키지 구조 생성, pyproject.toml | TODO-SDK §1 |
| 개발 도구 설정 (pytest, ruff, mypy) | TODO-SDK §1 |
| ClickHouse 스키마 스크립트 (spans, traces, gpu_metrics) | TODO-INFRA §1.1 |
| PostgreSQL 스키마 스크립트 (physical_gpus, compute_resources, resource_contexts) | TODO-INFRA §1.2 |
| Docker Compose (개발용): ClickHouse + PostgreSQL | TODO-INFRA §2.2 |
| Makefile (`make dev`, `make test`, `make build`, `make clean`) | TODO-INFRA §3 |
| 설정 파일 구조 (config.yaml), 환경변수 | TODO-INFRA §4 |
| GitHub Actions 기본 워크플로우 (lint, test) | TODO-INFRA §6 |

#### 기술 결정 사항

| 결정 항목 | 후보 | 결정 기준 |
|-----------|------|-----------|
| 서버 언어 | **Go** vs Rust | Go: 빠른 개발, gRPC 생태계 성숙. Rust: 성능 최적, 학습 곡선 높음 |
| 모노레포 도구 | Turborepo / 단순 Makefile | 프로젝트 규모에 맞게 선택 |
| DB 마이그레이션 | golang-migrate / Flyway | 서버 언어에 따라 결정 |
| Python 패키지 매니저 | uv / poetry / hatch | 빌드 속도, 표준 준수 |

#### 산출물

- `packages/` 디렉토리 구조
- `docker-compose.dev.yml`
- DB 마이그레이션 스크립트 (`migrations/`)
- `Makefile`
- `.github/workflows/ci.yml`
- `.env.example`

#### 의존성

- 없음 (시작점)

---

### M1: SDK Core

**핵심 목표**: `axonize.init()`, `@trace`, `span()` — 추론 스레드에서 Span 생성

#### 완료 기준 (Definition of Done)

- [ ] `axonize.init(endpoint=..., service_name=...)` 호출로 SDK 초기화 가능
- [ ] `@axonize.trace()` 데코레이터로 Trace 생성
- [ ] `with axonize.span("name")` context manager로 Span 생성
- [ ] Span 계층 구조 (parent-child) 자동 관리
- [ ] `span.set_attribute()` 로 메타데이터 첨부
- [ ] Ring buffer에 Span이 비동기로 enqueue됨
- [ ] 단위 테스트 통과 (Span 생성, 계층 구조, attribute 설정)

#### 주요 작업 항목

| 작업 | TODO 파일 매핑 |
|------|---------------|
| `axonize.init()` 구현 (endpoint, service_name, environment, batch/flush 설정) | TODO-SDK §2.1 |
| `@axonize.trace()` 데코레이터 | TODO-SDK §2.2 |
| `axonize.span()` context manager | TODO-SDK §2.2 |
| Span 계층 구조 (parent_span_id, Context Propagation) | TODO-SDK §2.2 |
| `span.set_attribute()` 구현 | TODO-SDK §2.2 |
| SpanKind (INTERNAL, CLIENT, SERVER), SpanStatus (OK, ERROR) | TODO-SDK §2.2 |
| Ring buffer 구현 (lock-free, < 100ns enqueue) | TODO-SDK §4.1 |
| 배치 처리 (batch_size 도달 또는 flush_interval 경과 시 drain) | TODO-SDK §4.1 |
| 단위 테스트 (Span 생성, 계층, 속성) | TODO-SDK §6 |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| Context Propagation | Python `contextvars` 사용하여 현재 Span 추적 |
| Ring Buffer | `collections.deque` 기반 또는 직접 구현 (성능 벤치마크 후 결정) |
| Thread Safety | 추론 스레드 → Buffer enqueue는 lock-free, Background 스레드가 drain |

#### 산출물

- `packages/sdk-py/axonize/` 코어 모듈
- `axonize.init()`, `@trace`, `span()` API
- Ring buffer 구현체
- 단위 테스트 스위트

#### 의존성

- **M0** (프로젝트 구조, Python 패키지 셋업)

---

### M2: Pipeline

**핵심 목표**: OTLP 서버 + ClickHouse 저장 — SDK → 서버 E2E 파이프라인

#### 완료 기준 (Definition of Done)

- [ ] SDK에서 Span을 OTLP gRPC로 전송 가능
- [ ] 서버가 OTLP gRPC 엔드포인트로 Span 수신
- [ ] 수신된 Span이 ClickHouse `spans` 테이블에 저장됨
- [ ] 기본 Query API (`GET /api/v1/traces`, `GET /api/v1/traces/:id`)가 동작
- [ ] E2E 테스트: SDK에서 Span 전송 → 서버 저장 → API 조회까지 확인
- [ ] 10K spans/sec 수신 가능 (부하 테스트)

#### 주요 작업 항목

| 작업 | TODO 파일 매핑 |
|------|---------------|
| **SDK: OTLP gRPC exporter** | TODO-SDK §4.2 |
| Span → OTLP 포맷 변환 | TODO-SDK §4.2 |
| gzip 압축 | TODO-SDK §4.2 |
| 재시도 로직, Graceful degradation | TODO-SDK §4.2 |
| OTel Semantic Conventions 준수, Axonize 확장 속성 | TODO-SDK §5 |
| **Server: OTLP gRPC 엔드포인트** | TODO-SERVER §2.1 |
| OTLP → 내부 모델 변환, 확장 속성 파싱 | TODO-SERVER §2.2 |
| ClickHouse 벌크 insert (배치 버퍼링) | TODO-SERVER §2.2, §5.1 |
| ClickHouse 커넥션 풀, spans/traces 테이블 CRUD | TODO-SERVER §5.1 |
| **Server: 기본 Query API** | TODO-SERVER §3, §4 |
| `GET /api/v1/traces` (목록, 시간 범위, 필터, 페이지네이션) | TODO-SERVER §4.1 |
| `GET /api/v1/traces/:trace_id` (상세, Span 계층) | TODO-SERVER §4.1 |
| JSON 응답 포맷, 에러 핸들링 | TODO-SERVER §4.4 |
| Health check, 구조화된 로깅, Graceful shutdown | TODO-SERVER §6 |
| E2E 테스트, 부하 테스트 | TODO-SERVER §7 |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| gRPC 프레임워크 | Go: `google.golang.org/grpc`, Rust: `tonic` |
| ClickHouse 클라이언트 | Go: `clickhouse-go`, Rust: `clickhouse-rs` |
| 벌크 insert 전략 | 서버 내 배치 버퍼 → 주기적 flush (1초 또는 1000개) |
| API 프레임워크 | Go: `chi` / `echo` / `fiber`, Rust: `axum` |

#### 산출물

- SDK gRPC exporter 모듈
- Server ingest service (OTLP 수신)
- Server query service + REST API
- ClickHouse 데이터 레이어
- E2E 테스트 스크립트
- 부하 테스트 스크립트

#### 의존성

- **M0** (DB 스키마, Docker Compose)
- **M1** (SDK Core — Span 생성, Ring buffer)

---

### M3: GPU Profiling

**핵심 목표**: pynvml 연동, 3-Layer GPU Identity, GPU 메트릭 수집

#### 완료 기준 (Definition of Done)

- [ ] SDK가 시스템의 NVIDIA GPU를 자동 탐지 (Full GPU + MIG)
- [ ] 3-Layer Identity 구현: Physical GPU → Compute Resource → Context
- [ ] `span.set_gpus(["cuda:0"])` 호출 시 GPU Attribution 자동 첨부
- [ ] 비동기 GPU 메트릭 수집 (utilization, memory, temperature, power, clock)
- [ ] GPU 메트릭이 서버 → ClickHouse `gpu_metrics` 테이블에 저장
- [ ] 서버 GPU Registry: 새 GPU 발견 시 PostgreSQL에 자동 등록
- [ ] GPU API 동작 (`GET /api/v1/gpus`, `GET /api/v1/gpus/:id/metrics`)
- [ ] 추론 스레드 오버헤드 < 1μs 유지 (GPU 수집은 별도 스레드)

#### 주요 작업 항목

| 작업 | TODO 파일 매핑 |
|------|---------------|
| **SDK: pynvml 래퍼** | TODO-SDK §3 |
| GPU 디바이스 탐지 (Full GPU, MIG 인스턴스) | TODO-SDK §3 |
| resource_uuid / physical_gpu_uuid 추출 | TODO-SDK §3 |
| 런타임 메트릭 수집 (utilization, memory, temp, power, clock) | TODO-SDK §3 |
| 비동기 수집 (별도 스레드, gpu_snapshot_interval_ms) | TODO-SDK §3 |
| `span.set_gpus()` → GPUAttribution 자동 구성 | TODO-SDK §2.2 |
| GPU 속성을 OTel 확장 포맷 (gpu.*) 으로 변환 | TODO-SDK §5 |
| **Server: GPU Registry** | TODO-SERVER §2.3 |
| 새 GPU 발견 시 PostgreSQL physical_gpus/compute_resources에 등록 | TODO-SERVER §2.3, §5.2 |
| resource_contexts 관리 | TODO-SERVER §5.2 |
| gpu_metrics 테이블 쓰기 | TODO-SERVER §5.1 |
| GPU API 엔드포인트 (`/api/v1/gpus`, metrics 시계열) | TODO-SERVER §4.2 |
| GPU별 집계 쿼리 | TODO-SERVER §3.4 |
| 오버헤드 벤치마크 스크립트 | TODO-SDK §6 |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| GPU 탐지 전략 | pynvml 초기화 → `nvmlDeviceGetCount` → MIG enumeration |
| MIG 식별 | `nvmlDeviceGetMigDeviceHandleByIndex` → MIG UUID 추출 |
| 수집 주기 | 기본 100ms (`gpu_snapshot_interval_ms`), 설정 가능 |
| 메트릭 → Span 매핑 | Span 종료 시점에 가장 최근 snapshot을 첨부 |

#### 산출물

- `packages/sdk-py/axonize/gpu/` 모듈 (pynvml 래퍼, 3-Layer Identity)
- GPUAttribution 자동 첨부 로직
- 서버 GPU Registry 서비스
- GPU REST API
- 오버헤드 벤치마크 결과

#### 의존성

- **M2** (E2E 파이프라인 — Span 전송/저장 경로)

---

### M4: Dashboard v1

**핵심 목표**: Trace 목록/상세, GPU 상태, Overview 차트

#### 완료 기준 (Definition of Done)

- [ ] Overview 대시보드: 총 추론 수, 평균 지연, 에러율, 활성 GPU 수
- [ ] 시간대별 추론 처리량 차트, 지연 시간 분포 히스토그램
- [ ] Traces 목록: 필터(시간, 서비스, 상태, 모델) + 정렬 + 페이지네이션
- [ ] Trace 상세: Span 타임라인(Gantt), 계층 구조(트리), 개별 Span 상세
- [ ] GPU 목록: 각 GPU 상태 카드, 필터
- [ ] GPU 상세: 실시간 메트릭 시계열 차트, 최근 Span 목록
- [ ] Docker 이미지로 빌드 및 배포 가능

#### 주요 작업 항목

| 작업 | TODO 파일 매핑 |
|------|---------------|
| React + TypeScript + Vite 프로젝트 생성 | TODO-DASHBOARD §1 |
| Tailwind + shadcn/ui 설정 | TODO-DASHBOARD §1 |
| 차트 라이브러리 (Recharts 또는 ECharts) | TODO-DASHBOARD §1 |
| React Query + React Router 설정 | TODO-DASHBOARD §1 |
| 사이드바 네비게이션, 헤더 (서비스/환경/시간 선택) | TODO-DASHBOARD §2 |
| **Overview 페이지**: 요약 카드 + 처리량/지연 차트 | TODO-DASHBOARD §3 |
| **Traces 목록**: 테이블, 필터, 정렬, 페이지네이션 | TODO-DASHBOARD §4.1 |
| **Trace 상세**: Gantt 타임라인, 트리 뷰, Span 상세 패널 | TODO-DASHBOARD §4.2 |
| **GPU 목록**: 카드 뷰, 테이블 뷰 | TODO-DASHBOARD §5.1 |
| **GPU 상세**: 스펙, 실시간 메트릭 차트, 최근 Span | TODO-DASHBOARD §5.2 |
| API 클라이언트, React Query hooks | TODO-DASHBOARD §8 |
| 공통 컴포넌트 (로딩, 에러, 빈 상태, 시간 선택기) | TODO-DASHBOARD §7 |
| Server: Analytics API (`/api/v1/analytics/overview`, latency, throughput) | TODO-SERVER §4.3 |
| Dashboard Docker 이미지 (nginx + static) | TODO-INFRA §2.1 |
| docker-compose에 Dashboard 서비스 추가 | TODO-INFRA §2.2 |

#### 기술 결정 사항

| 결정 항목 | 후보 | 결정 기준 |
|-----------|------|-----------|
| 차트 라이브러리 | Recharts vs ECharts | Recharts: React 네이티브, 가벼움. ECharts: 고성능, 대량 데이터 |
| Gantt 차트 | 직접 구현 vs 라이브러리 | Jaeger UI 참고하여 직접 구현 권장 (커스터마이징 자유도) |
| 실시간 업데이트 | Polling vs WebSocket | MVP는 Polling (5초), 향후 WebSocket 고려 |

#### 산출물

- `packages/dashboard/` React 앱
- Overview / Traces / GPU 페이지
- Server Analytics API
- Dashboard Docker 이미지
- 통합 docker-compose (전체 스택)

#### 의존성

- **M2** (Query API — Traces, Spans 조회)
- **M3** (GPU API — GPU 메트릭 조회)

---

### M5: Launch Ready

**핵심 목표**: vLLM/Ollama/Diffusers 통합 예제, 문서, `pip install axonize`

#### 완료 기준 (Definition of Done)

- [ ] `pip install axonize`로 PyPI에서 설치 가능
- [ ] vLLM 통합 예제가 동작하고 문서화됨
- [ ] Ollama 통합 예제가 동작하고 문서화됨
- [ ] Diffusers 통합 예제가 동작하고 문서화됨
- [ ] LLM 특화 API (`llm_span`, `record_token`, TTFT/TPOT 자동 계산) 동작
- [ ] README: 프로젝트 소개, Quick Start, 스크린샷/데모
- [ ] 사용자 문서 사이트 배포 (Getting Started, SDK Reference, 예제)
- [ ] `docker compose up` → 전체 스택 5분 내 실행 가능
- [ ] E2E 데모: SDK → Server → Dashboard에서 Trace 시각화 확인
- [ ] 성능 기준 충족: SDK < 1% 오버헤드, Ingest > 10K/sec, Query P95 < 500ms

#### 주요 작업 항목

| 작업 | TODO 파일 매핑 |
|------|---------------|
| **SDK: LLM 특화 API** | TODO-SDK §2.3 |
| `axonize.llm_span()` context manager | TODO-SDK §2.3 |
| `span.record_token()` — TTFT, TPOT 자동 계산 | TODO-SDK §2.3 |
| tokens_input, tokens_output 자동 수집 | TODO-SDK §2.3 |
| **통합 예제** | TODO-SDK §6, TODO-DOCS §3.2 |
| vLLM 통합 예제 + 문서 | TODO-DOCS §3.2 |
| Ollama 통합 예제 + 문서 | TODO-DOCS §3.2 |
| Diffusers 통합 예제 + 문서 | TODO-DOCS §3.2 |
| 커스텀 모델 예제 | TODO-DOCS §3.2 |
| **문서** | TODO-DOCS 전체 |
| README (소개, 기능, 스크린샷, Quick Start) | TODO-DOCS §1 |
| Getting Started (설치, 첫 번째 Trace) | TODO-DOCS §2 |
| SDK API Reference | TODO-DOCS §3.1 |
| SDK 고급 설정 (샘플링, 배치, GPU 프로파일링) | TODO-DOCS §3.3 |
| 서버 설치/배포 문서 | TODO-DOCS §4.1 |
| REST API Reference | TODO-DOCS §4.2 |
| 대시보드 가이드 | TODO-DOCS §5 |
| 문서 플랫폼 설정 (Docusaurus/MkDocs) | TODO-DOCS §8 |
| CONTRIBUTING.md | TODO-DOCS §7 |
| **패키징/배포** | - |
| PyPI 패키지 빌드 및 배포 (`pip install axonize`) | - |
| Server/Dashboard Docker 이미지 게시 (ghcr.io) | TODO-INFRA §6 |
| Docker Compose 프로덕션 설정 (리소스 제한, 헬스체크) | TODO-INFRA §2.3 |
| 샘플 데이터 시드 스크립트 | TODO-INFRA §3 |
| **성능 검증** | - |
| SDK 오버헤드 벤치마크 (< 1%) | TODO-SDK §6 |
| Ingest 부하 테스트 (> 10K spans/sec) | TODO-SERVER §7 |
| Dashboard 응답 시간 확인 (P95 < 500ms) | - |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| PyPI 배포 | `twine` 또는 GitHub Actions에서 자동 배포 |
| 문서 플랫폼 | MkDocs Material (Python 생태계 친화) 또는 Docusaurus |
| 통합 방식 | 프레임워크별 auto-instrumentation vs 명시적 코드 |

#### 산출물

- `axonize` PyPI 패키지 (v0.1.0)
- Server + Dashboard Docker 이미지 (v0.1.0)
- `examples/` 디렉토리 (vLLM, Ollama, Diffusers)
- 문서 사이트
- README.md
- CONTRIBUTING.md
- 성능 벤치마크 리포트

#### 의존성

- **M1 ~ M4** (전체 기능 완성)

---

## Part 2: Datacenter Edition

> 목표: 멀티 노드 GPU 클러스터 환경에서 분산 추론 추적 및 엔터프라이즈 기능
> 대상: GPU 클러스터를 운영하는 Platform 팀, 데이터센터 관리자

---

### M6: Multi-Node

**핵심 목표**: 노드 에이전트, 멀티 노드 디스커버리, 클러스터 뷰

#### 완료 기준 (Definition of Done)

- [ ] 노드 에이전트가 각 머신에서 독립 실행되며 GPU 상태를 주기적 보고
- [ ] 서버가 여러 노드의 에이전트를 자동 디스커버리 및 등록
- [ ] 대시보드에 클러스터 뷰: 전체 노드 목록, 노드별 GPU 상태 요약
- [ ] 노드 간 GPU 사용률 비교 가능
- [ ] Trace에 node_id가 포함되어 어떤 노드에서 실행됐는지 추적 가능

#### 주요 작업 항목

| 작업 | 설명 |
|------|------|
| 노드 에이전트 설계 및 구현 (Go/Rust 바이너리) | 경량 데몬, systemd/Docker로 배포 |
| 에이전트 → 서버 heartbeat + GPU 상태 보고 | gRPC 스트림 또는 주기적 HTTP |
| 서버: 노드 레지스트리 (등록, 상태, 마지막 heartbeat) | PostgreSQL `nodes` 테이블 |
| 서버: 멀티 노드 GPU 집계 쿼리 | ClickHouse + PostgreSQL JOIN |
| 대시보드: 클러스터 Overview 페이지 | 노드 그리드, GPU 히트맵 |
| 대시보드: 노드 상세 페이지 | 노드별 GPU 목록 + 메트릭 |
| SDK: node_id 자동 감지 및 Span에 첨부 | hostname/K8s node name |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| 에이전트 배포 방식 | 바이너리 (systemd) vs DaemonSet (K8s) vs Docker sidecar |
| 디스커버리 프로토콜 | 에이전트 → 서버 등록 (pull 방식) vs mDNS/gossip |
| 에이전트-서버 통신 | gRPC bidirectional stream (실시간) vs HTTP polling |

#### 산출물

- `packages/agent/` 노드 에이전트
- 서버 노드 레지스트리 서비스
- 클러스터 뷰 대시보드 페이지
- 에이전트 설치/배포 가이드

#### 의존성

- **M5** (Personal Edition 완성)

---

### M7: Orchestration

**핵심 목표**: K8s/SLURM 연동, 분산 추론 추적, 토폴로지 시각화

#### 완료 기준 (Definition of Done)

- [ ] K8s 환경에서 자동으로 Pod/Node/Namespace 메타데이터 수집
- [ ] SLURM Job ID와 Trace가 연결되어 Job 단위 추적 가능
- [ ] 분산 추론 (Tensor/Pipeline Parallelism) 시 여러 노드의 Span이 하나의 Trace로 통합
- [ ] 토폴로지 시각화: NVLink/NVSwitch 연결 관계, GPU 간 데이터 흐름
- [ ] Helm 차트로 K8s 배포 가능

#### 주요 작업 항목

| 작업 | 설명 |
|------|------|
| K8s 메타데이터 수집 (Downward API 또는 K8s API) | Pod, Node, Namespace, labels |
| SLURM 연동 (Job ID, partition, node list) | 환경변수 파싱 (`SLURM_JOB_ID` 등) |
| Distributed Context Propagation | NCCL 통신 시 trace context 전파 |
| NVLink/NVSwitch 토폴로지 탐지 | `nvmlDeviceGetTopologyCommonAncestor` |
| 대시보드: 토폴로지 뷰 (GPU 간 연결 그래프) | D3.js 또는 Cytoscape.js |
| 대시보드: 분산 추론 Trace 뷰 (멀티 노드 Gantt) | 노드별 색상 구분 |
| Helm 차트 작성 (Server, Dashboard, Agent) | TODO-DOCS §4.1 |
| K8s Operator (선택): 자동 sidecar injection | - |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| Context Propagation | 텐서 메타데이터 첨부 vs 별도 side-channel |
| K8s 배포 | Helm 차트 vs Kustomize vs Operator |
| 토폴로지 시각화 | D3.js force graph vs 커스텀 SVG |

#### 산출물

- K8s 통합 모듈 (에이전트 + SDK)
- SLURM 통합 모듈
- Distributed tracing 기능
- 토폴로지 시각화 컴포넌트
- Helm 차트 (`charts/axonize/`)
- K8s 배포 가이드

#### 의존성

- **M6** (노드 에이전트, 멀티 노드 기반)

---

### M8: Enterprise

**핵심 목표**: 멀티테넌시, RBAC, 알림 시스템, SSO

#### 완료 기준 (Definition of Done)

- [ ] 멀티테넌시: 조직/팀별 데이터 격리
- [ ] RBAC: Admin, Viewer, Editor 역할 기반 접근 제어
- [ ] SSO: OIDC/SAML 연동 (Okta, Google Workspace 등)
- [ ] 알림 시스템: 임계값 기반 알림 (Slack, Email, Webhook)
- [ ] 감사 로그 (Audit Log): 주요 액션 기록
- [ ] API Key 관리: SDK 인증용 키 발급/폐기

#### 주요 작업 항목

| 작업 | 설명 |
|------|------|
| 멀티테넌시 데이터 모델 (organization_id, team_id) | ClickHouse 파티셔닝, PostgreSQL row-level security |
| 사용자/역할/권한 관리 (PostgreSQL) | users, roles, permissions 테이블 |
| RBAC 미들웨어 (서버) | API 요청별 권한 검증 |
| SSO 연동 (OIDC Provider) | `dex` 또는 직접 구현 |
| 알림 엔진 | 규칙 평가 → 채널별 전송 |
| 알림 채널: Slack, Email, Webhook, PagerDuty | 채널 어댑터 패턴 |
| 대시보드: 팀/조직 관리 UI | 설정 페이지 |
| 대시보드: 알림 규칙 관리 UI | 생성/편집/삭제/테스트 |
| API Key 발급/관리 | SDK → Server 인증 |
| 감사 로그 | 주요 CRUD 작업 기록 |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| 인증 프레임워크 | JWT + OIDC (dex 또는 자체 구현) |
| 테넌시 격리 | 논리적 격리 (organization_id 필터) vs 물리적 격리 (별도 DB) |
| 알림 엔진 | 내장 규칙 엔진 vs 외부 (Prometheus Alertmanager 연동) |

#### 산출물

- 인증/인가 시스템
- 멀티테넌시 데이터 레이어
- 알림 엔진 + 채널 어댑터
- SSO 연동
- 엔터프라이즈 관리 대시보드
- 보안 가이드 문서

#### 의존성

- **M6** (멀티 노드 — 기본 클러스터 관리)
- M7과 병렬 진행 가능

---

### M9: Scale

**핵심 목표**: 샤딩, Adaptive Sampling, 일일 1억+ 추론, Multi-vendor GPU

#### 완료 기준 (Definition of Done)

- [ ] ClickHouse 클러스터 (샤딩 + 레플리케이션) 구성 및 운영
- [ ] Adaptive Sampling: 트래픽에 따라 자동 샘플링 비율 조정
- [ ] 일일 1억+ 추론 처리 (부하 테스트 검증)
- [ ] AMD GPU (ROCm) 기본 지원
- [ ] 데이터 계층화: Hot/Warm/Cold 스토리지
- [ ] 서버 수평 스케일링 (Stateless ingest, 로드밸런서)

#### 주요 작업 항목

| 작업 | 설명 |
|------|------|
| ClickHouse 클러스터 구성 (Distributed 테이블, 샤딩 키) | service_name 또는 trace_id 기반 |
| ClickHouse 레플리케이션 (ZooKeeper/ClickHouse Keeper) | 고가용성 |
| Adaptive Sampling 엔진 | 트래픽 기반 자동 조정 (head-based + tail-based) |
| 서버 수평 스케일링 | Stateless 설계, K8s HPA |
| Hot/Warm/Cold 계층화 | TTL 기반 자동 이동 (SSD → HDD → S3) |
| AMD ROCm 지원 (`pyrsmi` 또는 `rocm-smi` 래퍼) | GPUBackend 추상화 |
| Intel GPU / AWS Inferentia 지원 (선택) | 벤더별 Backend 구현 |
| 대규모 부하 테스트 (1억 spans/day) | k6 또는 커스텀 |
| 성능 튜닝 가이드 문서 | 운영 가이드 |

#### 기술 결정 사항

| 결정 항목 | 설명 |
|-----------|------|
| 샤딩 키 | `sipHash64(trace_id)` (trace 단위 완결성) vs `service_name` (서비스별 격리) |
| Adaptive Sampling | Head-based (결정 시점: 요청 시작) vs Tail-based (결정 시점: 요청 완료) |
| Multi-vendor 추상화 | `GPUBackend` ABC → 벤더별 구현체 |
| 콜드 스토리지 | S3 + ClickHouse S3 engine |

#### 산출물

- ClickHouse 클러스터 배포 매뉴얼
- Adaptive Sampling 모듈
- GPU Backend 추상화 계층 + AMD 구현체
- 대규모 배포 아키텍처 가이드
- 성능 벤치마크 리포트 (1억+ 검증)

#### 의존성

- **M6 ~ M8** (Datacenter 기반 기능)

---

## 마일스톤 의존성 맵

```
M0: Foundation
 └──▶ M1: SDK Core
       └──▶ M2: Pipeline
             └──▶ M3: GPU Profiling
                   └──▶ M4: Dashboard v1
                         └──▶ M5: Launch Ready ── v0.1 Release ──┐
                                                                   │
                               ┌───────────────────────────────────┘
                               ▼
                         M6: Multi-Node
                          ├──▶ M7: Orchestration ──▶ M9: Scale
                          └──▶ M8: Enterprise ─────▶ M9: Scale
```

---

## TODO 파일 → 마일스톤 매핑 요약

| TODO 파일 | M0 | M1 | M2 | M3 | M4 | M5 |
|-----------|:--:|:--:|:--:|:--:|:--:|:--:|
| **TODO-SDK** | §1 | §2.1, §2.2, §4.1 | §4.2, §5 | §3 | - | §2.3, §6 |
| **TODO-SERVER** | §1 | - | §2, §3, §4.1, §4.4, §5.1, §6, §7 | §2.3, §3.4, §4.2, §5.2 | §4.3 | - |
| **TODO-DASHBOARD** | - | - | - | - | §1~§8 | - |
| **TODO-INFRA** | §1, §2.2, §3, §4, §6 | - | - | - | §2.1, §2.2 | §2.3 |
| **TODO-DOCS** | - | - | - | - | - | §1~§8 |

---

*This document is a living artifact and will be updated as milestones are completed.*
