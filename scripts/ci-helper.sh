#!/bin/bash
#
# CI Helper Script - CI 运行辅助工具
# 
# 功能：
# - 检查 CI 环境状态
# - 生成构建报告
# - 缓存管理
# - 测试环境准备
#
# 用法：
#   ./ci-helper.sh [command] [options]
#
# Commands:
#   check        - 检查 CI 环境
#   report       - 生成构建报告
#   cache-clean  - 清理旧缓存
#   prep-test    - 准备测试环境
#   version      - 显示版本信息
#
# v2.103.0 - 工部新增

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 版本信息
SCRIPT_VERSION="v2.103.0"
SCRIPT_NAME="ci-helper.sh"

# 默认配置
CI_ENV_FILE="${CI_ENV_FILE:-/tmp/ci-env.json}"
TEST_DATA_DIR="${TEST_DATA_DIR:-/tmp/nas-os-test}"
TEST_VAR_DIR="${TEST_VAR_DIR:-/var/lib/nas-os}"
CACHE_DIR="${CACHE_DIR:-~/.cache/nas-os-ci}"

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    if [[ "${DEBUG:-false}" == "true" ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

# 显示版本
show_version() {
    echo "$SCRIPT_NAME $SCRIPT_VERSION"
}

# 显示帮助
show_help() {
    cat << EOF
$SCRIPT_NAME - CI 运行辅助工具

用法:
  $SCRIPT_NAME [command] [options]

Commands:
  check        检查 CI 环境状态
  report       生成构建报告
  cache-clean  清理旧缓存文件
  prep-test    准备测试环境目录
  version      显示版本信息
  help         显示帮助信息

Options:
  --json       JSON 格式输出
  --verbose    详细输出
  --dry-run    预览模式（不执行实际操作）

Examples:
  $SCRIPT_NAME check
  $SCRIPT_NAME cache-clean --dry-run
  $SCRIPT_NAME prep-test --json
  $SCRIPT_NAME report --output report.json

环境变量:
  CI_ENV_FILE   CI 环境文件路径 (默认: /tmp/ci-env.json)
  TEST_DATA_DIR 测试数据目录 (默认: /tmp/nas-os-test)
  TEST_VAR_DIR  变量目录 (默认: /var/lib/nas-os)
  CACHE_DIR     缓存目录 (默认: ~/.cache/nas-os-ci)
  DEBUG         启用调试模式 (true/false)

EOF
}

# 检查 CI 环境
check_ci_env() {
    local json_output="${1:-false}"
    
    local result
    result=$(cat << EOF
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "environment": {
    "is_ci": "${CI:-false}",
    "ci_name": "${CI_NAME:-unknown}",
    "github_actions": "${GITHUB_ACTIONS:-false}",
    "github_workflow": "${GITHUB_WORKFLOW:-}",
    "github_run_id": "${GITHUB_RUN_ID:-}",
    "github_run_number": "${GITHUB_RUN_NUMBER:-}",
    "github_sha": "${GITHUB_SHA:-}",
    "github_ref": "${GITHUB_REF:-}",
    "github_repository": "${GITHUB_REPOSITORY:-}",
    "runner_os": "${RUNNER_OS:-unknown}",
    "runner_arch": "${RUNNER_ARCH:-unknown}"
  },
  "go": {
    "version": "$(go version 2>/dev/null | awk '{print $3}' || echo 'not installed')",
    "goroot": "${GOROOT:-}",
    "gopath": "${GOPATH:-}"
  },
  "docker": {
    "version": "$(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',' || echo 'not installed')",
    "buildx": "$(docker buildx version 2>/dev/null | awk '{print $2}' || echo 'not available')"
  },
  "system": {
    "os": "$(uname -s)",
    "arch": "$(uname -m)",
    "kernel": "$(uname -r)",
    "cpu_cores": "$(nproc)",
    "memory_total_mb": "$(free -m | awk '/^Mem:/{print $2}')",
    "disk_available_gb": "$(df -BG . | awk 'NR==2{print $4}' | tr -d 'G')"
  }
}
EOF
)
    
    if [[ "$json_output" == "true" ]]; then
        echo "$result"
    else
        echo "=== CI 环境检查 ==="
        echo ""
        echo "CI 环境: ${CI:-false}"
        echo "GitHub Actions: ${GITHUB_ACTIONS:-false}"
        echo "工作流: ${GITHUB_WORKFLOW:-unknown}"
        echo "运行 ID: ${GITHUB_RUN_ID:-unknown}"
        echo "分支: ${GITHUB_REF:-unknown}"
        echo "提交: ${GITHUB_SHA:-unknown}"
        echo ""
        echo "=== Go 环境 ==="
        go version 2>/dev/null || echo "Go 未安装"
        echo ""
        echo "=== Docker 环境 ==="
        docker --version 2>/dev/null || echo "Docker 未安装"
        echo ""
        echo "=== 系统资源 ==="
        echo "CPU 核心: $(nproc)"
        echo "内存: $(free -h | awk '/^Mem:/{print $2}')"
        echo "磁盘可用: $(df -h . | awk 'NR==2{print $4}')"
    fi
}

# 清理旧缓存
cache_clean() {
    local dry_run="${1:-false}"
    local days="${2:-30}"
    local cache_dir="${CACHE_DIR/#\~/$HOME}"
    
    log_info "缓存目录: $cache_dir"
    
    if [[ ! -d "$cache_dir" ]]; then
        log_warn "缓存目录不存在: $cache_dir"
        return 0
    fi
    
    local count
    local size
    
    count=$(find "$cache_dir" -type f -mtime +$days 2>/dev/null | wc -l)
    size=$(find "$cache_dir" -type f -mtime +$days -exec du -ch {} + 2>/dev/null | grep total$ | awk '{print $1}')
    
    if [[ "$dry_run" == "true" ]]; then
        log_info "[DRY-RUN] 将删除 $count 个文件，共 $size"
        find "$cache_dir" -type f -mtime +$days -ls 2>/dev/null || true
    else
        log_info "清理 $days 天前的缓存..."
        find "$cache_dir" -type f -mtime +$days -delete 2>/dev/null || true
        find "$cache_dir" -type d -empty -delete 2>/dev/null || true
        log_info "已删除 $count 个文件，释放 $size 空间"
    fi
}

# 准备测试环境
prepare_test_env() {
    local json_output="${1:-false}"
    local test_dir="${TEST_DATA_DIR}"
    local var_dir="${TEST_VAR_DIR}"
    
    log_info "准备测试环境..."
    
    # 创建测试数据目录
    mkdir -p "$test_dir"
    chmod 755 "$test_dir"
    
    # 创建变量目录
    sudo mkdir -p "$var_dir"
    sudo chmod 777 "$var_dir"
    
    # 创建子目录
    sudo mkdir -p "$var_dir/backups"
    sudo mkdir -p "$var_dir/plugins"
    sudo mkdir -p "$var_dir/config"
    sudo mkdir -p "$var_dir/snapshots"
    sudo mkdir -p "$var_dir/data"
    sudo chmod -R 777 "$var_dir"
    
    # 设置环境变量
    export NAS_OS_TEST_DIR="$test_dir"
    export NAS_OS_VAR_DIR="$var_dir"
    
    local status="success"
    local message="测试环境准备完成"
    
    if [[ "$json_output" == "true" ]]; then
        cat << EOF
{
  "status": "$status",
  "message": "$message",
  "directories": {
    "test_data": "$test_dir",
    "var_dir": "$var_dir",
    "backups": "$var_dir/backups",
    "plugins": "$var_dir/plugins",
    "config": "$var_dir/config",
    "snapshots": "$var_dir/snapshots",
    "data": "$var_dir/data"
  },
  "environment": {
    "NAS_OS_TEST_DIR": "$test_dir",
    "NAS_OS_VAR_DIR": "$var_dir"
  }
}
EOF
    else
        log_info "$message"
        echo "  - 测试数据目录: $test_dir"
        echo "  - 变量目录: $var_dir"
    fi
}

# 生成构建报告
generate_report() {
    local output_file="${1:-}"
    local report
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    local go_version
    go_version=$(go version 2>/dev/null | awk '{print $3}' || echo "unknown")
    
    local build_time=""
    local start_time="${BUILD_START_TIME:-}"
    if [[ -n "$start_time" ]]; then
        build_time="$(($(date +%s) - $(date -d "$start_time" +%s 2>/dev/null || echo 0)))s"
    fi
    
    report=$(cat << EOF
{
  "timestamp": "$timestamp",
  "version": "$SCRIPT_VERSION",
  "ci": {
    "workflow": "${GITHUB_WORKFLOW:-}",
    "run_id": "${GITHUB_RUN_ID:-}",
    "run_number": "${GITHUB_RUN_NUMBER:-}",
    "sha": "${GITHUB_SHA:-}",
    "ref": "${GITHUB_REF:-}",
    "actor": "${GITHUB_ACTOR:-}"
  },
  "build": {
    "go_version": "$go_version",
    "duration": "$build_time",
    "cache_hit": "${CACHE_HIT:-unknown}"
  },
  "coverage": {
    "total": "${COVERAGE:-unknown}",
    "threshold": "${COVERAGE_THRESHOLD:-25}"
  },
  "environment": {
    "os": "$(uname -s)",
    "arch": "$(uname -m)",
    "cpu_cores": $(nproc),
    "memory_mb": $(free -m | awk '/^Mem:/{print $2}')
  }
}
EOF
)
    
    if [[ -n "$output_file" ]]; then
        echo "$report" | jq '.' > "$output_file"
        log_info "报告已保存到: $output_file"
    else
        echo "$report"
    fi
}

# 主函数
main() {
    local command="${1:-help}"
    shift || true
    
    local json_output="false"
    local dry_run="false"
    local verbose="false"
    local output_file=""
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                json_output="true"
                shift
                ;;
            --dry-run)
                dry_run="true"
                shift
                ;;
            --verbose)
                verbose="true"
                shift
                ;;
            --output)
                output_file="$2"
                shift 2
                ;;
            *)
                shift
                ;;
        esac
    done
    
    # 执行命令
    case "$command" in
        check)
            check_ci_env "$json_output"
            ;;
        report)
            generate_report "$output_file"
            ;;
        cache-clean)
            cache_clean "$dry_run"
            ;;
        prep-test)
            prepare_test_env "$json_output"
            ;;
        version|--version|-v)
            show_version
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"