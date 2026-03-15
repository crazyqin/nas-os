#!/bin/bash
# =============================================================================
# NAS-OS 磁盘健康检查脚本 v2.59.0
# =============================================================================
# 用途：检查磁盘 SMART 状态、温度、性能，输出结构化结果
# 用法：
#   ./disk-health-check.sh                 # 检查所有磁盘
#   ./disk-health-check.sh /dev/sda        # 检查指定磁盘
#   ./disk-health-check.sh --json          # JSON 格式输出
#   ./disk-health-check.sh --smart         # 详细 SMART 信息
#   ./disk-health-check.sh --test short    # 运行 SMART 短测试
#   ./disk-health-check.sh --monitor       # 持续监控模式
# =============================================================================

set -euo pipefail

# =============================================================================
# 版本信息
# =============================================================================
VERSION="2.59.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置
# =============================================================================

# 温度阈值
TEMP_WARNING="${TEMP_WARNING:-50}"         # 温度警告阈值 (°C)
TEMP_CRITICAL="${TEMP_CRITICAL:-60}"       # 温度严重阈值 (°C)

# SMART 属性阈值
REALLOCATED_WARNING="${REALLOCATED_WARNING:-10}"
REALLOCATED_CRITICAL="${REALLOCATED_CRITICAL:-100}"
PENDING_WARNING="${PENDING_WARNING:-10}"
PENDING_CRITICAL="${PENDING_CRITICAL:-100}"
CRC_ERROR_WARNING="${CRC_ERROR_WARNING:-100}"

# 健康评分阈值
HEALTH_WARNING="${HEALTH_WARNING:-70}"
HEALTH_CRITICAL="${HEALTH_CRITICAL:-40}"

# 监控配置
MONITOR_INTERVAL="${MONITOR_INTERVAL:-300}"    # 监控间隔(秒)
HISTORY_FILE="${HISTORY_FILE:-/var/lib/nas-os/disk-health-history.json}"
ALERT_COOLDOWN="${ALERT_COOLDOWN:-3600}"        # 告警冷却时间(秒)

# 告警配置
ALERT_ENABLED="${ALERT_ENABLED:-true}"
ALERT_WEBHOOK="${ALERT_WEBHOOK:-}"
ALERT_DINGTALK="${ALERT_DINGTALK:-}"
ALERT_FEISHU="${ALERT_FEISHU:-}"

# Prometheus 指标
METRICS_FILE="${METRICS_FILE:-/var/lib/nas-os/metrics/disk-health.prom}"

# 输出格式
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"

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
DISKS=()
RESULTS=()
HEALTH_SCORES=()
ALERTS=()

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

get_timestamp() {
    date -Iseconds 2>/dev/null || date '+%Y-%m-%dT%H:%M:%S%z'
}

check_smartctl() {
    if ! command -v smartctl &>/dev/null; then
        log ERROR "smartctl 未安装，请安装 smartmontools 包"
        log INFO "Debian/Ubuntu: sudo apt install smartmontools"
        log INFO "CentOS/RHEL:   sudo yum install smartmontools"
        return 1
    fi
    return 0
}

# =============================================================================
# 磁盘发现
# =============================================================================

discover_disks() {
    DISKS=()
    
    # 使用 lsblk 列出磁盘
    if command -v lsblk &>/dev/null; then
        while IFS= read -r line; do
            local name=$(echo "$line" | awk '{print $1}')
            local type=$(echo "$line" | awk '{print $2}')
            
            if [ "$type" = "disk" ]; then
                DISKS+=("/dev/$name")
            fi
        done < <(lsblk -d -n -o NAME,TYPE 2>/dev/null)
    fi
    
    # 备用方案：扫描 /dev 目录
    if [ ${#DISKS[@]} -eq 0 ]; then
        for dev in /dev/sd? /dev/nvme* /dev/vd? /dev/hd?; do
            if [ -b "$dev" ]; then
                DISKS+=("$dev")
            fi
        done 2>/dev/null
    fi
    
    log INFO "发现 ${#DISKS[@]} 个磁盘: ${DISKS[*]}"
}

# =============================================================================
# SMART 检查函数
# =============================================================================

# 检查 SMART 是否支持
check_smart_support() {
    local device="$1"
    
    if ! smartctl -i "$device" &>/dev/null; then
        return 1
    fi
    
    # 检查 SMART 是否可用
    if smartctl -i "$device" 2>/dev/null | grep -qi "SMART support is: Available"; then
        return 0
    fi
    
    return 1
}

# 启用 SMART
enable_smart() {
    local device="$1"
    
    if smartctl -s on "$device" &>/dev/null; then
        log INFO "已启用 SMART: $device"
        return 0
    fi
    
    return 1
}

# 获取磁盘基本信息
get_disk_info() {
    local device="$1"
    local info_file=$(mktemp)
    
    smartctl -i "$device" > "$info_file" 2>/dev/null || {
        rm -f "$info_file"
        return 1
    }
    
    # 解析信息
    local model=$(grep -E "^Device Model:|^Model Number:" "$info_file" | awk -F: '{print $2}' | xargs)
    local serial=$(grep "Serial Number:" "$info_file" | awk -F: '{print $2}' | xargs)
    local firmware=$(grep "Firmware Version:" "$info_file" | awk -F: '{print $2}' | xargs)
    local capacity=$(grep "User Capacity:" "$info_file" | awk -F: '{gsub(/[^0-9]/,"",$2); print $2}')
    local rotation=$(grep "Rotation Rate:" "$info_file" | awk -F: '{print $2}' | xargs)
    local is_ssd="false"
    
    if [[ "$rotation" == *"Solid State"* ]] || [[ "$model" == *"SSD"* ]]; then
        is_ssd="true"
        rotation="SSD"
    fi
    
    rm -f "$info_file"
    
    echo "device=$device"
    echo "model=$model"
    echo "serial=$serial"
    echo "firmware=$firmware"
    echo "capacity=$capacity"
    echo "rotation=$rotation"
    echo "is_ssd=$is_ssd"
}

# 获取 SMART 健康状态
get_smart_health() {
    local device="$1"
    
    local health_output=$(smartctl -H "$device" 2>/dev/null)
    
    if echo "$health_output" | grep -qi "PASSED"; then
        echo "healthy"
    elif echo "$health_output" | grep -qi "FAILED"; then
        echo "failed"
    else
        echo "unknown"
    fi
}

# 获取 SMART 属性
get_smart_attributes() {
    local device="$1"
    local attrs_file=$(mktemp)
    
    smartctl -A "$device" > "$attrs_file" 2>/dev/null || {
        rm -f "$attrs_file"
        return 1
    }
    
    # 关键属性
    local temperature="N/A"
    local reallocated="0"
    local pending="0"
    local crc_errors="0"
    local power_hours="0"
    local power_cycles="0"
    
    # 温度
    temperature=$(grep -E "Temperature|Airflow_Temperature" "$attrs_file" | head -1 | awk '{print $10}')
    
    # 重分配扇区
    reallocated=$(grep "Reallocated_Sector_Ct" "$attrs_file" | awk '{print $10}')
    
    # 待定扇区
    pending=$(grep "Current_Pending_Sector" "$attrs_file" | awk '{print $10}')
    
    # CRC 错误
    crc_errors=$(grep "CRC_Error_Count" "$attrs_file" | awk '{print $10}')
    
    # 通电时间
    power_hours=$(grep -E "Power_On_Hours" "$attrs_file" | head -1 | awk '{print $10}')
    
    # 通电次数
    power_cycles=$(grep "Power_Cycle_Count" "$attrs_file" | awk '{print $10}')
    
    rm -f "$attrs_file"
    
    # 设置默认值
    temperature=${temperature:-0}
    reallocated=${reallocated:-0}
    pending=${pending:-0}
    crc_errors=${crc_errors:-0}
    power_hours=${power_hours:-0}
    power_cycles=${power_cycles:-0}
    
    echo "temperature=$temperature"
    echo "reallocated=$reallocated"
    echo "pending=$pending"
    echo "crc_errors=$crc_errors"
    echo "power_hours=$power_hours"
    echo "power_cycles=$power_cycles"
}

# 计算健康评分
calculate_health_score() {
    local device="$1"
    local temperature="$2"
    local reallocated="$3"
    local pending="$4"
    local crc_errors="$5"
    local smart_health="$6"
    
    local score=100
    
    # SMART 健康状态直接决定基础分
    if [ "$smart_health" = "failed" ]; then
        score=0
    elif [ "$smart_health" = "unknown" ]; then
        score=50
    fi
    
    # 温度扣分
    if [ "$temperature" -gt "$TEMP_CRITICAL" ] 2>/dev/null; then
        score=$((score - 20))
    elif [ "$temperature" -gt "$TEMP_WARNING" ] 2>/dev/null; then
        score=$((score - 10))
    fi
    
    # 重分配扇区扣分
    if [ "$reallocated" -gt "$REALLOCATED_CRITICAL" ] 2>/dev/null; then
        score=$((score - 30))
    elif [ "$reallocated" -gt "$REALLOCATED_WARNING" ] 2>/dev/null; then
        score=$((score - 15))
    fi
    
    # 待定扇区扣分
    if [ "$pending" -gt "$PENDING_CRITICAL" ] 2>/dev/null; then
        score=$((score - 25))
    elif [ "$pending" -gt "$PENDING_WARNING" ] 2>/dev/null; then
        score=$((score - 10))
    fi
    
    # CRC 错误扣分
    if [ "$crc_errors" -gt "$CRC_ERROR_WARNING" ] 2>/dev/null; then
        score=$((score - 10))
    fi
    
    # 确保分数在 0-100 之间
    if [ "$score" -lt 0 ]; then
        score=0
    fi
    
    echo "$score"
}

# 获取健康状态
get_health_status() {
    local score="$1"
    
    if [ "$score" -ge "$HEALTH_WARNING" ]; then
        echo "healthy"
    elif [ "$score" -ge "$HEALTH_CRITICAL" ]; then
        echo "warning"
    else
        echo "critical"
    fi
}

# =============================================================================
# 检查函数
# =============================================================================

check_disk() {
    local device="$1"
    local result=""
    
    log INFO "检查磁盘: $device"
    
    # 检查 SMART 支持
    if ! check_smart_support "$device"; then
        log WARN "磁盘不支持 SMART: $device"
        
        if [ "$OUTPUT_FORMAT" = "json" ]; then
            cat << EOF
{
    "device": "$device",
    "smart_supported": false,
    "health": "unknown",
    "score": 0
}
EOF
        else
            echo "  设备:    $device"
            echo "  SMART:   不支持"
            echo "  状态:    未知"
        fi
        return 2
    fi
    
    # 启用 SMART
    enable_smart "$device" 2>/dev/null || true
    
    # 获取基本信息
    eval $(get_disk_info "$device")
    
    # 获取 SMART 状态
    local smart_health=$(get_smart_health "$device")
    
    # 获取 SMART 属性
    eval $(get_smart_attributes "$device")
    
    # 计算健康评分
    local score=$(calculate_health_score "$device" "$temperature" "$reallocated" "$pending" "$crc_errors" "$smart_health")
    
    # 获取健康状态
    local status=$(get_health_status "$score")
    
    # 存储结果
    RESULTS+=("$device:$status:$score:$temperature")
    HEALTH_SCORES+=("$score")
    
    # 触发告警
    if [ "$status" = "critical" ]; then
        trigger_alert "disk_health" "critical" "磁盘 $device 健康状态严重: 评分 $score"
    elif [ "$status" = "warning" ]; then
        trigger_alert "disk_health" "warning" "磁盘 $device 需要关注: 评分 $score"
    fi
    
    if [ "$temperature" -gt "$TEMP_CRITICAL" ]; then
        trigger_alert "disk_temperature" "critical" "磁盘 $device 温度过高: ${temperature}°C"
    elif [ "$temperature" -gt "$TEMP_WARNING" ]; then
        trigger_alert "disk_temperature" "warning" "磁盘 $device 温度偏高: ${temperature}°C"
    fi
    
    # 输出结果
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        cat << EOF
{
    "device": "$device",
    "model": "$model",
    "serial": "$serial",
    "firmware": "$firmware",
    "capacity": $capacity,
    "is_ssd": $is_ssd,
    "smart_supported": true,
    "smart_health": "$smart_health",
    "temperature": $temperature,
    "attributes": {
        "reallocated_sectors": $reallocated,
        "pending_sectors": $pending,
        "crc_errors": $crc_errors,
        "power_on_hours": $power_hours,
        "power_cycles": $power_cycles
    },
    "score": $score,
    "status": "$status"
}
EOF
    else
        local status_color="$GREEN"
        [ "$status" = "warning" ] && status_color="$YELLOW"
        [ "$status" = "critical" ] && status_color="$RED"
        
        echo ""
        echo "  设备:      $device"
        echo "  型号:      ${model:-N/A}"
        echo "  序列号:    ${serial:-N/A}"
        echo "  容量:      ${capacity:-N/A} bytes"
        echo "  类型:      $([ "$is_ssd" = "true" ] && echo "SSD" || echo "HDD (${rotation:-N/A})")"
        echo "  SMART:     $smart_health"
        echo "  温度:      ${temperature}°C"
        echo "  重分配扇区: $reallocated"
        echo "  待定扇区:   $pending"
        echo "  CRC 错误:   $crc_errors"
        echo "  通电时间:   ${power_hours} 小时"
        echo "  通电次数:   ${power_cycles}"
        echo -e "  健康评分:   ${status_color}$score${NC}"
        echo -e "  状态:       ${status_color}$status${NC}"
    fi
    
    # 返回状态
    case "$status" in
        healthy) return 0 ;;
        warning) return 2 ;;
        critical) return 1 ;;
        *) return 3 ;;
    esac
}

# 运行 SMART 测试
run_smart_test() {
    local device="$1"
    local test_type="${2:-short}"
    
    log INFO "在 $device 上运行 SMART $test_type 测试..."
    
    if smartctl -t "$test_type" "$device" 2>/dev/null; then
        log SUCCESS "测试已启动"
        
        # 显示测试进度
        local test_time=2
        [ "$test_type" = "long" ] && test_time=60
        
        log INFO "预计完成时间: ${test_time} 分钟"
        log INFO "使用 '$SCRIPT_NAME --test-status $device' 查看测试状态"
    else
        log ERROR "测试启动失败"
        return 1
    fi
}

# 获取测试状态
get_test_status() {
    local device="$1"
    
    smartctl -l selftest "$device" 2>/dev/null
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
    
    ALERTS+=("[$severity] $message")
    
    log WARN "告警 [$severity]: $message"
    
    # Webhook
    if [ -n "$ALERT_WEBHOOK" ]; then
        local payload=$(cat << EOF
{
    "alert_type": "$alert_type",
    "severity": "$severity",
    "message": "$message",
    "timestamp": "$(get_timestamp)",
    "source": "nas-os-disk-health"
}
EOF
)
        curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_WEBHOOK" 2>/dev/null || true
    fi
    
    # 钉钉
    if [ -n "$ALERT_DINGTALK" ]; then
        local payload=$(cat << EOF
{
    "msgtype": "text",
    "text": {
        "content": "[NAS-OS 磁盘告警]\n级别: $severity\n消息: $message\n时间: $(get_timestamp)"
    }
}
EOF
)
        curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$ALERT_DINGTALK" 2>/dev/null || true
    fi
}

# =============================================================================
# Prometheus 指标
# =============================================================================

export_prometheus_metrics() {
    local metrics_dir=$(dirname "$METRICS_FILE")
    mkdir -p "$metrics_dir" 2>/dev/null || return
    
    local tmp_file=$(mktemp)
    
    # 写入指标头
    cat > "$tmp_file" << 'EOF'
# HELP nas_disk_health_score Disk health score (0-100)
# TYPE nas_disk_health_score gauge

# HELP nas_disk_temperature_celsius Disk temperature in Celsius
# TYPE nas_disk_temperature_celsius gauge

# HELP nas_disk_reallocated_sectors Number of reallocated sectors
# TYPE nas_disk_reallocated_sectors gauge

# HELP nas_disk_pending_sectors Number of pending sectors
# TYPE nas_disk_pending_sectors gauge

# HELP nas_disk_crc_errors Number of CRC errors
# TYPE nas_disk_crc_errors gauge

# HELP nas_disk_power_on_hours Power on hours
# TYPE nas_disk_power_on_hours gauge

# HELP nas_disk_smart_status SMART status (1=healthy, 0=failed, -1=unknown)
# TYPE nas_disk_smart_status gauge
EOF
    
    # 写入各磁盘指标
    for result in "${RESULTS[@]}"; do
        IFS=':' read -r device status score temperature <<< "$result"
        local disk_name=$(basename "$device")
        
        cat >> "$tmp_file" << EOF
nas_disk_health_score{device="$disk_name"} $score
nas_disk_temperature_celsius{device="$disk_name"} $temperature
nas_disk_smart_status{device="$disk_name"} $( [ "$status" = "healthy" ] && echo 1 || ([ "$status" = "critical" ] && echo 0 || echo -1) )
EOF
    done
    
    mv "$tmp_file" "$METRICS_FILE"
    log DEBUG "Prometheus 指标已导出: $METRICS_FILE"
}

# =============================================================================
# 报告生成
# =============================================================================

generate_report() {
    local format="${1:-text}"
    
    # 计算统计数据
    local total=${#RESULTS[@]}
    local healthy=0
    local warning=0
    local critical=0
    local total_score=0
    local avg_temp=0
    
    for result in "${RESULTS[@]}"; do
        IFS=':' read -r device status score temperature <<< "$result"
        total_score=$((total_score + score))
        avg_temp=$((avg_temp + temperature))
        
        case "$status" in
            healthy) ((healthy++)) ;;
            warning) ((warning++)) ;;
            critical) ((critical++)) ;;
        esac
    done
    
    [ $total -gt 0 ] && total_score=$((total_score / total))
    [ $total -gt 0 ] && avg_temp=$((avg_temp / total))
    
    if [ "$format" = "json" ]; then
        cat << EOF
{
    "timestamp": "$(get_timestamp)",
    "summary": {
        "total_disks": $total,
        "healthy": $healthy,
        "warning": $warning,
        "critical": $critical,
        "average_score": $total_score,
        "average_temperature": $avg_temp
    },
    "disks": [
$(for i in "${!RESULTS[@]}"; do
    IFS=':' read -r device status score temperature <<< "${RESULTS[$i]}"
    local comma=","
    [ $i -eq $(( ${#RESULTS[@]} - 1 )) ] && comma=""
    cat << DISK
        {
            "device": "$device",
            "status": "$status",
            "score": $score,
            "temperature": $temperature
        }$comma
DISK
done)
    ],
    "alerts": [
$(for i in "${!ALERTS[@]}"; do
    local comma=","
    [ $i -eq $(( ${#ALERTS[@]} - 1 )) ] && comma=""
    echo "        \"${ALERTS[$i]}\"$comma"
done)
    ]
}
EOF
    else
        cat << EOF
================================================================================
                      NAS-OS 磁盘健康报告
================================================================================
生成时间: $(get_timestamp)

【总体状态】
  检查磁盘: $total
  健康:     $healthy
  警告:     $warning
  严重:     $critical

【健康评分】
  平均评分: $total_score
  平均温度: ${avg_temp}°C

$(if [ ${#ALERTS[@]} -gt 0 ]; then
    echo "【告警列表】"
    for alert in "${ALERTS[@]}"; do
        echo "  - $alert"
    done
fi)
================================================================================
EOF
    fi
}

# =============================================================================
# 监控模式
# =============================================================================

monitor_mode() {
    log INFO "启动磁盘健康监控 (间隔: ${MONITOR_INTERVAL}秒)"
    
    while true; do
        RESULTS=()
        ALERTS=()
        HEALTH_SCORES=()
        
        discover_disks
        
        for disk in "${DISKS[@]}"; do
            check_disk "$disk" > /dev/null
        done
        
        export_prometheus_metrics
        generate_report
        
        sleep "$MONITOR_INTERVAL"
    done
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    local mode="check"
    local target_disk=""
    local test_type="short"
    
    # 解析参数
    while [ $# -gt 0 ]; do
        case "$1" in
            --json|-j)
                OUTPUT_FORMAT="json"
                shift
                ;;
            --smart|-s)
                mode="smart"
                shift
                ;;
            --test|-t)
                mode="test"
                test_type="${2:-short}"
                shift 2
                ;;
            --test-status)
                mode="test-status"
                target_disk="${2:-}"
                shift 2
                ;;
            --monitor|-m)
                mode="monitor"
                shift
                ;;
            --report|-r)
                mode="report"
                shift
                ;;
            --help|-h)
                cat << EOF
用法: $SCRIPT_NAME [选项] [设备]

选项:
    --json, -j              JSON 格式输出
    --smart, -s             显示详细 SMART 信息
    --test, -t <type>       运行 SMART 测试 (short|long)
    --test-status <device>  查看测试状态
    --monitor, -m           持续监控模式
    --report, -r            生成报告
    --help, -h              显示帮助

示例:
    $SCRIPT_NAME                    # 检查所有磁盘
    $SCRIPT_NAME --json             # JSON 格式输出
    $SCRIPT_NAME /dev/sda           # 检查指定磁盘
    $SCRIPT_NAME --test long        # 运行长测试

环境变量:
    TEMP_WARNING           温度警告阈值 (默认: 50°C)
    TEMP_CRITICAL          温度严重阈值 (默认: 60°C)
    ALERT_WEBHOOK          告警 Webhook URL
    ALERT_DINGTALK         钉钉告警 URL
    ALERT_FEISHU           飞书告警 URL
EOF
                exit 0
                ;;
            /dev/*)
                target_disk="$1"
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
    
    # 检查 smartctl
    check_smartctl || exit 1
    
    # 执行模式
    case "$mode" in
        check)
            if [ -n "$target_disk" ]; then
                check_disk "$target_disk"
            else
                discover_disks
                if [ "$OUTPUT_FORMAT" = "json" ]; then
                    echo "{"
                    echo "  \"timestamp\": \"$(get_timestamp)\","
                    echo "  \"disks\": ["
                    
                    for i in "${!DISKS[@]}"; do
                        local comma=","
                        [ $i -eq $(( ${#DISKS[@]} - 1 )) ] && comma=""
                        check_disk "${DISKS[$i]}" | sed 's/^/    /'
                        echo "    $comma"
                    done
                    
                    echo "  ]"
                    echo "}"
                else
                    echo -e "${CYAN}==================== NAS-OS 磁盘健康检查 ====================${NC}"
                    
                    for disk in "${DISKS[@]}"; do
                        check_disk "$disk"
                    done
                    
                    echo ""
                    generate_report
                fi
                
                export_prometheus_metrics
            fi
            ;;
        smart)
            [ -n "$target_disk" ] || { log ERROR "请指定设备"; exit 1; }
            smartctl -a "$target_disk"
            ;;
        test)
            [ -n "$target_disk" ] || { log ERROR "请指定设备"; exit 1; }
            run_smart_test "$target_disk" "$test_type"
            ;;
        test-status)
            [ -n "$target_disk" ] || { log ERROR "请指定设备"; exit 1; }
            get_test_status "$target_disk"
            ;;
        monitor)
            monitor_mode
            ;;
        report)
            discover_disks
            for disk in "${DISKS[@]}"; do
                check_disk "$disk" > /dev/null
            done
            generate_report "${OUTPUT_FORMAT}"
            ;;
    esac
}

main "$@"