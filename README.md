# MistVault 👻

맥 미니(M4)를 활용한 초경량 원격 저장소 & 미디어 스트리밍 게이트웨이

---

## 개요

MistVault는 MistyClawn을 위해 설계된, M4 맥 미니 전용 저전력 원격 파일 저장 및 미디어 스트리밍 시스템이다. 

- **운영 환경:** 애플 실리콘(M4) 맥 미니
- **컨셉:** 도커를 쓰지 않는 네이티브 빌드 방식으로 시스템 리소스(RAM/CPU) 점유를 최소화하여 AcidClaw, MistClaw 등 에이전트와 공존 가능
- **주요 기능:**
  - **MistVault Gateway (Port 8088):** 서비스 상태 모니터링 및 통합 대시보드
  - **MistVault Storage (Port 8097):** Go 네이티브로 구현된 고성능 파일 탐색기 (SPA 방식 UX)
  - **Jellyfin (Port 8096):** 미디어 스트리밍 서버

---

## 핵심 구성 요소

### 1. MistVault Gateway (dashboard.js)
- **자동 감지:** 시스템의 테일스케일(Tailscale) IP를 자동으로 찾아내어 외부 접속 링크 제공
- **워치독(Watchdog):** 스토리지 서버 등 핵심 프로세스가 죽으면 10초 내에 자동으로 부활시킴
- **UI:** 모던 다크 테마 기반의 반응형 게이트웨이

### 2. MistVault Storage (main.go -> mistvault_storage_server)
- **M4 Native:** Go 언어로 빌드되어 극도로 낮은 메모리 점유율 보장
- **Explorer UX:** 
  - 페이지 이동 없는 SPA(Single Page Application) 방식 탐색
  - 더블 클릭으로 폴더 진입 및 파일 다운로드
  - 브레드크럼을 이용한 빠른 경로 이동
- **Root:** `/Volumes` (맥에 연결된 모든 외장 드라이브 접근 가능)

### 3. Jellyfin
- **Native App:** macOS 전용 바이너리 구동으로 하드웨어 가속 트랜스코딩 지원

---

## 설치 및 실행

### 1. 바이너리 빌드
M4 Mac mini(Apple Silicon) 환경에 최적화된 바이너리를 생성하려면 아래 명령어를 실행한다:
```sh
# 현재 디렉토리에서 Go 소스를 빌드
GOOS=darwin GOARCH=arm64 go build -o mistvault_storage_server main.go

# 실행 권한 부여
chmod +x mistvault_storage_server
```

### 2. 서비스 실행
- **수동 실행:** `./mistvault_storage_server`
- **자동 실행:** `com.mistyclawn.mistvault.dashboard.plist`를 LaunchAgent에 등록하면 대시보드가 스토리지 서버를 자동으로 감시하고 실행함.

---

## 📂 프로젝트 구조
- `main.go`: 스토리지 서버 소스 (Go)
- `dashboard.js`: 게이트웨이 대시보드 (Node.js)
- `guard.sh`: 볼륨 마운트 및 서비스 체크 스크립트
- `mistvault_storage_server`: 빌드된 바이너리

---

## 앞으로 할 일 (TODO)
- [ ] YTDLP 이식: 대시보드 내 원격 다운로드 인터페이스 추가
- [ ] 보안 강화: 대시보드 및 스토리지 접근 시 비밀번호/토큰 인증 레이어 추가
- [ ] 로그 뷰어: 대시보드에서 시스템 로그 직접 확인 기능
