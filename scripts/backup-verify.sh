#!/bin/bash
# =============================================================================
# NAS-OS 备份验证脚本 v2.70.0
# =============================================================================
# 用途：验证备份完整性，检查备份文件、校验和、可恢复性
# 用法：./backup-verify.sh [--backup-dir DIR] [--json] [--full]
# =============================================================================

set -euo pipefail

# 版本
VERSION="2.70.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

# =============================================================================
# 默认配置
# =============================================================================

# 备份目录（支持多个）
BACKUP_DIRS="${BACKUP_DIRS:-/var/lib/nas-os/backups /backup/nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"

# 数据目录
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"

# 日志目录
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 校验和算法
CHECKSUM_ALG="${CHECKSUM_ALG:-sha256}"

# 备份保留天数
RETENTION_DAYS="${RETENTION_DAYS:-30}"

# 最小备份大小（MB）
MIN_BACKUP_SIZE_MB="${MIN_BACKUP_SIZE_MB:-1}"

# 输出格式
OUTPUT_FORMAT="${OUTPUT_FORMAT:-text}"

# 完整验证（包含恢复测试）
FULL_VERIFY="${FULL_VERIFY:-false}"

# 验证超时（秒）
VERIFY_TIMEOUT="${VERIFY_TIMEOUT:-300}"

# =============================================================================
# 颜色定义
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# =============================================================================
# 全局状态
# =============================================================================

OVERALL_STATUS="healthy"
CHECKS=()
ERRORS=()
WARNINGS=()
BACKUPS=()
START_TIME=$(date +%s)

# =============================================================================
# 日志函数
# =============================================================================

log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case "$level" in
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - ${message}" >&2
            ;;
        WARN)
            echo -e "${YELLOW}[WARN]${NC} ${timestamp} - ${message}" >&2
            ;;
        INFO)
            echo -e "${GREEN}[INFO]${NC} ${timestamp} - ${message}"
            ;;
        DEBUG)
            if [ "${DEBUG:-false}" = "true" ]; then
                echo -e "${CYAN}[DEBUG]${NC} ${timestamp} - ${message}"
            fi
            ;;
    esac
}

# =============================================================================
# 工具函数
# =============================================================================

# 添加检查结果
add_check() {
    local name="$1"
    local status="$2"
    local message="$3"
    local details="${4:-}"
    
    CHECKS+=("name=$name|status=$status|message=$message|details=$details")
    
    if [ "$status" = "critical" ]; then
        ERRORS+=("$message")
        OVERALL_STATUS="unhealthy"
    elif [ "$status" = "warning" ]; then
        WARNINGS+=("$message")
        [ "$OVERALL_STATUS" = "healthy" ] && OVERALL_STATUS="degraded"
    fi
}

# 检查命令是否存在
check_command() {
    local cmd="$1"
    command -v "$cmd" &> /dev/null
}

# 格式化文件大小
format_size() {
    local bytes=$1
    if [ $bytes -ge 1073741824 ]; then
        echo "$(echo "scale=2; $bytes/1073741824" | bc) GB"
    elif [ $bytes -ge 1048576 ]; then
        echo "$(echo "scale=2; $bytes/1048576" | bc) MB"
    elif [ $bytes -ge 1024 ]; then
        echo "$(echo "scale=2; $bytes/1024" | bc) KB"
    else
        echo "$bytes B"
    fi
}

# 获取文件校验和
get_checksum() {
    local file="$1"
    local alg="${2:-sha256}"
    
    case "$alg" in
        sha256)
            sha256sum "$file" 2>/dev/null | awk '{print $1}'
            ;;
        sha512)
            sha512sum "$file" 2>/dev/null | awk '{print $1}'
            ;;
        md5)
            md5sum "$file" 2>/dev/null | awk '{print $1}'
            ;;
        *)
            sha256sum "$file" 2>/dev/null | awk '{print $1}'
            ;;
    esac
}

# =============================================================================
# 解析参数
# =============================================================================

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --backup-dir)
                BACKUP_DIR="$2"
                shift 2
                ;;
            --json)
                OUTPUT_FORMAT="json"
                shift
                ;;
            --full)
                FULL_VERIFY="true"
                shift
                ;;
            --debug)
                DEBUG="true"
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log ERROR "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << EOF
NAS-OS 备份验证脚本 v${VERSION}

用法: $SCRIPT_NAME [选项]

选项:
    --backup-dir DIR    指定备份目录（默认: $BACKUP_DIR）
    --json              JSON 格式输出
    --full              完整验证（包含恢复测试）
    --debug             显示调试信息
    -h, --help          显示帮助信息

示例:
    $SCRIPT_NAME                        # 验证默认备份目录
    $SCRIPT_NAME --backup-dir /backup   # 验证指定目录
    $SCRIPT_NAME --full                 # 完整验证

EOF
}

# =============================================================================
# 检查函数
# =============================================================================

# 检查备份目录
check_backup_directories() {
    log INFO "检查备份目录..."
    
    local found=0
    local errors=0
    
    # 检查主备份目录
    if [ ! -d "$BACKUP_DIR" ]; then
        add_check "backup_dir" "critical" "备份目录不存在: $BACKUP_DIR"
        return 1
    fi
    
    # 检查目录权限
    if [ ! -r "$BACKUP_DIR" ]; then
        add_check "backup_dir" "critical" "备份目录不可读: $BACKUP_DIR"
        return 1
    fi
    
    # 检查磁盘空间
    local usage=$(df -h "$BACKUP_DIR" 2>/dev/null | tail -1 | awk '{print $5}' | tr -d '%')
    local available=$(df -h "$BACKUP_DIR" 2>/dev/null | tail -1 | awk '{print $4}')
    
    if [ "$usage" -gt 90 ]; then
        add_check "backup_disk" "critical" "备份目录磁盘空间不足: ${usage}% 使用" \
            "path=$BACKUP_DIR, usage=${usage}%, available=$available"
        return 1
    elif [ "$usage" -gt 80 ]; then
        add_check "backup_disk" "warning" "备份目录磁盘空间紧张: ${usage}% 使用" \
            "path=$BACKUP_DIR, usage=${usage}%, available=$available"
    else
        add_check "backup_disk" "healthy" "备份目录磁盘空间充足 (${usage}% 使用, 可用: $available)"
    fi
    
    found=1
    return 0
}

# 查找并分析备份文件
find_backup_files() {
    log INFO "查找备份文件..."
    
    local backup_count=0
    local total_size=0
    local latest_backup=""
    local latest_time=0
    
    # 支持的备份格式
    local patterns=(
        "*.tar.gz"
        "*.tar.bz2"
        "*.tar.xz"
        "*.tgz"
        "*.zip"
        "*.sql.gz"
        "*.sql"
        "*.dump"
        "*.backup"
        "backup-*.tar*"
        "nas-os-backup-*"
    )
    
    # 查找备份文件
    for pattern in "${patterns[@]}"; do
        while IFS= read -r -d '' file; do
            backup_count=$((backup_count + 1))
            local file_size=$(stat -c %s "$file" 2>/dev/null || stat -f %z "$file" 2>/dev/null)
            total_size=$((total_size + file_size))
            
            # 获取文件修改时间
            local file_time=$(stat -c %Y "$file" 2>/dev/null || stat -f %m "$file" 2>/dev/null)
            
            if [ "$file_time" -gt "$latest_time" ]; then
                latest_time=$file_time
                latest_backup="$file"
            fi
            
            BACKUPS+=("$file")
        done < <(find "$BACKUP_DIR" -type f -name "$pattern" -print0 2>/dev/null)
    done
    
    if [ "$backup_count" -eq 0 ]; then
        add_check "backup_files" "warning" "未找到备份文件" "path=$BACKUP_DIR"
        return 1
    fi
    
    # 检查最新备份时间
    local now=$(date +%s)
    local age_hours=$(( (now - latest_time) / 3600 ))
    local age_days=$(( age_hours / 24 ))
    
    local status="healthy"
    local message=""
    
    if [ "$age_days" -gt 7 ]; then
        status="critical"
        message="最新备份已过期 (${age_days} 天前)"
    elif [ "$age_days" -gt 3 ]; then
        status="warning"
        message="最新备份较旧 (${age_days} 天前)"
    else
        status="healthy"
        message="最新备份较新 (${age_hours} 小时前)"
    fi
    
    add_check "backup_freshness" "$status" "$message" \
        "count=$backup_count, total_size=$(format_size $total_size), latest=$(basename "$latest_backup"), age_days=$age_days"
    
    # 检查备份大小
    local min_size_bytes=$((MIN_BACKUP_SIZE_MB * 1048576))
    local avg_size=$((total_size / backup_count))
    
    if [ "$avg_size" -lt "$min_size_bytes" ]; then
        add_check "backup_size" "warning" "备份文件平均大小过小" \
            "avg_size=$(format_size $avg_size), min_expected=$(format_size $min_size_bytes)"
    else
        add_check "backup_size" "healthy" "备份文件大小正常 (平均: $(format_size $avg_size))"
    fi
    
    return 0
}

# 验证备份文件完整性
verify_backup_integrity() {
    log INFO "验证备份文件完整性..."
    
    local verified=0
    local failed=0
    local total=${#BACKUPS[@]}
    
    # 限制验证数量（避免耗时过长）
    local max_verify=5
    local verify_count=0
    
    for backup in "${BACKUPS[@]}"; do
        if [ "$verify_count" -ge "$max_verify" ]; then
            log DEBUG "达到最大验证数量 ($max_verify)，跳过剩余备份"
            break
        fi
        
        verify_count=$((verify_count + 1))
        local filename=$(basename "$backup")
        
        log DEBUG "验证: $filename"
        
        # 检查文件是否可读
        if [ ! -r "$backup" ]; then
            add_check "backup_${verify_count}" "critical" "备份文件不可读: $filename"
            failed=$((failed + 1))
            continue
        fi
        
        # 检查校验和文件
        local checksum_file="${backup}.${CHECKSUM_ALG}"
        if [ -f "$checksum_file" ]; then
            log DEBUG "验证校验和: $checksum_file"
            
            local expected_checksum=$(awk '{print $1}' "$checksum_file" 2>/dev/null)
            local actual_checksum=$(get_checksum "$backup" "$CHECKSUM_ALG")
            
            if [ "$expected_checksum" = "$actual_checksum" ]; then
                add_check "backup_${verify_count}" "healthy" "备份校验通过: $filename" \
                    "checksum=$actual_checksum"
                verified=$((verified + 1))
            else
                add_check "backup_${verify_count}" "critical" "备份校验失败: $filename" \
                    "expected=$expected_checksum, actual=$actual_checksum"
                failed=$((failed + 1))
            fi
            continue
        fi
        
        # 无校验和文件，尝试验证文件格式
        case "$backup" in
            *.tar.gz|*.tgz)
                if gzip -t "$backup" 2>/dev/null && tar -tzf "$backup" &>/dev/null; then
                    add_check "backup_${verify_count}" "healthy" "tar.gz 格式有效: $filename"
                    verified=$((verified + 1))
                else
                    add_check "backup_${verify_count}" "critical" "tar.gz 格式损坏: $filename"
                    failed=$((failed + 1))
                fi
                ;;
            *.tar.bz2)
                if bzip2 -t "$backup" 2>/dev/null && tar -tjf "$backup" &>/dev/null; then
                    add_check "backup_${verify_count}" "healthy" "tar.bz2 格式有效: $filename"
                    verified=$((verified + 1))
                else
                    add_check "backup_${verify_count}" "critical" "tar.bz2 格式损坏: $filename"
                    failed=$((failed + 1))
                fi
                ;;
            *.tar.xz)
                if xz -t "$backup" 2>/dev/null && tar -tJf "$backup" &>/dev/null; then
                    add_check "backup_${verify_count}" "healthy" "tar.xz 格式有效: $filename"
                    verified=$((verified + 1))
                else
                    add_check "backup_${verify_count}" "critical" "tar.xz 格式损坏: $filename"
                    failed=$((failed + 1))
                fi
                ;;
            *.zip)
                if unzip -t "$backup" &>/dev/null; then
                    add_check "backup_${verify_count}" "healthy" "zip 格式有效: $filename"
                    verified=$((verified + 1))
                else
                    add_check "backup_${verify_count}" "critical" "zip 格式损坏: $filename"
                    failed=$((failed + 1))
                fi
                ;;
            *.sql.gz)
                if gzip -t "$backup" 2>/dev/null; then
                    add_check "backup_${verify_count}" "healthy" "sql.gz 格式有效: $filename"
                    verified=$((verified + 1))
                else
                    add_check "backup_${verify_count}" "critical" "sql.gz 格式损坏: $filename"
                    failed=$((failed + 1))
                fi
                ;;
            *)
                # 无法验证格式，生成校验和
                local checksum=$(get_checksum "$backup" "$CHECKSUM_ALG")
                add_check "backup_${verify_count}" "warning" "备份格式未知，已生成校验和: $filename" \
                    "checksum=$checksum, note=consider_adding_checksum_file"
                verified=$((verified + 1))
                ;;
        esac
    done
    
    # 汇总结果
    if [ "$failed" -gt 0 ]; then
        add_check "backup_integrity" "critical" "发现损坏的备份文件" \
            "verified=$verified, failed=$failed, total=$total"
        return 1
    elif [ "$verified" -gt 0 ]; then
        add_check "backup_integrity" "healthy" "所有验证的备份文件完整" \
            "verified=$verified, total=$total"
    else
        add_check "backup_integrity" "warning" "未验证任何备份文件"
    fi
    
    return 0
}

# 验证备份保留策略
verify_retention_policy() {
    log INFO "验证备份保留策略..."
    
    local now=$(date +%s)
    local cutoff=$((now - RETENTION_DAYS * 86400))
    
    local old_count=0
    local old_size=0
    local old_files=()
    
    for backup in "${BACKUPS[@]}"; do
        local file_time=$(stat -c %Y "$backup" 2>/dev/null || stat -f %m "$backup" 2>/dev/null)
        
        if [ "$file_time" -lt "$cutoff" ]; then
            old_count=$((old_count + 1))
            old_size=$((old_size + $(stat -c %s "$backup" 2>/dev/null || stat -f %z "$backup" 2>/dev/null)))
            old_files+=("$backup")
        fi
    done
    
    if [ "$old_count" -gt 0 ]; then
        add_check "retention_policy" "warning" "发现过期备份文件 ($RETENTION_DAYS 天)" \
            "old_count=$old_count, old_size=$(format_size $old_size)"
    else
        add_check "retention_policy" "healthy" "备份保留策略正常"
    fi
    
    return 0
}

# 验证备份目录权限
verify_backup_permissions() {
    log INFO "验证备份目录权限..."
    
    # 检查目录所有者
    local dir_owner=$(stat -c '%U:%G' "$BACKUP_DIR" 2>/dev/null || stat -f '%Su:%Sg' "$BACKUP_DIR" 2>/dev/null)
    
    # 检查目录权限
    local dir_perms=$(stat -c '%a' "$BACKUP_DIR" 2>/dev/null || stat -f '%OLp' "$BACKUP_DIR" 2>/dev/null)
    
    if [ "$dir_perms" -gt "755" ]; then
        add_check "backup_permissions" "warning" "备份目录权限过于宽松" \
            "path=$BACKUP_DIR, permissions=$dir_perms, owner=$dir_owner"
    else
        add_check "backup_permissions" "healthy" "备份目录权限正确" \
            "path=$BACKUP_DIR, permissions=$dir_perms, owner=$dir_owner"
    fi
    
    return 0
}

# 验证备份内容（完整模式）
verify_backup_content() {
    if [ "$FULL_VERIFY" != "true" ]; then
        return 0
    fi
    
    log INFO "执行完整验证（包含内容检查）..."
    
    if [ ${#BACKUPS[@]} -eq 0 ]; then
        add_check "backup_content" "warning" "无备份文件可验证内容"
        return 0
    fi
    
    # 选择最新的备份进行内容验证
    local latest_backup="${BACKUPS[0]}"
    local latest_time=0
    
    for backup in "${BACKUPS[@]}"; do
        local file_time=$(stat -c %Y "$backup" 2>/dev/null || stat -f %m "$backup" 2>/dev/null)
        if [ "$file_time" -gt "$latest_time" ]; then
            latest_time=$file_time
            latest_backup="$backup"
        fi
    done
    
    local filename=$(basename "$latest_backup")
    log INFO "验证备份内容: $filename"
    
    # 创建临时目录
    local temp_dir=$(mktemp -d)
    local cleanup="rm -rf $temp_dir"
    trap "$cleanup" EXIT
    
    # 尝试解压并列出内容
    local content_check="failed"
    local file_count=0
    local expected_files=(
        "config"
        "data"
        "database"
        "metadata"
    )
    local found_files=()
    
    case "$latest_backup" in
        *.tar.gz|*.tgz)
            if tar -xzf "$latest_backup" -C "$temp_dir" 2>/dev/null; then
                content_check="passed"
                file_count=$(find "$temp_dir" -type f | wc -l)
                
                # 检查关键文件
                for expected in "${expected_files[@]}"; do
                    if find "$temp_dir" -name "*$expected*" -type f -o -name "*$expected*" -type d | grep -q .; then
                        found_files+=("$expected")
                    fi
                done
            fi
            ;;
        *.tar.bz2)
            if tar -xjf "$latest_backup" -C "$temp_dir" 2>/dev/null; then
                content_check="passed"
                file_count=$(find "$temp_dir" -type f | wc -l)
            fi
            ;;
        *.zip)
            if unzip -q "$latest_backup" -d "$temp_dir" 2>/dev/null; then
                content_check="passed"
                file_count=$(find "$temp_dir" -type f | wc -l)
            fi
            ;;
    esac
    
    if [ "$content_check" = "passed" ]; then
        add_check "backup_content" "healthy" "备份内容验证通过" \
            "file=$filename, extracted_files=$file_count, found_content=${found_files[*]}"
    else
        add_check "backup_content" "critical" "备份内容验证失败" \
            "file=$filename, error=extraction_failed"
    fi
    
    return 0
}

# 检查备份自动化
check_backup_automation() {
    log INFO "检查备份自动化配置..."
    
    local has_cron=false
    local has_systemd=false
    local has_timer=false
    
    # 检查 cron 任务
    if [ -f /etc/crontab ] && grep -q "backup" /etc/crontab 2>/dev/null; then
        has_cron=true
    fi
    
    for user_cron in /var/spool/cron/crontabs/* /var/spool/cron/*; do
        if [ -f "$user_cron" ] && grep -q "backup" "$user_cron" 2>/dev/null; then
            has_cron=true
            break
        fi
    done
    
    # 检查 systemd timer
    if systemctl list-timers --all 2>/dev/null | grep -q "backup"; then
        has_systemd=true
    fi
    
    # 检查自定义定时任务
    if [ -d /etc/systemd/system ] && ls /etc/systemd/system/*.timer 2>/dev/null | grep -q "backup"; then
        has_timer=true
    fi
    
    if $has_cron || $has_systemd || $has_timer; then
        add_check "backup_automation" "healthy" "检测到备份自动化配置" \
            "cron=$has_cron, systemd=$has_systemd, timer=$has_timer"
    else
        add_check "backup_automation" "warning" "未检测到备份自动化配置" \
            "note=consider_setting_up_automated_backups"
    fi
    
    return 0
}

# =============================================================================
# 输出函数
# =============================================================================

output_text() {
    echo ""
    echo "========================================"
    echo "  NAS-OS 备份验证报告 v${VERSION}"
    echo "========================================"
    echo ""
    
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2)
        
        local status_icon
        case "$status" in
            healthy) status_icon="${GREEN}✓${NC}" ;;
            warning) status_icon="${YELLOW}!${NC}" ;;
            critical) status_icon="${RED}✗${NC}" ;;
            *) status_icon="${BLUE}?${NC}" ;;
        esac
        
        printf "  %s %-25s %s\n" "$status_icon" "$name:" "$message"
    done
    
    echo ""
    echo "========================================"
    
    if [ ${#ERRORS[@]} -gt 0 ]; then
        echo -e "${RED}发现 ${#ERRORS[@]} 个错误:${NC}"
        for err in "${ERRORS[@]}"; do
            echo "  - $err"
        done
    fi
    
    if [ ${#WARNINGS[@]} -gt 0 ]; then
        echo -e "${YELLOW}发现 ${#WARNINGS[@]} 个警告:${NC}"
        for warn in "${WARNINGS[@]}"; do
            echo "  - $warn"
        done
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    
    echo ""
    echo -e "总体状态: $( [ "$OVERALL_STATUS" = "healthy" ] && echo "${GREEN}健康${NC}" || ([ "$OVERALL_STATUS" = "degraded" ] && echo "${YELLOW}降级${NC}" || echo "${RED}异常${NC}") )"
    echo "备份数量: ${#BACKUPS[@]}"
    echo "检查项数: ${#CHECKS[@]}"
    echo "耗时: ${duration}s"
    echo ""
}

output_json() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    
    local checks_json="["
    local first=true
    for check in "${CHECKS[@]}"; do
        local name=$(echo "$check" | grep -o 'name=[^|]*' | cut -d= -f2)
        local status=$(echo "$check" | grep -o 'status=[^|]*' | cut -d= -f2)
        local message=$(echo "$check" | grep -o 'message=[^|]*' | cut -d= -f2)
        local details=$(echo "$check" | grep -o 'details=[^|]*' | cut -d= -f2-)
        
        [ "$first" = "false" ] && checks_json+=","
        first=false
        
        checks_json+="{\"name\":\"$name\",\"status\":\"$status\",\"message\":\"$message\""
        [ -n "$details" ] && checks_json+=",\"details\":\"$details\""
        checks_json+="}"
    done
    checks_json+="]"
    
    local backups_json="["
    first=true
    for backup in "${BACKUPS[@]}"; do
        [ "$first" = "false" ] && backups_json+=","
        first=false
        backups_json+="\"$(basename "$backup")\""
    done
    backups_json+="]"
    
    local errors_json="["
    first=true
    for err in "${ERRORS[@]}"; do
        [ "$first" = "false" ] && errors_json+=","
        first=false
        errors_json+="\"$err\""
    done
    errors_json+="]"
    
    local warnings_json="["
    first=true
    for warn in "${WARNINGS[@]}"; do
        [ "$first" = "false" ] && warnings_json+=","
        first=false
        warnings_json+="\"$warn\""
    done
    warnings_json+="]"
    
    cat << EOF
{
    "version": "$VERSION",
    "timestamp": "$(date -Iseconds)",
    "status": "$OVERALL_STATUS",
    "duration_seconds": $duration,
    "backup_dir": "$BACKUP_DIR",
    "backup_count": ${#BACKUPS[@]},
    "checks": $checks_json,
    "backups": $backups_json,
    "errors": $errors_json,
    "warnings": $warnings_json,
    "summary": {
        "total": ${#CHECKS[@]},
        "healthy": $(grep -c 'status=healthy' <<< "${CHECKS[*]}" 2>/dev/null || echo 0),
        "warnings": ${#WARNINGS[@]},
        "critical": ${#ERRORS[@]}
    }
}
EOF
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    parse_args "$@"
    
    log INFO "开始备份验证..."
    log INFO "版本: $VERSION"
    log INFO "备份目录: $BACKUP_DIR"
    
    # 执行检查
    check_backup_directories
    find_backup_files
    
    if [ ${#BACKUPS[@]} -gt 0 ]; then
        verify_backup_integrity
        verify_retention_policy
        verify_backup_permissions
        verify_backup_content
    fi
    
    check_backup_automation
    
    # 输出结果
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        output_json
    else
        output_text
    fi
    
    # 返回状态码
    if [ "$OVERALL_STATUS" = "healthy" ]; then
        exit 0
    elif [ "$OVERALL_STATUS" = "degraded" ]; then
        exit 1
    else
        exit 2
    fi
}

# 执行
main "$@"