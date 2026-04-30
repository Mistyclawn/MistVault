# MistVault 👻

맥 미니(M4)를 활용한 초경량 원격 저장소 & 미디어 스트리밍 보관함

---

## 개요

MistVault는 MistyClawn을 위해 설계된, M4 맥 미니 전용 저전력 원격 파일 저장 및 미디어 스트리밍 시스템이다. 

- **운영 환경:** 애플 실리콘(M4) 맥 미니
- **컨셉:** 도커를 쓰지 않는 네이티브 설치 방식으로 AcidClaw, MistClaw 등 LLM/비서 에이전트의 리소스 충돌 없이 구동
- **주요 기능:**
  - 외부/내부 어디서나 안전하게 파일 접근 (Tailscale, SFTP, FileBrowser 지원)
  - 미디어 스트리밍 (Jellyfin)
  - 관리/제어 자동화 스크립트

---

## 설계 배경

도커를 포함한 상시 서버 운영 시, Mac mini의 자원이 부족해 AI/LLM 어시스턴트(Buddy, MistClaw 등)의 작동이 어려울 수 있음. 따라서 네이티브(bare-metal) 방식으로 최고 효율/저점유 구조를 택함.

---

## 핵심 구성

1. **Jellyfin (미디어 서버)**
    - macOS 네이티브용 바이너리 사용 → 도커/VM보다 램 점유 최소화
    - 비디오, 음원 등 미디어 스트리밍

2. **FileBrowser (웹 파일 관리자)**
    - 단일 실행 파일 (Go 기반, 극저점유)
    - 브라우저 기반 웹 파일 관리

3. **Tailscale/SFTP**
    - 외부 접속은 Tailscale / 내부 연결은 macOS 기본 SFTP 서버로 처리
    - 별도의 포트포워딩 필요 없음 (MagicDNS 활용)

4. **자동화 스크립트**
    - 서버 온/오프/상태점검 스크립트 제공 (추가 예정)

---

## 설치 및 세팅

1. Homebrew 설치 및 도구 설치
    ```sh
    brew install --cask jellyfin
    brew install filebrowser
    ```
2. Jellyfin.app 실행 (응용프로그램에서 실행 가능)
3. filebrowser 실행: `/opt/homebrew/bin/filebrowser -r /Volumes/ROGALLY/`
4. Tailscale 및 SFTP는 macOS 시스템 환경설정에서 활성화

---

## 앞으로 할 일 (TODO)
- [ ] Jellyfin 기본 설정/미디어 라이브러리 경로 잡기
- [ ] FileBrowser 설정 파일 및 접속 계정 암호화
- [ ] 자동화 스크립트 (서버 시작/중지/상태) 추가
- [ ] 상세 세팅 및 문서화 계속 보강
