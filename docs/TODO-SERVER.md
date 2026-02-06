# TODO: Backend Server

> MVP 목표: OTLP 수신, 저장, 조회 API 제공

---

## 1. 프로젝트 셋업

- [ ] 언어/프레임워크 결정 (Go 또는 Rust)
- [ ] 프로젝트 구조 생성
- [ ] 의존성 설정 (gRPC, ClickHouse client, PostgreSQL client)
- [ ] 설정 파일 구조 (YAML 또는 환경변수)

---

## 2. Ingest Service (OTLP 수신)

### 2.1 gRPC 서버
- [ ] OTLP gRPC 엔드포인트 구현 (`/opentelemetry.proto.collector.trace.v1.TraceService/Export`)
- [ ] HTTP/JSON 엔드포인트 (선택)
- [ ] 요청 검증
- [ ] 압축 해제 (gzip)

### 2.2 데이터 처리
- [ ] OTLP → 내부 Span 모델 변환
- [ ] Axonize 확장 속성 파싱 (ai.*, gpu.*)
- [ ] 배치 버퍼링
- [ ] ClickHouse 벌크 insert

### 2.3 GPU Registry 연동
- [ ] 새 GPU 발견 시 PostgreSQL에 등록
- [ ] resource_uuid → physical_gpu_uuid 매핑 조회

---

## 3. Query Service

### 3.1 Trace 조회
- [ ] trace_id로 단건 조회
- [ ] 시간 범위 + 필터 조회
- [ ] 페이지네이션

### 3.2 Span 조회
- [ ] trace_id의 모든 Span 조회 (계층 구조)
- [ ] 필터링 (model_name, service_name, status)

### 3.3 집계 쿼리
- [ ] 서비스별 평균 duration
- [ ] 모델별 추론 횟수
- [ ] GPU별 사용률 통계
- [ ] 시간대별 추론 처리량

### 3.4 GPU 메트릭
- [ ] 시계열 GPU 상태 조회
- [ ] GPU별 집계

---

## 4. API Service (REST)

### 4.1 Trace API
- [ ] `GET /api/v1/traces` - 목록 조회
- [ ] `GET /api/v1/traces/:trace_id` - 상세 조회
- [ ] `GET /api/v1/traces/:trace_id/spans` - Span 목록

### 4.2 GPU API
- [ ] `GET /api/v1/gpus` - GPU 목록
- [ ] `GET /api/v1/gpus/:resource_uuid` - GPU 상세
- [ ] `GET /api/v1/gpus/:resource_uuid/metrics` - GPU 메트릭 시계열

### 4.3 Analytics API
- [ ] `GET /api/v1/analytics/overview` - 대시보드 요약
- [ ] `GET /api/v1/analytics/latency` - 지연 시간 분포
- [ ] `GET /api/v1/analytics/throughput` - 처리량 추이

### 4.4 공통
- [ ] JSON 응답 포맷
- [ ] 에러 핸들링
- [ ] 요청 로깅

---

## 5. 데이터베이스 연동

### 5.1 ClickHouse
- [ ] 커넥션 풀
- [ ] spans 테이블 CRUD
- [ ] traces 테이블 CRUD
- [ ] gpu_metrics 테이블 CRUD
- [ ] 쿼리 최적화 (인덱스 활용)

### 5.2 PostgreSQL
- [ ] 커넥션 풀
- [ ] physical_gpus 테이블 CRUD
- [ ] compute_resources 테이블 CRUD
- [ ] resource_contexts 테이블 CRUD

---

## 6. 운영

- [ ] Health check 엔드포인트
- [ ] 메트릭 노출 (Prometheus 포맷)
- [ ] 구조화된 로깅
- [ ] Graceful shutdown

---

## 7. 테스트

- [ ] 단위 테스트
- [ ] 통합 테스트 (DB 연동)
- [ ] 부하 테스트 (10K spans/sec 목표)

---

## 8. 우선순위

| 순서 | 항목 | 이유 |
|------|------|------|
| 1 | 프로젝트 셋업 | 기반 |
| 2 | Ingest Service | SDK 연동 필수 |
| 3 | ClickHouse 연동 | 데이터 저장 |
| 4 | 기본 Query API | 대시보드 연동 |
| 5 | PostgreSQL (GPU Registry) | GPU 관리 |
| 6 | Analytics API | 대시보드 고도화 |
