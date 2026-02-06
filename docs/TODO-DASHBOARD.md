# TODO: Dashboard (React)

> MVP 목표: Trace 조회, GPU 모니터링 기본 UI

---

## 1. 프로젝트 셋업

- [x] React + TypeScript 프로젝트 생성 (Vite 권장)
- [x] UI 프레임워크 선택 (Tailwind + shadcn/ui 권장)
- [x] 차트 라이브러리 선택 (Recharts 또는 Apache ECharts)
- [x] 상태 관리 (React Query 권장)
- [x] 라우팅 (React Router)

---

## 2. 레이아웃

- [x] 사이드바 네비게이션
- [ ] 헤더 (서비스 선택, 환경 선택, 시간 범위)
- [x] 반응형 레이아웃

---

## 3. 대시보드 (Overview)

### 3.1 요약 카드
- [x] 총 추론 수 (오늘/어제 비교)
- [x] 평균 지연 시간
- [x] 에러율
- [x] 활성 GPU 수

### 3.2 차트
- [x] 시간대별 추론 처리량 (라인 차트)
- [x] 지연 시간 분포 (히스토그램)
- [ ] 서비스별 추론 비율 (파이 차트)

---

## 4. Traces 페이지

### 4.1 목록 뷰
- [x] 테이블: trace_id, 시작 시간, duration, 상태, 서비스
- [x] 필터: 시간 범위, 서비스, 상태, 모델
- [x] 정렬: 시간, duration
- [x] 페이지네이션

### 4.2 상세 뷰
- [x] Trace 기본 정보
- [x] Span 타임라인 (Gantt 스타일)
- [x] Span 계층 구조 (트리 뷰)
- [x] 개별 Span 상세 (클릭 시)
  - [x] 타이밍 정보
  - [x] 모델 정보
  - [x] GPU 정보
  - [x] Attributes

---

## 5. GPUs 페이지

### 5.1 GPU 목록
- [x] 카드 뷰: 각 GPU 상태 요약
- [ ] 테이블 뷰: 상세 정보
- [ ] 필터: 노드, 모델, 타입 (Full/MIG)

### 5.2 GPU 상세
- [x] 기본 스펙 정보
- [x] 실시간 메트릭 (utilization, memory, power)
- [x] 시계열 차트 (지난 1시간/24시간)
- [ ] 해당 GPU에서 실행된 최근 Span 목록

---

## 6. Analytics 페이지 (선택)

- [ ] 모델별 성능 비교
- [ ] GPU 효율성 분석
- [ ] 비용 분석 (cost_usd 기반)

---

## 7. 공통 컴포넌트

- [x] 로딩 스피너
- [x] 에러 상태 표시
- [x] 빈 상태 표시
- [ ] 시간 범위 선택기
- [x] 검색 입력
- [ ] 필터 드롭다운

---

## 8. API 연동

- [x] API 클라이언트 설정 (axios 또는 fetch)
- [x] React Query hooks
  - [x] useTraces()
  - [x] useTrace(traceId)
  - [x] useGpus()
  - [x] useGpuMetrics(resourceUuid)
  - [x] useAnalytics()
- [x] 에러 핸들링
- [x] 로딩 상태 관리

---

## 9. 테스트

- [ ] 컴포넌트 테스트 (Vitest + Testing Library)
- [ ] E2E 테스트 (Playwright) - 선택

---

## 10. 우선순위

| 순서 | 항목 | 이유 |
|------|------|------|
| 1 | 프로젝트 셋업 | 기반 |
| 2 | 레이아웃 + 라우팅 | 구조 |
| 3 | Traces 목록/상세 | 핵심 기능 |
| 4 | 대시보드 Overview | 첫인상 |
| 5 | GPUs 페이지 | 차별화 |
| 6 | Analytics | 고도화 |
