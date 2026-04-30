#!/bin/zsh

# MistVault Storage & Service Guard
# /Volumes/ROGALLY/github/MistVault/guard.sh

TARGET_VOLUME="/Volumes/ROGALLY"
VAULT_PATH="$TARGET_VOLUME/github/MistVault"
FB_BIN="/opt/homebrew/bin/filebrowser"
LOG_FILE="$VAULT_PATH/vault_guard.log"

echo "[$(date)] Guard check started..." >> "$LOG_FILE"

# 1. 마운트 상태 확인
if [ ! -d "$TARGET_VOLUME" ]; then
    echo "❌ [ERROR] $TARGET_VOLUME 이 마운트되지 않았습니다!" >> "$LOG_FILE"
    # 여기서 알림(osascript 등)을 보낼 수도 있음
    exit 1
fi

# 2. 다른 드라이브 오인 방지 (특정 파일 존재 여부 확인)
if [ ! -f "$VAULT_PATH/README.md" ]; then
    echo "⚠️ [WARN] 드라이브는 마운트되었으나 MistVault 경로를 찾을 수 없습니다. 다른 드라이브일 가능성이 있습니다." >> "$LOG_FILE"
    exit 1
fi

echo "✅ Storage OK: $TARGET_VOLUME" >> "$LOG_FILE"

# 3. 서비스 상태 확인 및 필요시 재시작 (예: FileBrowser)
if ! pgrep -x "filebrowser" > /dev/null; then
    echo "🚀 Restarting FileBrowser..." >> "$LOG_FILE"
    # 백그라운드 실행 (로그는 별도 관리)
    nohup "$FB_BIN" -r "$TARGET_VOLUME" -p 8080 -a 0.0.0.0 > "$VAULT_PATH/filebrowser.log" 2>&1 &
fi

# 4. Jellyfin은 macOS App 형태이므로 실행 여부만 체크
if ! pgrep -x "jellyfin" > /dev/null; then
    echo "🎬 Jellyfin is not running. Please start Jellyfin.app manually if needed." >> "$LOG_FILE"
    # open -a Jellyfin # 필요시 자동 실행
fi

echo "[$(date)] Guard check finished." >> "$LOG_FILE"
