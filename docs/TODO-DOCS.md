# TODO: Documentation

> MVP 목표: 사용자가 30분 내에 설치하고 시작할 수 있는 문서

---

## 1. README

- [ ] 프로젝트 소개 (한 문장)
- [ ] 핵심 기능 목록
- [ ] 스크린샷/데모 GIF
- [ ] Quick Start (5분 내)
- [ ] 상세 설치 가이드 링크
- [ ] 라이선스

---

## 2. 시작 가이드

### 2.1 설치
- [ ] Docker Compose로 서버 실행
- [ ] SDK 설치 (`pip install axonize`)
- [ ] 기본 설정

### 2.2 첫 번째 Trace
- [ ] 간단한 예제 코드
- [ ] 대시보드에서 확인하는 방법

---

## 3. SDK 문서

### 3.1 API Reference
- [ ] `axonize.init()` 옵션
- [ ] `@axonize.trace()` 데코레이터
- [ ] `axonize.span()` context manager
- [ ] `axonize.llm_span()` LLM 특화
- [ ] Span attributes

### 3.2 예제
- [ ] 기본 사용법
- [ ] Diffusers 통합
- [ ] vLLM 통합
- [ ] 커스텀 모델

### 3.3 고급 설정
- [ ] 샘플링 설정
- [ ] 배치 설정
- [ ] GPU 프로파일링 설정

---

## 4. 서버 문서

### 4.1 설치/배포
- [ ] Docker Compose
- [ ] Kubernetes (Helm) - 나중에
- [ ] 설정 옵션

### 4.2 API Reference
- [ ] REST API 엔드포인트
- [ ] 요청/응답 예시
- [ ] 에러 코드

---

## 5. 대시보드 문서

- [ ] 주요 화면 설명
- [ ] 필터 사용법
- [ ] Trace 분석 방법

---

## 6. 운영 가이드

- [ ] 백업/복구
- [ ] 스케일링
- [ ] 트러블슈팅 FAQ

---

## 7. 기여 가이드

- [ ] CONTRIBUTING.md
- [ ] 코드 스타일
- [ ] PR 프로세스
- [ ] 개발 환경 설정

---

## 8. 문서 플랫폼

- [ ] 플랫폼 선택 (Docusaurus, MkDocs, GitBook 등)
- [ ] 도메인 설정 (docs.axonize.io 등)
- [ ] 검색 기능
- [ ] 버전 관리

---

## 9. 우선순위

| 순서 | 항목 | 이유 |
|------|------|------|
| 1 | README | 첫인상 |
| 2 | Quick Start | 즉시 시작 |
| 3 | SDK 기본 사용법 | 핵심 |
| 4 | SDK 예제 (Diffusers, vLLM) | 타겟 유저 |
| 5 | 서버 설치 | 운영 |
| 6 | 대시보드 가이드 | 사용성 |
