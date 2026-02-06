# TODO: Python SDK (axonize-py)

> MVP 목표: 추론 Span 수집 및 OTLP 전송

---

## 1. 프로젝트 셋업

- [x] Python 패키지 구조 생성 (`axonize/`)
- [x] pyproject.toml 설정 (의존성: opentelemetry-api, pynvml, grpcio)
- [x] 개발 환경 설정 (pytest, ruff, mypy)

---

## 2. Core API

### 2.1 초기화
- [x] `axonize.init()` 구현
  - [x] endpoint 설정
  - [x] service_name, environment 설정
  - [x] batch_size, flush_interval_ms 설정
  - [x] sampling_rate 설정

### 2.2 Trace/Span
- [x] `@axonize.trace()` 데코레이터
- [x] `axonize.span()` context manager
- [x] Span 계층 구조 (parent_span_id)
- [x] `span.set_attribute()` 구현
- [x] `span.set_gpus()` 구현
- [x] SpanKind (INTERNAL, CLIENT, SERVER)
- [x] SpanStatus (OK, ERROR)

### 2.3 LLM 특화
- [x] `axonize.llm_span()` context manager
- [x] `span.record_token()` - TTFT, TPOT 자동 계산
- [x] tokens_input, tokens_output 자동 수집

---

## 3. GPU 프로파일링

- [x] pynvml 래퍼 구현
- [x] GPU 디바이스 탐지 (Full GPU, MIG)
- [x] resource_uuid 추출 (MIG UUID 또는 GPU UUID)
- [x] physical_gpu_uuid 추출
- [x] 런타임 메트릭 수집
  - [x] utilization_percent
  - [x] memory_used_gb / memory_total_gb
  - [x] temperature_celsius
  - [x] power_watts
  - [x] clock_mhz
- [x] 비동기 수집 (추론 스레드 블로킹 방지)

---

## 4. 데이터 전송

### 4.1 버퍼링
- [x] Ring buffer 구현 (또는 Queue 사용)
- [x] 배치 처리 (batch_size 도달 시 전송)
- [x] 주기적 flush (flush_interval_ms)

### 4.2 OTLP Export
- [x] gRPC exporter 구현
- [x] Span → OTLP 포맷 변환
- [x] 압축 (gzip)
- [x] 재시도 로직 (네트워크 오류 시)
- [x] Graceful degradation (서버 장애 시 로컬 버퍼링)

---

## 5. OTel 호환성

- [x] OTel Semantic Conventions 준수
- [x] Axonize 확장 속성 (ai.*, gpu.*, cost.*)
- [x] 기존 OTel TracerProvider와 공존 가능하도록

---

## 6. 테스트

- [x] 단위 테스트 (Span 생성, GPU 메트릭)
- [x] 통합 테스트 (OTLP 전송)
- [x] 오버헤드 벤치마크 스크립트
- [x] 예제 코드 (Diffusers, vLLM)

---

## 7. 우선순위

| 순서 | 항목 | 이유 |
|------|------|------|
| 1 | 프로젝트 셋업 | 기반 |
| 2 | Core API (Trace/Span) | 핵심 기능 |
| 3 | OTLP Export | 서버 연동 필수 |
| 4 | GPU 프로파일링 | 차별화 기능 |
| 5 | LLM 특화 | 타겟 유저 |
| 6 | 테스트 | 품질 보증 |
