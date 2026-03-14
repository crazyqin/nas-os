#!/bin/bash
# NAS-OS 内存分析脚本
# 用法: ./scripts/memory_profile.sh [command] [options]
# 命令:
#   heap      - 堆内存分析
#   goroutine - Goroutine 分析
#   cpu       - CPU 分析（包含内存分配）
#   full      - 完整内存分析报告
#   compare   - 比较两次内存快照

set -e

# 配置
OUTPUT_DIR="reports/performance"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BINARY="${BINARY:-./nasd}"
PORT="${PORT:-6060}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 辅助函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检查 pprof 是否可用
check_pprof() {
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装"
        exit 1
    fi

    # 检查服务是否启用了 pprof
    if ! curl -s "http://localhost:$PORT/debug/pprof/" > /dev/null 2>&1; then
        log_warn "服务可能未启用 pprof，端口: $PORT"
        log_info "请确保服务启动时设置了 -pprof 或在代码中导入了 net/http/pprof"
    fi
}

# 堆内存分析
analyze_heap() {
    log_step "开始堆内存分析..."

    local output_file="$OUTPUT_DIR/heap_${TIMESTAMP}.prof"
    local svg_file="$OUTPUT_DIR/heap_${TIMESTAMP}.svg"
    local txt_file="$OUTPUT_DIR/heap_${TIMESTAMP}.txt"

    # 获取堆快照
    log_info "获取堆快照..."
    curl -s "http://localhost:$PORT/debug/pprof/heap" -o "$output_file"

    # 生成文本报告
    log_info "生成文本报告..."
    go tool pprof -text "$output_file" > "$txt_file" 2>/dev/null || {
        log_error "无法生成文本报告"
    }

    # 生成 SVG 图
    log_info "生成可视化图表..."
    go tool pprof -svg "$output_file" > "$svg_file" 2>/dev/null || {
        log_warn "无法生成 SVG，尝试生成 PNG..."
        go tool pprof -png "$output_file" > "${svg_file%.svg}.png" 2>/dev/null || {
            log_error "无法生成图表"
        }
    }

    # 打印摘要
    echo ""
    log_info "=== 堆内存摘要 ==="
    if [ -f "$txt_file" ]; then
        head -20 "$txt_file"
    fi

    echo ""
    log_info "报告已保存:"
    echo "  - Profile: $output_file"
    echo "  - 文本报告: $txt_file"
    [ -f "$svg_file" ] && echo "  - SVG 图: $svg_file"
    [ -f "${svg_file%.svg}.png" ] && echo "  - PNG 图: ${svg_file%.svg}.png"
}

# Goroutine 分析
analyze_goroutine() {
    log_step "开始 Goroutine 分析..."

    local output_file="$OUTPUT_DIR/goroutine_${TIMESTAMP}.prof"
    local txt_file="$OUTPUT_DIR/goroutine_${TIMESTAMP}.txt"

    # 获取 Goroutine 快照
    log_info "获取 Goroutine 快照..."
    curl -s "http://localhost:$PORT/debug/pprof/goroutine" -o "$output_file"

    # 生成文本报告
    log_info "生成文本报告..."
    go tool pprof -text "$output_file" > "$txt_file" 2>/dev/null || {
        log_error "无法生成文本报告"
    }

    # 统计 goroutine 数量
    local count=$(grep -c "goroutine" "$output_file" 2>/dev/null || echo "未知")

    echo ""
    log_info "=== Goroutine 分析 ==="
    echo "Goroutine 数量: $count"

    if [ -f "$txt_file" ]; then
        echo ""
        echo "主要堆栈:"
        head -30 "$txt_file"
    fi

    echo ""
    log_info "报告已保存:"
    echo "  - Profile: $output_file"
    echo "  - 文本报告: $txt_file"
}

# CPU 分析（包含内存分配信息）
analyze_cpu() {
    log_step "开始 CPU 分析..."

    local duration="${1:-30s}"
    local output_file="$OUTPUT_DIR/cpu_${TIMESTAMP}.prof"
    local svg_file="$OUTPUT_DIR/cpu_${TIMESTAMP}.svg"
    local txt_file="$OUTPUT_DIR/cpu_${TIMESTAMP}.txt"

    log_info "采集 CPU profile (${duration})..."
    curl -s "http://localhost:$PORT/debug/pprof/profile?seconds=${duration%s}" -o "$output_file"

    # 生成文本报告
    log_info "生成文本报告..."
    go tool pprof -text "$output_file" > "$txt_file" 2>/dev/null || {
        log_error "无法生成文本报告"
    }

    # 生成 SVG 图
    log_info "生成可视化图表..."
    go tool pprof -svg "$output_file" > "$svg_file" 2>/dev/null || true

    # 打印摘要
    echo ""
    log_info "=== CPU 分析摘要 ==="
    if [ -f "$txt_file" ]; then
        head -20 "$txt_file"
    fi

    echo ""
    log_info "报告已保存:"
    echo "  - Profile: $output_file"
    echo "  - 文本报告: $txt_file"
    [ -f "$svg_file" ] && echo "  - SVG 图: $svg_file"
}

# 分配分析
analyze_allocs() {
    log_step "开始内存分配分析..."

    local output_file="$OUTPUT_DIR/allocs_${TIMESTAMP}.prof"
    local svg_file="$OUTPUT_DIR/allocs_${TIMESTAMP}.svg"
    local txt_file="$OUTPUT_DIR/allocs_${TIMESTAMP}.txt"

    # 获取分配快照
    log_info "获取分配快照..."
    curl -s "http://localhost:$PORT/debug/pprof/allocs" -o "$output_file"

    # 生成文本报告
    log_info "生成文本报告..."
    go tool pprof -text "$output_file" > "$txt_file" 2>/dev/null || {
        log_error "无法生成文本报告"
    }

    # 生成 SVG 图
    log_info "生成可视化图表..."
    go tool pprof -svg "$output_file" > "$svg_file" 2>/dev/null || true

    # 打印摘要
    echo ""
    log_info "=== 内存分配摘要 ==="
    if [ -f "$txt_file" ]; then
        head -20 "$txt_file"
    fi

    echo ""
    log_info "报告已保存:"
    echo "  - Profile: $output_file"
    echo "  - 文本报告: $txt_file"
    [ -f "$svg_file" ] && echo "  - SVG 图: $svg_file"
}

# 完整内存分析
analyze_full() {
    log_step "开始完整内存分析..."

    echo "=========================================="
    echo "     NAS-OS 完整内存分析"
    echo "=========================================="
    echo ""

    # 堆内存
    analyze_heap
    echo ""

    # Goroutine
    analyze_goroutine
    echo ""

    # 内存分配
    analyze_allocs
    echo ""

    # 系统内存状态
    log_step "系统内存状态..."
    if command -v free &> /dev/null; then
        free -h
    fi

    echo ""
    # 进程内存状态
    log_step "进程内存状态..."
    if pid=$(pgrep -f "nasd" | head -1); then
        if [ -n "$pid" ]; then
            ps -o pid,rss,vsz,pmem,comm -p "$pid" 2>/dev/null || \
                ps -o pid,rss,vsz,pcpu,comm -p "$pid"
        fi
    fi

    # 生成汇总报告
    local report_file="$OUTPUT_DIR/memory_report_${TIMESTAMP}.md"
    generate_report "$report_file"

    echo ""
    log_info "完整报告已保存到: $report_file"
}

# 生成汇总报告
generate_report() {
    local report_file="$1"

    cat > "$report_file" << EOF
# NAS-OS 内存分析报告

**分析时间:** $(date '+%Y-%m-%d %H:%M:%S')
**服务端口:** $PORT

## 系统内存状态

\`\`\`
$(free -h 2>/dev/null || echo "无法获取")
\`\`\`

## 进程内存状态

\`\`\`
$(ps aux | grep nasd | grep -v grep || echo "未找到 nasd 进程")
\`\`\`

## 堆内存分析

\`\`\`
$(cat "$OUTPUT_DIR/heap_${TIMESTAMP}.txt" 2>/dev/null | head -30 || echo "无数据")
\`\`\`

## Goroutine 分析

\`\`\`
$(cat "$OUTPUT_DIR/goroutine_${TIMESTAMP}.txt" 2>/dev/null | head -20 || echo "无数据")
\`\`\`

## 内存分配分析

\`\`\`
$(cat "$OUTPUT_DIR/allocs_${TIMESTAMP}.txt" 2>/dev/null | head -20 || echo "无数据")
\`\`\`

## 文件列表

| 文件 | 描述 |
|------|------|
| heap_${TIMESTAMP}.prof | 堆内存 Profile |
| heap_${TIMESTAMP}.svg | 堆内存可视化 |
| goroutine_${TIMESTAMP}.prof | Goroutine Profile |
| allocs_${TIMESTAMP}.prof | 内存分配 Profile |
| allocs_${TIMESTAMP}.svg | 内存分配可视化 |

## 分析建议

EOF

    # 添加分析建议
    if [ -f "$OUTPUT_DIR/heap_${TIMESTAMP}.txt" ]; then
        local top_alloc=$(head -5 "$OUTPUT_DIR/heap_${TIMESTAMP}.txt" | grep -oP '\d+\.\d+\s*MB' | head -1)
        if [ -n "$top_alloc" ]; then
            echo "- 最大内存分配: $top_alloc" >> "$report_file"
        fi
    fi

    echo "- 使用 \`go tool pprof -http=:8080 <profile_file>\` 启动交互式分析" >> "$report_file"
    echo "- 使用 \`go tool pprof -list=<function> <profile_file>\` 查看函数详情" >> "$report_file"
}

# 比较两次内存快照
compare_profiles() {
    local profile1="$1"
    local profile2="$2"

    if [ -z "$profile1" ] || [ -z "$profile2" ]; then
        log_error "用法: $0 compare <profile1> <profile2>"
        exit 1
    fi

    log_step "比较内存快照..."

    local output_file="$OUTPUT_DIR/compare_${TIMESTAMP}.txt"

    go tool pprof -text -base="$profile1" "$profile2" > "$output_file" 2>/dev/null || {
        log_error "无法比较 profiles"
        exit 1
    }

    echo ""
    log_info "=== 内存变化 ==="
    cat "$output_file"

    log_info "比较结果已保存到: $output_file"
}

# 运行基准测试并生成内存 profile
benchmark_memory() {
    log_step "运行内存基准测试..."

    local output_file="$OUTPUT_DIR/bench_mem_${TIMESTAMP}.prof"

    # 运行基准测试并生成内存 profile
    go test -bench=. -benchmem -memprofile="$output_file" ./tests/benchmark/... 2>&1 | \
        tee "$OUTPUT_DIR/bench_mem_${TIMESTAMP}.txt"

    echo ""
    log_info "基准测试内存 Profile: $output_file"
}

# 实时监控
monitor_live() {
    local interval="${1:-5}"

    log_step "实时内存监控 (刷新间隔: ${interval}s)..."

    echo "按 Ctrl+C 停止"
    echo ""

    while true; do
        local timestamp=$(date '+%H:%M:%S')

        # 获取内存信息
        local mem_info=$(curl -s "http://localhost:$PORT/debug/pprof/heap?debug=1" 2>/dev/null | head -20)

        # 获取 goroutine 数量
        local goroutine_count=$(curl -s "http://localhost:$PORT/debug/pprof/goroutine?debug=1" 2>/dev/null | grep -c "goroutine" || echo "N/A")

        # 系统内存
        local sys_mem=$(free -h 2>/dev/null | grep "Mem:" | awk '{print $3 "/" $2}')

        echo "[$timestamp] Goroutines: $goroutine_count | 系统内存: $sys_mem"

        sleep "$interval"
    done
}

# 显示帮助
show_help() {
    echo "NAS-OS 内存分析脚本"
    echo ""
    echo "用法: $0 <command> [options]"
    echo ""
    echo "命令:"
    echo "  heap        堆内存分析"
    echo "  goroutine   Goroutine 分析"
    echo "  cpu [秒]    CPU 分析 (默认 30 秒)"
    echo "  allocs      内存分配分析"
    echo "  full        完整内存分析报告"
    echo "  benchmark   运行内存基准测试"
    echo "  monitor     实时内存监控"
    echo "  compare     比较两次内存快照"
    echo ""
    echo "环境变量:"
    echo "  BINARY      服务二进制文件 (默认: ./nasd)"
    echo "  PORT        pprof 端口 (默认: 6060)"
    echo ""
    echo "示例:"
    echo "  $0 heap              # 堆内存分析"
    echo "  $0 cpu 60            # 60 秒 CPU 分析"
    echo "  $0 full              # 完整分析报告"
    echo "  $0 monitor 10        # 每 10 秒刷新监控"
    echo "  $0 compare heap1.prof heap2.prof  # 比较快照"
}

# 主函数
main() {
    local command="${1:-help}"

    case "$command" in
        heap)
            check_pprof
            analyze_heap
            ;;
        goroutine)
            check_pprof
            analyze_goroutine
            ;;
        cpu)
            check_pprof
            analyze_cpu "${2:-30s}"
            ;;
        allocs)
            check_pprof
            analyze_allocs
            ;;
        full)
            check_pprof
            analyze_full
            ;;
        benchmark)
            benchmark_memory
            ;;
        monitor)
            check_pprof
            monitor_live "${2:-5}"
            ;;
        compare)
            compare_profiles "$2" "$3"
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

# 运行
main "$@"