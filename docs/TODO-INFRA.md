# TODO: Infrastructure

> MVP 목표: 로컬 개발 환경 + Docker Compose 배포

---

## 1. 데이터베이스 스키마

### 1.1 ClickHouse
- [ ] spans 테이블 생성 스크립트
- [ ] traces 테이블 생성 스크립트
- [ ] gpu_metrics 테이블 생성 스크립트
- [ ] 인덱스 설정
- [ ] TTL 설정 (spans: 30일, traces: 90일, gpu_metrics: 7일)
- [ ] 마이그레이션 스크립트 관리 방안

### 1.2 PostgreSQL
- [ ] physical_gpus 테이블 생성 스크립트
- [ ] compute_resources 테이블 생성 스크립트
- [ ] resource_contexts 테이블 생성 스크립트
- [ ] 인덱스 설정 (labels JSONB GIN 인덱스)
- [ ] 마이그레이션 도구 선택 (Flyway, golang-migrate 등)

---

## 2. Docker 설정

### 2.1 개별 이미지
- [ ] SDK 테스트용 Python 이미지
- [ ] Server 이미지 (멀티스테이지 빌드)
- [ ] Dashboard 이미지 (nginx + static)

### 2.2 Docker Compose (개발용)
- [ ] docker-compose.yml 작성
  - [ ] ClickHouse 서비스
  - [ ] PostgreSQL 서비스
  - [ ] Redis 서비스 (선택)
  - [ ] Server 서비스
  - [ ] Dashboard 서비스
- [ ] 볼륨 설정 (데이터 영속성)
- [ ] 네트워크 설정
- [ ] 환경변수 파일 (.env.example)

### 2.3 Docker Compose (프로덕션)
- [ ] 리소스 제한 설정
- [ ] 헬스체크 설정
- [ ] 로그 드라이버 설정

---

## 3. 로컬 개발 환경

- [ ] Makefile 또는 Taskfile
  - [ ] `make dev` - 전체 스택 실행
  - [ ] `make test` - 테스트 실행
  - [ ] `make build` - 이미지 빌드
  - [ ] `make clean` - 정리
- [ ] 로컬 ClickHouse/PostgreSQL 실행 스크립트
- [ ] 샘플 데이터 시드 스크립트

---

## 4. 설정 관리

- [ ] 서버 설정 파일 구조 (config.yaml)
  ```yaml
  server:
    port: 4317
  clickhouse:
    host: localhost
    port: 9000
  postgresql:
    host: localhost
    port: 5432
  ```
- [ ] 환경변수 오버라이드
- [ ] 설정 검증

---

## 5. 모니터링 (선택)

- [ ] 서버 메트릭 노출 (/metrics)
- [ ] Grafana 대시보드 템플릿 (JSON)
- [ ] 알림 룰 (Prometheus Alertmanager)

---

## 6. CI/CD (선택)

- [ ] GitHub Actions 워크플로우
  - [ ] 테스트 실행
  - [ ] 이미지 빌드
  - [ ] 이미지 푸시 (ghcr.io 또는 Docker Hub)
- [ ] 버전 태깅 전략

---

## 7. 우선순위

| 순서 | 항목 | 이유 |
|------|------|------|
| 1 | DB 스키마 스크립트 | 개발 시작 전 필수 |
| 2 | Docker Compose (개발) | 로컬 환경 구축 |
| 3 | Makefile | 개발 편의성 |
| 4 | 설정 관리 | 서버 개발 시 필요 |
| 5 | Docker 이미지 | 배포 준비 |
| 6 | CI/CD | 자동화 |
