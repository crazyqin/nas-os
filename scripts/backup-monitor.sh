#!/bin/bash
# =============================================================================
# NAS-OS 备份任务监控脚本 v2.59.0
# =============================================================================
# 用途：监控备份任务状态、检查备份完整性、发送告警通知
# 用法：
#   ./backup-monitor.sh                    # 执行一次性检查
#   ./backup-monitor.sh --daemon          # 守护进程模式
#   ./backup-monitor.sh --status          # 查看备份状态
#   ./backup-monitor.sh --verify <file>   # 验证备份文件
#   ./backup-monitor.sh --report          # 生成备份报告
# =============================================================================

set -euo pipefail

# =============================================================================
# 版本信息
# =============================================================================
VERSION="2.59.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# =============================================================================
# 默认配置
# =============================================================================

# 备份目录
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
BACKUP_LOG_DIR="${BACKUP_LOG_DIR:-/var/log/nas-os}"
BACKUP_DATA_DIR="${BACKUP_DATA_DIR:-/var/lib/nas-os}"

# 监控配置
MONITOR_INTERVAL="${MONITOR_INTERVAL:-300}"         # 监控间隔(秒)
BACKUP_TIMEOUT="${BACKUP_TIMEOUT:-3600}"            # 备份超时时间(秒)
STALE_BACKUP_HOURS="${STALE_BACKUP_HOURS:-24}"      # 过期备份阈值(小时)
MIN_FREE_SPACE_GB="${MIN_FREE_SPACE_GB:-10}"        # 最小可用空间(GB)

# 保留策略检查
RETENTION_DAYS="${RETENTION_DAYS:-30}"
MIN_BACKUP_COUNT="${MIN_BACKUP_COUNT:-3}"           # 最少备份数量

# 校验配置
VERIFY_CHECKSUM="${VERIFY_CHECKSUM:-true}"
VERIFY_INTEGRITY="${VERIFY_INTEGRITY:-true}"

# 告警配置
ALERT_ENABLED="${ALERT_ENABLED:-true}"
ALERT_COOLDOWN="${ALERT_COOLDOWN:-1800}"            # 告警冷却时间(秒)
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_EMAIL="${ALERT_EMAIL:-}"
ALERT_DINGTALK="${ALERT_DINGTALK:-}"
ALERT_FEISHU="${ALERT_FEISHU:-}"

# Prometheus 配置
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9102}"
METRICS_ENABLED="${METRICS_ENABLED:-true}"
METRICS_FILE="${METRICS_FILE:-/var/lib/nas-os/metrics/backup.prom}"

# 数据文件
DATA_DIR="${DATA_DIR:-/var/lib/nas-os/backup-monitor}"
STATE_FILE="$DATA_DIR/backup-monitor.state"
HISTORY_FILE="$DATA_DIR/backup-history.json"

# PID 文件
PID_FILE="/var/run/nas-os-backup-monitor.pid"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# =============================================================================
# 全局变量
# =============================================================================
BACKUP_METRICS={}
ALERTS=()
LAST_ALERT_TIME={}
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNING=0

# =============================================================================
# 工具函数
# =============================================================================

log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case "$level" in
        ERROR)   echo -e "${RED}[$timestamp] [ERROR]${NC} $msg" >&2 ;;
        WARN)    echo -e "${YELLOW}[$timestamp] [WARN]${NC} $msg" ;;
        INFO)    echo -e "${BLUE}[$timestamp] [INFO]${NC} $msg" ;;
        SUCCESS) echo -e "${GREEN}[$timestamp] [OK]${NC} $msg" ;;
        DEBUG)   [ "${DEBUG:-false}" = "true" ] && echo -e "${CYAN}[$timestamp] [DEBUG]${NC} $msg" ;;
    esac
}

init_data_dir() {
    mkdir -p "$DATA_DIR" 2>/dev/null || {
        DATA_DIR="/tmp/nas-os-backup-monitor"
        mkdir -p "$DATA_DIR"
        log WARN "使用临时数据目录: $DATA_DIR"
    }
    
    # 初始化状态文件
    if [ ! -f "$STATE_FILE" ]; then
        echo '{}' > "$STATE_FILE"
    fi
}

get_timestamp() {
    date '+%Y-%m-%dT%H:%M:%S%z'
}

get_epoch_time() {
    date '+%s'
}

# =============================================================================
# 备份检查函数
# =============================================================================

# 获取备份列表
get_backup_list() {
    local backup_type="${1:-all}"
    
    if [ "$backup_type" = "all" ]; then
        find "$BACKUP_DIR" -type f \( -name "*.tar.gz" -o -name "*.tar.xz" -o -name "*.tar" -o -name "*.zip" \) 2>/dev/null | sort -r
    else
        find "$BACKUP_DIR" -type f -name "*-${backup_type}-*" \( -name "*.tar.gz" -o -name "*.tar.xz" -o -name "*.tar" -o -name "*.zip" \) 2>/dev/null | sort -r
    fi
}

# 获取最新备份
get_latest_backup() {
    local backup_type="${1:-all}"
    get_backup_list "$backup_type" | head -1
}

# 获取备份信息
get_backup_info() {
    local backup_file="$1"
    
    if [ ! -f "$backup_file" ]; then
        echo "error=file_not_found"
        return 1
    fi
    
    local size=$(stat -c%s "$backup_file" 2>/dev/null || stat -f%z "$backup_file" 2>/dev/null)
    local mtime=$(stat -c%Y "$backup_file" 2>/dev/null || stat -f%m "$backup_file" 2>/dev/null)
    local mtime_human=$(date -d "@$mtime" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || date -r "$mtime" '+%Y-%m-%d %H:%M:%S' 2>/dev/null)
    local checksum_file="${backup_file}.sha256"
    local checksum_status="missing"
    
    if [ -f "$checksum_file" ]; then
        checksum_status="present"
    fi
    
    # 计算年龄（小时）
    local now=$(date +%s)
    local age_hours=$(( (now - mtime) / 3600 ))
    
    # 提取备份类型
    local basename=$(basename "$backup_file")
    local backup_type="unknown"
    if [[ "$basename" == *"full"* ]]; then
        backup_type="full"
    elif [[ "$basename" == *"incremental"* ]]; then
        backup_type="incremental"
    elif [[ "$basename" == *"database"* ]]; then
        backup_type="database"
    elif [[ "$basename" == *"config"* ]]; then
        backup_type="config"
    fi
    
    echo "file=$backup_file"
    echo "size=$size"
    echo "size_human=$(numfmt --to=iec-i --suffix=B $size 2>/dev/null || echo $size)"
    echo "mtime=$mtime"
    echo "mtime_human=$mtime_human"
    echo "age_hours=$age_hours"
    echo "type=$backup_type"
    echo "checksum=$checksum_status"
}

# 检查备份空间
check_backup_space() {
    local backup_path="${1:-$BACKUP_DIR}"
    
    local info=$(df -BG "$backup_path" 2>/dev/null | awk 'NR==2 {print $2,$3,$4}')
    if [ -z "$info" ]; then
        log ERROR "无法获取磁盘空间信息: $backup_path"
        return 1
    fi
    
    local total=$(echo "$info" | awk '{print $1}' | tr -d 'G')
    local used=$(echo "$info" | awk '{print $2}' | tr -d 'G')
    local avail=$(echo "$info" | awk '{print $3}' | tr -d 'G')
    
    local usage_pct=$((used * 100 / total))
    
    log INFO "备份空间: 总计 ${total}GB, 已用 ${used}GB (${usage_pct}%), 可用 ${avail}GB"
    
    if [ "$avail" -lt "$MIN_FREE_SPACE_GB" ]; then
        log WARN "备份空间不足: 可用 ${avail}GB < 最小要求 ${MIN_FREE_SPACE_GB}GB"
        trigger_alert "backup_space" "warning" "备份空间不足: 可用 ${avail}GB"
        return 2
    fi
    
    return 0
}

# 检查备份时效性
check_backup_freshness() {
    local backup_type="${1:-all}"
    local max_age_hours="${2:-$STALE_BACKUP_HOURS}"
    
    local latest=$(get_latest_backup "$backup_type")
    
    if [ -z "$latest" ]; then
        log ERROR "未找到备份文件: $backup_type"
        trigger_alert "backup_missing" "critical" "未找到备份文件 (类型: $backup_type)"
        return 1
    fi
    
    eval $(get_backup_info "$latest")
    
    if [ "${age_hours:-999}" -gt "$max_age_hours" ]; then
        log WARN "备份过期: $latest (年龄: ${age_hours}小时 > ${max_age_hours}小时)"
        trigger_alert "backup_stale" "warning" "备份过期: ${age_hours}小时"
        return 2
    fi
    
    log SUCCESS "备份时效检查通过: $latest (${age_hours}小时)"
    return 0
}

# 检查备份数量
check_backup_count() {
    local backup_type="${1:-all}"
    local min_count="${2:-$MIN_BACKUP_COUNT}"
    
    local count=$(get_backup_list "$backup_type" | wc -l)
    
    if [ "$count" -lt "$min_count" ]; then
        log WARN "备份数量不足: $count < $min_count (类型: $backup_type)"
        trigger_alert "backup_count" "warning" "备份数量不足: $count (最少: $min_count)"
        return 2
    fi
    
    log SUCCESS "备份数量检查通过: $count 个备份"
    return 0
}

# 验证备份文件
verify_backup() {
    local backup_file="$1"
    local verify_checksum="${VERIFY_CHECKSUM:-true}"
    local verify_integrity="${VERIFY_INTEGRITY:-true}"
    
    if [ ! -f "$backup_file" ]; then
        log ERROR "备份文件不存在: $backup_file"
        return 1
    fi
    
    log INFO "验证备份文件: $backup_file"
    
    # 校验和验证
    if [ "$verify_checksum" = "true" ]; then
        local checksum_file="${backup_file}.sha256"
        if [ -f "$checksum_file" ]; then
            local stored_checksum=$(awk '{print $1}' "$checksum_file")
            local actual_checksum=$(sha256sum "$backup_file" 2>/dev/null | awk '{print $1}')
            
            if [ "$stored_checksum" = "$actual_checksum" ]; then
                log SUCCESS "校验和验证通过"
            else
                log ERROR "校验和不匹配: 文件可能已损坏"
                trigger_alert "backup_checksum" "critical" "备份文件校验失败: $(basename $backup_file)"
                return 1
            fi
        else
            log WARN "未找到校验和文件: $checksum_file"
        fi
    fi
    
    # 完整性验证
    if [ "$verify_integrity" = "true" ]; then
        case "$backup_file" in
            *.tar.gz|*.tgz)
                if gzip -t "$backup_file" 2>/dev/null; then
                    log SUCCESS "gzip 完整性验证通过"
                else
                    log ERROR "gzip 文件损坏"
                    return 1
                fi
                ;;
            *.tar.xz|*.txz)
                if xz -t "$backup_file" 2>/dev/null; then
                    log SUCCESS "xz 完整性验证通过"
                else
                    log ERROR "xz 文件损坏"
                    return 1
                fi
                ;;
            *.zip)
                if unzip -t "$backup_file" >/dev/null 2>&1; then
                    log SUCCESS "zip 完整性验证通过"
                else
                    log ERROR "zip 文件损坏"
                    return 1
                fi
                ;;
        esac
    fi
    
    return 0
}

# 检查备份进程
check_backup_process() {
    local running=false
    local pid=""
    local runtime=0
    
    # 检查备份进程
    if pgrep -f "backup.sh" >/dev/null 2>&1; then
        running=true
        pid=$(pgrep -f "backup.sh" | head -1)
        
        # 计算运行时间
        local start_time=$(ps -o lstart= -p "$pid" 2>/dev/null)
        if [ -n "$start_time" ]; then
            local start_epoch=$(date -d "$start_time" +%s 2>/dev/null || date -j -f "%a %b %d %H:%M:%S %Y" "$start_time" +%s 2>/dev/null)
            local now=$(date +%s)
            runtime=$((now - start_epoch))
        fi
    fi
    
    if [ "$running" = "true" ]; then
        if [ "$runtime" -gt "$BACKUP_TIMEOUT" ]; then
            log ERROR "备份进程超时: PID $pid, 运行时间 ${runtime}秒"
            trigger_alert "backup_timeout" "critical" "备份进程超时 (运行: ${runtime}秒)"
            return 1
        else
            log INFO "备份进程运行中: PID $pid, 运行时间 ${runtime}秒"
        fi
    else
        log INFO "无备份进程运行"
    fi
    
    return 0
}

# 检查备份日志错误
check_backup_log_errors() {
    local log_file="${BACKUP_LOG_DIR}/backup.log"
    local since_hours="${1:-24}"
    
    if [ ! -f "$log_file" ]; then
        log WARN "备份日志文件不存在: $log_file"
        return 0
    fi
    
    # 查找最近的错误
    local errors=$(grep -i "error\|fail\|critical" "$log_file" 2>/dev/null | tail -10)
    
    if [ -n "$errors" ]; then
        local error_count=$(echo "$errors" | wc -l)
        log WARN "发现 $error_count 条备份错误"
        trigger_alert "backup_errors" "warning" "备份日志中发现 $error_count 条错误"
        return 2
    fi
    
    return 0
}

# =============================================================================
# 告警函数
# =============================================================================

trigger_alert() {
    local alert_type="$1"
    local severity="$2"
    local message="$3"
    
    if [ "$ALERT_ENABLED" != "true" ]; then
        return
    fi
    
    # 检查冷却时间
    local now=$(get_epoch_time)
    local last_time=$(cat "$STATE_FILE" 2>/dev/null | grep -o "\"${alert_type}\":[0-9]*" | grep -o '[0-9]*' || echo 0)
    
    if [ -n "$last_time" ] && [ $((now - last_time)) -lt "$ALERT_COOLDOWN" ]; then
        log DEBUG "告警冷却中: $alert_type"
        return
    fi
    
    # 记录告警时间
    local tmp_file=$(mktemp)
    cat "$STATE_FILE" 2>/dev/null | sed "s/\"${alert_type}\":[0-9]*/\"${alert_type}\":$now/" > "$tmp_file" 2>/dev/null || echo "{\"$alert_type\":$now}" > "$tmp_file"
    mv "$tmp_file" "$STATE_FILE"
    
    # 发送告警
    log WARN "告警 [$severity]: $message"
    ALERTS+=("[$severity] $message")
    
    # Webhook 告警
    if [ -n "$ALERT_WEBHOOK" ]; then
        local payload=$(cat <<EOF
{
    "alert_type": "$alert_type",
    "severity": "$severity",
    "message": "$message",
    "timestamp": "$(get_timestamp)",
    "source": "nas-os-backup-monitor"
}
EOF
)
        curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_WEBHOOK" 2>/dev/null || true
    fi
    
    # 钉钉告警
    if [ -n "$ALERT_DINGTALK" ]; then
        local payload=$(cat <<EOF
{
    "msgtype": "text",
    "text": {
        "content": "[NAS-OS 备份告警]\n级别: $severity\n消息: $message\n时间: $(get_timestamp)"
    }
}
EOF
)
        curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_DINGTALK" 2>/dev/null || true
    fi
    
    # 飞书告警
    if [ -n "$ALERT_FEISHU" ]; then
        local payload=$(cat <<EOF
{
    "msg_type": "text",
    "content": {
        "text": "[NAS-OS 备份告警]\n级别: $severity\n消息: $message\n时间: $(get_timestamp)"
    }
}
EOF
)
        curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_FEISHU" 2>/dev/null || true
    fi
}

# =============================================================================
# Prometheus 指标
# =============================================================================

export_prometheus_metrics() {
    local metrics_dir=$(dirname "$METRICS_FILE")
    mkdir -p "$metrics_dir" 2>/dev/null || return
    
    local tmp_file=$(mktemp)
    
    # 备份统计指标
    local total_count=$(get_backup_list | wc -l)
    local full_count=$(get_backup_list "full" | wc -l)
    local incremental_count=$(get_backup_list "incremental" | wc -l)
    local database_count=$(get_backup_list "database" | wc -l)
    local config_count=$(get_backup_list "config" | wc -l)
    
    # 空间使用
    local backup_size=$(du -sb "$BACKUP_DIR" 2>/dev/null | awk '{print $1}')
    local space_info=$(df -B1 "$BACKUP_DIR" 2>/dev/null | awk 'NR==2 {print $2,$3,$4}')
    local space_total=$(echo "$space_info" | awk '{print $1}')
    local space_used=$(echo "$space_info" | awk '{print $2}')
    local space_avail=$(echo "$space_info" | awk '{print $3}')
    
    # 最新备份信息
    local latest=$(get_latest_backup)
    local latest_age=0
    local latest_size=0
    if [ -n "$latest" ]; then
        eval $(get_backup_info "$latest")
        latest_age="${age_hours:-0}"
        latest_size="${size:-0}"
    fi
    
    cat > "$tmp_file" << EOF
# HELP nas_backup_total_count Total number of backup files
# TYPE nas_backup_total_count gauge
nas_backup_total_count $total_count

# HELP nas_backup_full_count Number of full backup files
# TYPE nas_backup_full_count gauge
nas_backup_full_count $full_count

# HELP nas_backup_incremental_count Number of incremental backup files
# TYPE nas_backup_incremental_count gauge
nas_backup_incremental_count $incremental_count

# HELP nas_backup_database_count Number of database backup files
# TYPE nas_backup_database_count gauge
nas_backup_database_count $database_count

# HELP nas_backup_config_count Number of config backup files
# TYPE nas_backup_config_count gauge
nas_backup_config_count $config_count

# HELP nas_backup_total_size_bytes Total size of all backup files in bytes
# TYPE nas_backup_total_size_bytes gauge
nas_backup_total_size_bytes $backup_size

# HELP nas_backup_space_total_bytes Total backup space in bytes
# TYPE nas_backup_space_total_bytes gauge
nas_backup_space_total_bytes $space_total

# HELP nas_backup_space_used_bytes Used backup space in bytes
# TYPE nas_backup_space_used_bytes gauge
nas_backup_space_used_bytes $space_used

# HELP nas_backup_space_available_bytes Available backup space in bytes
# TYPE nas_backup_space_available_bytes gauge
nas_backup_space_available_bytes $space_avail

# HELP nas_backup_latest_age_hours Age of latest backup in hours
# TYPE nas_backup_latest_age_hours gauge
nas_backup_latest_age_hours $latest_age

# HELP nas_backup_latest_size_bytes Size of latest backup in bytes
# TYPE nas_backup_latest_size_bytes gauge
nas_backup_latest_size_bytes $latest_size

# HELP nas_backup_monitor_checks_total Total number of checks performed
# TYPE nas_backup_monitor_checks_total counter
nas_backup_monitor_checks_total $((CHECKS_PASSED + CHECKS_FAILED + CHECKS_WARNING))

# HELP nas_backup_monitor_checks_passed Number of passed checks
# TYPE nas_backup_monitor_checks_passed counter
nas_backup_monitor_checks_passed $CHECKS_PASSED

# HELP nas_backup_monitor_checks_failed Number of failed checks
# TYPE nas_backup_monitor_checks_failed counter
nas_backup_monitor_checks_failed $CHECKS_FAILED

# HELP nas_backup_monitor_checks_warning Number of warning checks
# TYPE nas_backup_monitor_checks_warning counter
nas_backup_monitor_checks_warning $CHECKS_WARNING
EOF
    
    mv "$tmp_file" "$METRICS_FILE"
    log DEBUG "Prometheus 指标已导出: $METRICS_FILE"
}

# =============================================================================
# 报告函数
# =============================================================================

generate_backup_report() {
    local format="${1:-text}"
    
    local report=""
    local timestamp=$(get_timestamp)
    
    # 备份统计
    local total_count=$(get_backup_list | wc -l)
    local backup_size=$(du -sh "$BACKUP_DIR" 2>/dev/null | awk '{print $1}')
    
    # 空间使用
    local space_info=$(df -h "$BACKUP_DIR" 2>/dev/null | awk 'NR==2 {print $2,$3,$4,$5}')
    local space_total=$(echo "$space_info" | awk '{print $1}')
    local space_used=$(echo "$space_info" | awk '{print $2}')
    local space_avail=$(echo "$space_info" | awk '{print $3}')
    local space_pct=$(echo "$space_info" | awk '{print $4}')
    
    # 最新备份
    local latest=$(get_latest_backup)
    local latest_info=""
    if [ -n "$latest" ]; then
        latest_info=$(get_backup_info "$latest")
    fi
    
    if [ "$format" = "json" ]; then
        cat << EOF
{
    "timestamp": "$timestamp",
    "backup_directory": "$BACKUP_DIR",
    "statistics": {
        "total_backups": $total_count,
        "total_size": "$backup_size"
    },
    "storage": {
        "total": "$space_total",
        "used": "$space_used",
        "available": "$space_avail",
        "usage_percent": "$space_pct"
    },
    "latest_backup": {
        $(echo "$latest_info" | tr '\n' ',' | sed 's/,$//')
    },
    "monitoring": {
        "checks_passed": $CHECKS_PASSED,
        "checks_failed": $CHECKS_FAILED,
        "checks_warning": $CHECKS_WARNING
    }
}
EOF
    else
        cat << EOF
================================================================================
                        NAS-OS 备份监控报告
================================================================================
生成时间: $timestamp
备份目录: $BACKUP_DIR

【备份统计】
  总备份数: $total_count
  总大小:   $backup_size

【存储使用】
  总空间:   $space_total
  已使用:   $space_used
  可用:     $space_avail
  使用率:   $space_pct

【最新备份】
$(if [ -n "$latest" ]; then
    echo "  文件:     $latest"
    eval $(get_backup_info "$latest")
    echo "  大小:     ${size_human:-unknown}"
    echo "  时间:     ${mtime_human:-unknown}"
    echo "  年龄:     ${age_hours:-unknown} 小时"
else
    echo "  无备份文件"
fi)

【监控状态】
  通过检查: $CHECKS_PASSED
  失败检查: $CHECKS_FAILED
  警告检查: $CHECKS_WARNING

================================================================================
EOF
    fi
}

# =============================================================================
# 主检查函数
# =============================================================================

run_checks() {
    log INFO "开始备份监控检查..."
    
    # 检查备份空间
    check_backup_space
    case $? in
        0) ((CHECKS_PASSED++)) ;;
        2) ((CHECKS_WARNING++)) ;;
        *) ((CHECKS_FAILED++)) ;;
    esac
    
    # 检查备份时效性
    check_backup_freshness
    case $? in
        0) ((CHECKS_PASSED++)) ;;
        2) ((CHECKS_WARNING++)) ;;
        *) ((CHECKS_FAILED++)) ;;
    esac
    
    # 检查备份数量
    check_backup_count
    case $? in
        0) ((CHECKS_PASSED++)) ;;
        2) ((CHECKS_WARNING++)) ;;
        *) ((CHECKS_FAILED++)) ;;
    esac
    
    # 检查备份进程
    check_backup_process
    case $? in
        0) ((CHECKS_PASSED++)) ;;
        2) ((CHECKS_WARNING++)) ;;
        *) ((CHECKS_FAILED++)) ;;
    esac
    
    # 检查备份日志
    check_backup_log_errors
    case $? in
        0) ((CHECKS_PASSED++)) ;;
        2) ((CHECKS_WARNING++)) ;;
        *) ((CHECKS_FAILED++)) ;;
    esac
    
    # 导出 Prometheus 指标
    if [ "$METRICS_ENABLED" = "true" ]; then
        export_prometheus_metrics
    fi
    
    log INFO "备份监控检查完成: 通过 $CHECKS_PASSED, 警告 $CHECKS_WARNING, 失败 $CHECKS_FAILED"
}

# =============================================================================
# 守护进程函数
# =============================================================================

daemon_mode() {
    log INFO "启动备份监控守护进程 (间隔: ${MONITOR_INTERVAL}秒)"
    
    # 写入 PID 文件
    echo $$ > "$PID_FILE"
    
    # 设置信号处理
    trap 'log INFO "收到停止信号"; rm -f "$PID_FILE"; exit 0' SIGTERM SIGINT
    
    while true; do
        run_checks
        sleep "$MONITOR_INTERVAL"
    done
}

# =============================================================================
# 状态查看
# =============================================================================

show_status() {
    echo ""
    echo -e "${CYAN}==================== NAS-OS 备份监控状态 ====================${NC}"
    echo ""
    
    # 运行状态
    if [ -f "$PID_FILE" ] && kill -0 $(cat "$PID_FILE") 2>/dev/null; then
        echo -e "状态:     ${GREEN}运行中${NC} (PID: $(cat $PID_FILE))"
    else
        echo -e "状态:     ${YELLOW}未运行${NC}"
    fi
    
    # 备份目录
    echo "备份目录: $BACKUP_DIR"
    
    # 备份统计
    local total=$(get_backup_list | wc -l)
    echo "备份数量: $total"
    
    # 最新备份
    local latest=$(get_latest_backup)
    if [ -n "$latest" ]; then
        eval $(get_backup_info "$latest")
        echo "最新备份: $(basename $latest) (${age_hours:-?}小时前)"
    fi
    
    # 空间使用
    local space_info=$(df -h "$BACKUP_DIR" 2>/dev/null | awk 'NR==2 {print $3,$4,$5}')
    echo "空间使用: $space_info"
    
    # 最近告警
    if [ ${#ALERTS[@]} -gt 0 ]; then
        echo ""
        echo -e "${YELLOW}最近告警:${NC}"
        for alert in "${ALERTS[@]:0:5}"; do
            echo "  - $alert"
        done
    fi
    
    echo ""
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    local mode="check"
    local verify_file=""
    
    # 解析参数
    while [ $# -gt 0 ]; do
        case "$1" in
            --daemon|-d)
                mode="daemon"
                shift
                ;;
            --status|-s)
                mode="status"
                shift
                ;;
            --verify|-v)
                mode="verify"
                verify_file="${2:-}"
                if [ -z "$verify_file" ]; then
                    verify_file=$(get_latest_backup)
                fi
                shift 2
                ;;
            --report|-r)
                mode="report"
                shift
                ;;
            --json)
                OUTPUT_FORMAT="json"
                shift
                ;;
            --help|-h)
                cat << EOF
用法: $SCRIPT_NAME [选项]

选项:
    --daemon, -d          守护进程模式
    --status, -s          查看监控状态
    --verify, -v [FILE]   验证备份文件
    --report, -r          生成备份报告
    --json                JSON 格式输出
    --help, -h            显示帮助

环境变量:
    BACKUP_DIR            备份目录 (默认: /var/lib/nas-os/backups)
    MONITOR_INTERVAL      监控间隔秒数 (默认: 300)
    STALE_BACKUP_HOURS    过期备份阈值小时 (默认: 24)
    ALERT_WEBHOOK         告警 Webhook URL
    ALERT_DINGTALK        钉钉告警 URL
    ALERT_FEISHU          飞书告警 URL

EOF
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done
    
    # 初始化
    init_data_dir
    
    # 执行模式
    case "$mode" in
        daemon)
            daemon_mode
            ;;
        status)
            show_status
            ;;
        verify)
            if [ -z "$verify_file" ]; then
                log ERROR "请指定要验证的备份文件"
                exit 1
            fi
            verify_backup "$verify_file"
            ;;
        report)
            generate_backup_report "${OUTPUT_FORMAT:-text}"
            ;;
        check)
            run_checks
            ;;
    esac
}

main "$@"