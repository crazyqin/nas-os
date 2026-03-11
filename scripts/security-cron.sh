#!/bin/bash
#
# 定时安全检查脚本 - 用于 cron 调度
# 功能：定期运行安全检查并发送通知
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="$PROJECT_DIR/logs/security-check.log"

# 确保日志目录存在
mkdir -p "$PROJECT_DIR/logs"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

cd "$PROJECT_DIR"

log "开始定时安全检查..."

# 运行安全检查
if [ -x "$SCRIPT_DIR/security-check.sh" ]; then
    OUTPUT=$("$SCRIPT_DIR/security-check.sh" --notify 2>&1) || true
    log "$OUTPUT"
    
    # 检查是否有高危问题
    if echo "$OUTPUT" | grep -q "需要立即关注"; then
        log "⚠️ 发现需要立即关注的安全问题！"
        # 这里可以添加紧急通知逻辑（如发送邮件、Discord 消息等）
    else
        log "✓ 安全检查通过"
    fi
else
    log "错误：安全检查脚本不存在或不可执行"
    exit 1
fi

log "定时安全检查完成"
