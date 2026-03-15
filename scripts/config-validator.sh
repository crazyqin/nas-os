#!/bin/bash
# =============================================================================
# NAS-OS 配置验证脚本 v2.72.0
# =============================================================================
# 用途：验证配置文件的正确性和完整性
# 用法：
#   ./config-validator.sh                  # 验证所有配置
#   ./config-validator.sh --config FILE    # 验证指定配置文件
#   ./config-validator.sh --strict         # 严格模式（警告也报错）
#   ./config-validator.sh --fix            # 自动修复简单问题
#   ./config-validator.sh --json           # JSON 输出
# =============================================================================

set -euo pipefail

#===========================================
# 版本与路径
#===========================================

VERSION="2.72.0"
SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

#===========================================
# 配置
#===========================================

# 默认配置路径
CONFIG_DIR="${CONFIG_DIR:-/etc/nas-os}"
DEFAULT_CONFIG="${DEFAULT_CONFIG:-/etc/nas-os/config.yaml}"

# 必需配置项
REQUIRED_FIELDS=(
    "server.port"
    "server.host"
    "database.path"
    "storage.pool_path"
)

# 推荐配置项
RECOMMENDED_FIELDS=(
    "logging.level"
    "logging.output"
    "metrics.enabled"
    "backup.enabled"
    "backup.schedule"
)

# 配置值范围
declare -A VALID_RANGES
VALID_RANGES["server.port"]="1-65535"
VALID_RANGES["server.timeout"]="1-3600"
VALID_RANGES["database.pool_size"]="1-100"
VALID_RANGES["backup.retention_days"]="1-365"
VALID_RANGES["logging.level"]="debug,info,warn,error"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

#===========================================
# 统计变量
#===========================================

ERRORS=()
WARNINGS=()
FIXES=()
CHECKS_PASSED=0
CHECKS_FAILED=0

#===========================================
# 工具函数
#===========================================

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
        STEP)    echo -e "${CYAN}[$timestamp] [STEP]${NC} $msg" ;;
        *)       echo "[$timestamp] $msg" ;;
    esac
}

# 检查命令是否存在
check_command() {
    command -v "$1" >/dev/null 2>&1
}

# 添加错误
add_error() {
    ERRORS+=("$1")
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
}

# 添加警告
add_warning() {
    WARNINGS+=("$1")
}

# 添加修复
add_fix() {
    FIXES+=("$1")
}

#===========================================
# 验证函数
#===========================================

# 检查文件存在
check_file_exists() {
    local file="$1"
    
    if [[ ! -f "$file" ]]; then
        add_error "配置文件不存在: $file"
        return 1
    fi
    
    if [[ ! -r "$file" ]]; then
        add_error "配置文件不可读: $file"
        return 1
    fi
    
    CHECKS_PASSED=$((CHECKS_PASSED + 1))
    return 0
}

# 检查 YAML 语法
check_yaml_syntax() {
    local file="$1"
    
    log STEP "检查 YAML 语法..."
    
    if check_command python3; then
        if python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
            log SUCCESS "YAML 语法正确"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
            return 0
        else
            add_error "YAML 语法错误"
            python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>&1 | head -5
            return 1
        fi
    elif check_command yq; then
        if yq eval '.' "$file" >/dev/null 2>&1; then
            log SUCCESS "YAML 语法正确"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
            return 0
        else
            add_error "YAML 语法错误"
            yq eval '.' "$file" 2>&1 | head -5
            return 1
        fi
    else
        log WARN "未安装 python3 或 yq，跳过 YAML 语法检查"
        add_warning "跳过 YAML 语法检查（缺少工具）"
        return 0
    fi
}

# 获取配置值
get_config_value() {
    local file="$1"
    local path="$2"
    local value=""
    
    if check_command yq; then
        value=$(yq eval ".$path" "$file" 2>/dev/null)
    elif check_command python3; then
        value=$(python3 -c "
import yaml
import sys
try:
    config = yaml.safe_load(open('$file'))
    keys = '$path'.split('.')
    result = config
    for key in keys:
        if isinstance(result, dict) and key in result:
            result = result[key]
        else:
            result = None
            break
    if result is not None:
        print(result)
except:
    pass
" 2>/dev/null)
    fi
    
    echo "$value"
}

# 检查必需字段
check_required_fields() {
    local file="$1"
    
    log STEP "检查必需配置项..."
    
    for field in "${REQUIRED_FIELDS[@]}"; do
        local value=$(get_config_value "$file" "$field")
        
        if [[ -z "$value" ]] || [[ "$value" == "null" ]]; then
            add_error "缺少必需配置项: $field"
        else
            log SUCCESS "✓ $field = $value"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
        fi
    done
}

# 检查推荐字段
check_recommended_fields() {
    local file="$1"
    
    log STEP "检查推荐配置项..."
    
    for field in "${RECOMMENDED_FIELDS[@]}"; do
        local value=$(get_config_value "$file" "$field")
        
        if [[ -z "$value" ]] || [[ "$value" == "null" ]]; then
            add_warning "缺少推荐配置项: $field"
        else
            log SUCCESS "✓ $field = $value"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
        fi
    done
}

# 检查配置值范围
check_value_ranges() {
    local file="$1"
    
    log STEP "检查配置值范围..."
    
    for field in "${!VALID_RANGES[@]}"; do
        local value=$(get_config_value "$file" "$field")
        local range="${VALID_RANGES[$field]}"
        
        if [[ -n "$value" ]] && [[ "$value" != "null" ]]; then
            # 检查数值范围
            if [[ "$range" =~ ^[0-9]+-[0-9]+$ ]]; then
                local min="${range%-*}"
                local max="${range#*-}"
                
                if [[ "$value" =~ ^[0-9]+$ ]]; then
                    if [[ $value -lt $min ]] || [[ $value -gt $max ]]; then
                        add_error "配置值超出范围: $field = $value (有效范围: $min-$max)"
                    else
                        log SUCCESS "✓ $field = $value (范围: $min-$max)"
                        CHECKS_PASSED=$((CHECKS_PASSED + 1))
                    fi
                fi
            # 检查枚举值
            elif [[ "$range" =~ , ]]; then
                local valid_values=(${range//,/ })
                local found=false
                
                for valid in "${valid_values[@]}"; do
                    if [[ "$value" == "$valid" ]]; then
                        found=true
                        break
                    fi
                done
                
                if [[ "$found" == "false" ]]; then
                    add_error "配置值无效: $field = $value (有效值: $range)"
                else
                    log SUCCESS "✓ $field = $value"
                    CHECKS_PASSED=$((CHECKS_PASSED + 1))
                fi
            fi
        fi
    done
}

# 检查路径存在性
check_paths() {
    local file="$1"
    
    log STEP "检查路径配置..."
    
    # 获取路径配置
    local paths=(
        "storage.pool_path"
        "database.path"
        "backup.path"
        "logs.path"
    )
    
    for path_field in "${paths[@]}"; do
        local path=$(get_config_value "$file" "$path_field")
        
        if [[ -n "$path" ]] && [[ "$path" != "null" ]]; then
            if [[ ! -d "$path" ]]; then
                add_warning "路径不存在: $path_field = $path"
            else
                # 检查权限
                if [[ ! -w "$path" ]]; then
                    add_error "路径不可写: $path_field = $path"
                else
                    log SUCCESS "✓ $path_field = $path (可写)"
                    CHECKS_PASSED=$((CHECKS_PASSED + 1))
                fi
            fi
        fi
    done
}

# 检查端口冲突
check_port_conflict() {
    local file="$1"
    
    log STEP "检查端口配置..."
    
    local port=$(get_config_value "$file" "server.port")
    
    if [[ -n "$port" ]] && [[ "$port" != "null" ]]; then
        # 检查端口是否被占用
        if command -v ss >/dev/null 2>&1; then
            if ss -tuln | grep -q ":$port "; then
                # 检查是否是自己
                if ss -tuln | grep ":$port " | grep -q "LISTEN"; then
                    add_warning "端口 $port 已被占用（可能是 NAS-OS 本身）"
                else
                    add_error "端口 $port 已被其他进程占用"
                fi
            else
                log SUCCESS "端口 $port 可用"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            fi
        elif command -v netstat >/dev/null 2>&1; then
            if netstat -tuln | grep -q ":$port "; then
                add_warning "端口 $port 已被占用"
            else
                log SUCCESS "端口 $port 可用"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            fi
        fi
    fi
}

# 检查数据库配置
check_database_config() {
    local file="$1"
    
    log STEP "检查数据库配置..."
    
    local db_path=$(get_config_value "$file" "database.path")
    local db_type=$(get_config_value "$file" "database.type")
    
    if [[ -n "$db_path" ]] && [[ "$db_path" != "null" ]]; then
        # 检查数据库文件
        if [[ -f "$db_path" ]]; then
            # 检查文件大小
            local size=$(stat -c%s "$db_path" 2>/dev/null || stat -f%z "$db_path" 2>/dev/null || echo "0")
            if [[ $size -gt 0 ]]; then
                log SUCCESS "数据库文件存在: $db_path ($(numfmt --to=iec $size 2>/dev/null || echo $size bytes))"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            else
                add_warning "数据库文件为空: $db_path"
            fi
        else
            add_warning "数据库文件不存在: $db_path（将在首次启动时创建）"
        fi
    fi
    
    # 检查数据库类型
    if [[ -n "$db_type" ]] && [[ "$db_type" != "null" ]]; then
        case "$db_type" in
            sqlite|sqlite3|boltdb|badger)
                log SUCCESS "数据库类型: $db_type"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
                ;;
            *)
                add_warning "未知数据库类型: $db_type"
                ;;
        esac
    fi
}

# 检查备份配置
check_backup_config() {
    local file="$1"
    
    log STEP "检查备份配置..."
    
    local enabled=$(get_config_value "$file" "backup.enabled")
    local backup_path=$(get_config_value "$file" "backup.path")
    local retention=$(get_config_value "$file" "backup.retention_days")
    
    if [[ "$enabled" == "true" ]]; then
        log SUCCESS "备份已启用"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        
        if [[ -n "$backup_path" ]] && [[ "$backup_path" != "null" ]]; then
            if [[ ! -d "$backup_path" ]]; then
                add_warning "备份目录不存在: $backup_path"
            else
                # 检查备份目录权限
                if [[ ! -w "$backup_path" ]]; then
                    add_error "备份目录不可写: $backup_path"
                else
                    log SUCCESS "备份目录可写: $backup_path"
                    CHECKS_PASSED=$((CHECKS_PASSED + 1))
                fi
                
                # 检查现有备份
                local backup_count=$(find "$backup_path" -name "*.bak" -o -name "*.tar.gz" 2>/dev/null | wc -l)
                log INFO "现有备份数量: $backup_count"
            fi
        fi
        
        if [[ -n "$retention" ]] && [[ "$retention" != "null" ]]; then
            if [[ $retention -lt 1 ]] || [[ $retention -gt 365 ]]; then
                add_warning "备份保留天数建议在 1-365 之间: $retention"
            else
                log SUCCESS "备份保留天数: $retention"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            fi
        fi
    else
        add_warning "备份未启用"
    fi
}

# 检查日志配置
check_logging_config() {
    local file="$1"
    
    log STEP "检查日志配置..."
    
    local level=$(get_config_value "$file" "logging.level")
    local output=$(get_config_value "$file" "logging.output")
    local max_size=$(get_config_value "$file" "logging.max_size")
    
    # 检查日志级别
    if [[ -n "$level" ]] && [[ "$level" != "null" ]]; then
        case "$level" in
            debug|info|warn|error)
                log SUCCESS "日志级别: $level"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
                ;;
            *)
                add_error "无效日志级别: $level (有效值: debug, info, warn, error)"
                ;;
        esac
    fi
    
    # 检查日志输出
    if [[ -n "$output" ]] && [[ "$output" != "null" ]]; then
        if [[ "$output" == "stdout" ]] || [[ "$output" == "stderr" ]]; then
            log SUCCESS "日志输出: $output"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
        elif [[ -f "$output" ]]; then
            if [[ -w "$output" ]]; then
                log SUCCESS "日志文件可写: $output"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            else
                add_error "日志文件不可写: $output"
            fi
        else
            # 检查目录是否存在
            local dir=$(dirname "$output")
            if [[ -d "$dir" ]] && [[ -w "$dir" ]]; then
                log SUCCESS "日志目录可写: $dir"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            else
                add_warning "日志目录不存在或不可写: $dir"
            fi
        fi
    fi
}

# 检查安全配置
check_security_config() {
    local file="$1"
    
    log STEP "检查安全配置..."
    
    # 检查认证配置
    local auth_enabled=$(get_config_value "$file" "auth.enabled")
    local jwt_secret=$(get_config_value "$file" "auth.jwt_secret")
    local session_timeout=$(get_config_value "$file" "auth.session_timeout")
    
    if [[ "$auth_enabled" != "false" ]]; then
        log SUCCESS "认证已启用"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        
        # 检查 JWT 密钥
        if [[ -z "$jwt_secret" ]] || [[ "$jwt_secret" == "null" ]]; then
            add_warning "未配置 JWT 密钥，将使用默认值"
        elif [[ ${#jwt_secret} -lt 16 ]]; then
            add_warning "JWT 密钥长度建议至少 16 字符"
        else
            log SUCCESS "JWT 密钥已配置"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
        fi
        
        # 检查会话超时
        if [[ -n "$session_timeout" ]] && [[ "$session_timeout" != "null" ]]; then
            if [[ $session_timeout -lt 300 ]]; then
                add_warning "会话超时时间过短: ${session_timeout}s（建议至少 5 分钟）"
            elif [[ $session_timeout -gt 86400 ]]; then
                add_warning "会话超时时间过长: ${session_timeout}s（建议不超过 24 小时）"
            else
                log SUCCESS "会话超时: ${session_timeout}s"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            fi
        fi
    else
        add_warning "认证已禁用（不推荐）"
    fi
    
    # 检查 HTTPS 配置
    local tls_enabled=$(get_config_value "$file" "server.tls.enabled")
    local tls_cert=$(get_config_value "$file" "server.tls.cert")
    local tls_key=$(get_config_value "$file" "server.tls.key")
    
    if [[ "$tls_enabled" == "true" ]]; then
        log SUCCESS "TLS 已启用"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        
        if [[ -n "$tls_cert" ]] && [[ "$tls_cert" != "null" ]]; then
            if [[ ! -f "$tls_cert" ]]; then
                add_error "TLS 证书不存在: $tls_cert"
            else
                log SUCCESS "TLS 证书存在"
                CHECKS_PASSED=$((CHECKS_PASSED + 1))
            fi
        fi
        
        if [[ -n "$tls_key" ]] && [[ "$tls_key" != "null" ]]; then
            if [[ ! -f "$tls_key" ]]; then
                add_error "TLS 密钥不存在: $tls_key"
            else
                # 检查密钥权限
                local key_perms=$(stat -c%a "$tls_key" 2>/dev/null || stat -f%OLp "$tls_key" 2>/dev/null)
                if [[ "$key_perms" != "600" ]] && [[ "$key_perms" != "400" ]]; then
                    add_warning "TLS 密钥权限不安全: $tls_key (建议: 600 或 400)"
                else
                    log SUCCESS "TLS 密钥权限正确"
                    CHECKS_PASSED=$((CHECKS_PASSED + 1))
                fi
            fi
        fi
    else
        add_warning "TLS 未启用（不推荐用于生产环境）"
    fi
}

# 验证单个配置文件
validate_config() {
    local file="$1"
    local strict="${2:-false}"
    local fix="${3:-false}"
    
    log INFO "验证配置文件: $file"
    echo ""
    
    # 检查文件存在
    if ! check_file_exists "$file"; then
        return 1
    fi
    
    # YAML 语法检查
    check_yaml_syntax "$file"
    
    # 必需字段检查
    check_required_fields "$file"
    
    # 推荐字段检查
    check_recommended_fields "$file"
    
    # 配置值范围检查
    check_value_ranges "$file"
    
    # 路径检查
    check_paths "$file"
    
    # 端口冲突检查
    check_port_conflict "$file"
    
    # 数据库配置检查
    check_database_config "$file"
    
    # 备份配置检查
    check_backup_config "$file"
    
    # 日志配置检查
    check_logging_config "$file"
    
    # 安全配置检查
    check_security_config "$file"
}

# 显示验证结果
show_results() {
    local strict="${1:-false}"
    
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}                     验证结果                              ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  通过检查: $CHECKS_PASSED"
    echo "  失败检查: $CHECKS_FAILED"
    echo ""
    
    # 显示错误
    if [[ ${#ERRORS[@]} -gt 0 ]]; then
        echo -e "${RED}错误 (${#ERRORS[@]}):${NC}"
        for error in "${ERRORS[@]}"; do
            echo -e "  ${RED}✗${NC} $error"
        done
        echo ""
    fi
    
    # 显示警告
    if [[ ${#WARNINGS[@]} -gt 0 ]]; then
        echo -e "${YELLOW}警告 (${#WARNINGS[@]}):${NC}"
        for warning in "${WARNINGS[@]}"; do
            echo -e "  ${YELLOW}!${NC} $warning"
        done
        echo ""
    fi
    
    # 显示修复
    if [[ ${#FIXES[@]} -gt 0 ]]; then
        echo -e "${GREEN}修复 (${#FIXES[@]}):${NC}"
        for fix in "${FIXES[@]}"; do
            echo -e "  ${GREEN}✓${NC} $fix"
        done
        echo ""
    fi
    
    # 总结
    if [[ ${#ERRORS[@]} -gt 0 ]]; then
        echo -e "${RED}✗ 配置验证失败${NC}"
        return 1
    elif [[ ${#WARNINGS[@]} -gt 0 ]] && [[ "$strict" == "true" ]]; then
        echo -e "${YELLOW}✗ 配置验证失败（严格模式）${NC}"
        return 1
    elif [[ ${#WARNINGS[@]} -gt 0 ]]; then
        echo -e "${YELLOW}✓ 配置验证通过（有警告）${NC}"
        return 0
    else
        echo -e "${GREEN}✓ 配置验证通过${NC}"
        return 0
    fi
}

# 显示帮助
show_help() {
    cat <<EOF
NAS-OS 配置验证工具 v${VERSION}

用法: $0 [options]

选项:
  --config FILE   验证指定配置文件
  --strict        严格模式（警告也报错）
  --fix           自动修复简单问题
  --json          JSON 格式输出
  -h, --help      显示帮助

示例:
  $0                           # 验证默认配置
  $0 --config /etc/nas-os/config.yaml
  $0 --strict                  # 严格模式
  $0 --fix                     # 自动修复

环境变量:
  CONFIG_DIR      配置目录 (默认: /etc/nas-os)
  DEFAULT_CONFIG  默认配置文件 (默认: /etc/nas-os/config.yaml)

检查项:
  - YAML 语法
  - 必需配置项
  - 推荐配置项
  - 配置值范围
  - 路径存在性和权限
  - 端口冲突
  - 数据库配置
  - 备份配置
  - 日志配置
  - 安全配置
EOF
}

#===========================================
# 主函数
#===========================================

main() {
    local config_file="$DEFAULT_CONFIG"
    local strict=false
    local fix=false
    local json_output=false
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --config)
                config_file="$2"
                shift 2
                ;;
            --strict)
                strict=true
                shift
                ;;
            --fix)
                fix=true
                shift
                ;;
            --json)
                json_output=true
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
    
    # 验证配置
    validate_config "$config_file" "$strict" "$fix"
    
    # 显示结果
    show_results "$strict"
}

# 运行
main "$@"