#!/bin/zsh

# MistVault Storage & Service Guard
# /Volumes/ROGALLY/github/MistVault/guard.sh

TARGET_VOLUMES=("/Volumes/ROGALLY" "/Volumes/DATA_EXT") # 여기에 추가 볼륨 경로 나열
VAULT_PATH="/Volumes/ROGALLY/github/MistVault"
FB_BIN="/opt/homebrew/bin/filebrowser"
LOG_FILE="$VAULT_PATH/vault_guard.log"

echo "[$(date)] Guard check started..." >> "$LOG_FILE"

# 1. 멀티 볼륨 마운트 및 식별 체크
VALID_VOLUMES=()
for vol in "${TARGET_VOLUMES[@]}"; do
    if [ -d "$vol" ]; then
        echo "✅ Detected: $vol" >> "$LOG_FILE"
        VALID_VOLUMES+=("$vol")
    else
        echo "❌ Missing: $vol" >> "$LOG_FILE"
    fi
done

if [ ${#VALID_VOLUMES[@]} -eq 0 ]; then
    echo "🚨 [CRITICAL] 접근 가능한 볼륨이 하나도 없습니다!" >> "$LOG_FILE"
    exit 1
fi

# 2. FileBrowser 실행 (모든 유효 볼륨의 상위인 /Volumes를 루트로 잡거나 개별 설정)
# 여기서는 /Volumes를 루트로 하되, 특정 볼륨들만 보이게 하려면 설정을 더 꼬아야 하지만
# 가장 심플하게 /Volumes 전체를 보여주되 위에서 체크한 볼륨들이 있는지 확인하는 식.
if ! pgrep -x "filebrowser" > /dev/null; then
    echo "🚀 Restarting FileBrowser (Root: /Volumes)..." >> "$LOG_FILE"
    # /Volumes를 루트로 잡으면 연결된 모든 하드에 접근 가능해짐
    nohup "$FB_BIN" -r "/Volumes" -p 8080 -a 0.0.0.0 > "$VAULT_PATH/filebrowser.log" 2>&1 &
fi

# 4. Jellyfin은 macOS App 형태이므로 실행 여부만 체크
if ! pgrep -x "jellyfin" > /dev/null; then
    echo "🎬 Jellyfin is not running. Please start Jellyfin.app manually if needed." >> "$LOG_FILE"
    # open -a Jellyfin # 필요시 자동 실행
fi

echo "[$(date)] Guard check finished." >> "$LOG_FILE"
