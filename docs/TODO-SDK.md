# TODO: Python SDK (axonize-py)

> MVP 목표: 추론 Span 수집 및 OTLP 전송

---

## 1. 프로젝트 셋업

- [ ] Python 패키지 구조 생성 (`axonize/`)
- [ ] pyproject.toml 설정 (의존성: opentelemetry-api, pynvml, grpcio)
- [ ] 개발 환경 설정 (pytest, ruff, mypy)

---

## 2. Core API

### 2.1 초기화
- [ ] `axonize.init()` 구현
  - [ ] endpoint 설정
  - [ ] service_name, environment 설정
  - [ ] batch_size, flush_interval_ms 설정
  - [ ] sampling_rate 설정

### 2.2 Trace/Span
- [ ] `@axonize.trace()` 데코레이터
- [ ] `axonize.span()` context manager
- [ ] Span 계층 구조 (parent_span_id)
- [ ] `span.set_attribute()` 구현
- [ ] `span.set_gpus()` 구현
- [ ] SpanKind (INTERNAL, CLIENT, SERVER)
- [ ] SpanStatus (OK, ERROR)

### 2.3 LLM 특화
- [ ] `axonize.llm_span()` context manager
- [ ] `span.record_token()` - TTFT, TPOT 자동 계산
- [ ] tokens_input, tokens_output 자동 수집

---

## 3. GPU 프로파일링

- [ ] pynvml 래퍼 구현
- [ ] GPU 디바이스 탐지 (Full GPU, MIG)
- [ ] resource_uuid 추출 (MIG UUID 또는 GPU UUID)
- [ ] physical_gpu_uuid 추출
- [ ] 런타임 메트릭 수집
  - [ ] utilization_percent
  - [ ] memory_used_gb / memory_total_gb
  - [ ] temperature_celsius
  - [ ] power_watts
  - [ ] clock_mhz
- [ ] 비동기 수집 (추론 스레드 블로킹 방지)

---

## 4. 데이터 전송

### 4.1 버퍼링
- [ ] Ring buffer 구현 (또는 Queue 사용)
- [ ] 배치 처리 (batch_size 도달 시 전송)
- [ ] 주기적 flush (flush_interval_ms)

### 4.2 OTLP Export
- [ ] gRPC exporter 구현
- [ ] Span → OTLP 포맷 변환
- [ ] 압축 (gzip)
- [ ] 재시도 로직 (네트워크 오류 시)
- [ ] Graceful degradation (서버 장애 시 로컬 버퍼링)

---

## 5. OTel 호환성

- [ ] OTel Semantic Conventions 준수
- [ ] Axonize 확장 속성 (ai.*, gpu.*, cost.*)
- [ ] 기존 OTel TracerProvider와 공존 가능하도록

---

## 6. 테스트

- [ ] 단위 테스트 (Span 생성, GPU 메트릭)
- [ ] 통합 테스트 (OTLP 전송)
- [ ] 오버헤드 벤치마크 스크립트
- [ ] 예제 코드 (Diffusers, vLLM)

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
