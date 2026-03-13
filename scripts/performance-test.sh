#!/bin/bash
# NAS-OS 性能测试脚本
# 用法: ./scripts/performance-test.sh [API_BASE_URL]

set -e

# 配置
API_BASE_URL="${1:-http://localhost:8080/api/v1}"
RESULTS_DIR="reports/performance"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$RESULTS_DIR/perf_test_$TIMESTAMP.md"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 创建结果目录
mkdir -p "$RESULTS_DIR"

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

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."

    local missing=0

    if ! command -v curl &> /dev/null; then
        log_error "curl 未安装"
        missing=1
    fi

    if ! command -v jq &> /dev/null; then
        log_error "jq 未安装"
        missing=1
    fi

    if [ $missing -eq 1 ]; then
        exit 1
    fi

    log_info "依赖检查通过"
}

# 测试 API 响应时间
test_api_response_time() {
    local endpoint="$1"
    local method="$2"
    local data="$3"
    local iterations="${4:-10}"

    log_info "测试 $method $endpoint ($iterations 次)"

    local total_time=0
    local min_time=999999
    local max_time=0
    local success=0
    local failed=0

    for i in $(seq 1 $iterations); do
        local start_time=$(date +%s%N)

        if [ "$method" == "GET" ]; then
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_BASE_URL$endpoint")
        else
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" \
                -H "Content-Type: application/json" \
                -d "$data" "$API_BASE_URL$endpoint")
        fi

        local end_time=$(date +%s%N)
        local duration=$(( (end_time - start_time) / 1000000 ))

        if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 400 ]; then
            success=$((success + 1))
            total_time=$((total_time + duration))

            if [ $duration -lt $min_time ]; then
                min_time=$duration
            fi
            if [ $duration -gt $max_time ]; then
                max_time=$duration
            fi
        else
            failed=$((failed + 1))
            log_warn "请求失败: HTTP $http_code"
        fi
    done

    if [ $success -gt 0 ]; then
        local avg_time=$((total_time / success))
        echo "  成功: $success, 失败: $failed"
        echo "  平均响应时间: ${avg_time}ms"
        echo "  最小: ${min_time}ms, 最大: ${max_time}ms"

        # 返回结果
        echo "$endpoint|$method|$avg_time|$min_time|$max_time|$success|$failed" >> /tmp/perf_results.txt
    else
        log_error "所有请求都失败了"
    fi
}

# 测试并发性能
test_concurrent_requests() {
    local endpoint="$1"
    local concurrency="$2"
    local total_requests="$3"

    log_info "并发测试: $concurrency 并发, 共 $total_requests 请求"

    local start_time=$(date +%s.%N)

    # 使用 xargs 并行执行
    seq 1 $total_requests | xargs -P $concurrency -I {} curl -s -o /dev/null "$API_BASE_URL$endpoint" &

    wait

    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local rps=$(echo "scale=2; $total_requests / $duration" | bc)

    echo "  总耗时: ${duration}s"
    echo "  请求/秒: $rps"

    echo "concurrent|$concurrency|$total_requests|$duration|$rps" >> /tmp/perf_results.txt
}

# 测试文件列表性能
test_file_list() {
    log_info "测试文件列表 API..."

    # 测试不同目录大小
    local paths=("/" "/home" "/var")

    for path in "${paths[@]}"; do
        log_info "测试路径: $path"
        test_api_response_time "/files/list?path=$path" "GET" "" 5
    done
}

# 测试搜索性能
test_search() {
    log_info "测试搜索 API..."

    local queries=("test" "config" "*.json" "README")

    for query in "${queries[@]}"; do
        log_info "搜索查询: $query"
        test_api_response_time "/search/query" "POST" "{\"query\":\"$query\"}" 5
    done
}

# 测试上传性能
test_upload() {
    log_info "测试上传性能..."

    local file_sizes=("1KB" "10KB" "100KB" "1MB" "10MB")

    for size in "${file_sizes[@]}"; do
        local bytes
        case $size in
            "1KB") bytes=1024 ;;
            "10KB") bytes=10240 ;;
            "100KB") bytes=102400 ;;
            "1MB") bytes=1048576 ;;
            "10MB") bytes=10485760 ;;
        esac

        # 创建测试文件
        local test_file="/tmp/test_upload_${size}.bin"
        dd if=/dev/zero of=$test_file bs=$bytes count=1 2>/dev/null

        log_info "上传 ${size} 文件..."

        local start_time=$(date +%s%N)
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" \
            -X POST \
            -F "file=@$test_file" \
            "$API_BASE_URL/files/upload?path=/tmp")

        local end_time=$(date +%s%N)
        local duration=$(( (end_time - start_time) / 1000000 ))

        echo "  上传 ${size}: ${duration}ms (HTTP $http_code)"

        # 清理
        rm -f $test_file
    done
}

# 测试下载性能
test_download() {
    log_info "测试下载性能..."

    local test_file="/tmp/test_download.bin"

    # 创建测试文件 (10MB)
    dd if=/dev/zero of=$test_file bs=1M count=10 2>/dev/null

    # 通过 API 下载 (假设 API 支持下载特定文件)
    log_info "下载 10MB 文件..."

    local start_time=$(date +%s%N)
    curl -s -o /dev/null "$API_BASE_URL/files/download?path=$test_file"
    local end_time=$(date +%s%N)

    local duration=$(( (end_time - start_time) / 1000000 ))
    local throughput=$(echo "scale=2; 10 * 1000 / $duration" | bc)

    echo "  下载耗时: ${duration}ms"
    echo "  吞吐量: ${throughput} MB/s"

    rm -f $test_file
}

# 测试缓存效果
test_cache_effectiveness() {
    log_info "测试缓存效果..."

    # 第一次请求 (缓存未命中)
    log_info "第一次请求 (缓存未命中)..."
    test_api_response_time "/files/list?path=/" "GET" "" 1

    # 等待一下
    sleep 1

    # 第二次请求 (缓存命中)
    log_info "第二次请求 (缓存命中)..."
    test_api_response_time "/files/list?path=/" "GET" "" 1

    # 获取缓存统计
    log_info "获取缓存统计..."
    curl -s "$API_BASE_URL/performance/metrics" | jq '.data.cacheHitRate' || echo "无法获取缓存统计"
}

# 测试数据库性能
test_database_performance() {
    log_info "测试数据库性能..."

    # 执行一些数据库操作
    test_api_response_time "/users" "GET" "" 10
    test_api_response_time "/shares" "GET" "" 10
}

# 生成报告
generate_report() {
    log_info "生成性能测试报告..."

    cat > "$REPORT_FILE" << EOF
# NAS-OS 性能测试报告

**测试时间:** $(date '+%Y-%m-%d %H:%M:%S')
**API 地址:** $API_BASE_URL

## 测试结果

| 端点 | 方法 | 平均响应时间(ms) | 最小(ms) | 最大(ms) | 成功 | 失败 |
|------|------|-----------------|----------|----------|------|------|
EOF

    if [ -f /tmp/perf_results.txt ]; then
        while IFS='|' read -r endpoint method avg min max success failed; do
            echo "| $endpoint | $method | $avg | $min | $max | $success | $failed |" >> "$REPORT_FILE"
        done < /tmp/perf_results.txt
    fi

    cat >> "$REPORT_FILE" << EOF

## 系统资源

\`\`\`
$(curl -s "$API_BASE_URL/performance/metrics" | jq '.' 2>/dev/null || echo "无法获取系统指标")
\`\`\`

## 建议

EOF

    # 添加性能建议
    if [ -f /tmp/perf_results.txt ]; then
        local avg_response=$(awk -F'|' '{sum+=$3; count++} END {print int(sum/count)}' /tmp/perf_results.txt)
        if [ $avg_response -gt 500 ]; then
            echo "- **警告:** 平均响应时间较高 (${avg_response}ms)，建议检查:" >> "$REPORT_FILE"
            echo "  - 数据库索引优化" >> "$REPORT_FILE"
            echo "  - 缓存策略调整" >> "$REPORT_FILE"
            echo "  - 服务器资源使用" >> "$REPORT_FILE"
        elif [ $avg_response -gt 200 ]; then
            echo "- **注意:** 平均响应时间一般 (${avg_response}ms)，可考虑优化" >> "$REPORT_FILE"
        else
            echo "- **良好:** 平均响应时间正常 (${avg_response}ms)" >> "$REPORT_FILE"
        fi
    fi

    log_info "报告已保存到: $REPORT_FILE"

    # 清理
    rm -f /tmp/perf_results.txt
}

# 健康检查
health_check() {
    log_info "执行健康检查..."

    local response=$(curl -s "$API_BASE_URL/performance/health" 2>/dev/null)

    if [ $? -eq 0 ]; then
        echo "$response" | jq '.' || echo "$response"
    else
        log_error "健康检查失败，API 可能未运行"
        exit 1
    fi
}

# 主函数
main() {
    echo "=========================================="
    echo "     NAS-OS 性能测试"
    echo "=========================================="
    echo ""

    check_dependencies
    health_check

    # 清理之前的结果
    rm -f /tmp/perf_results.txt

    # 执行测试
    echo ""
    log_info "开始性能测试..."
    echo ""

    test_file_list
    echo ""

    test_search
    echo ""

    # test_upload  # 需要认证，跳过
    # test_download # 需要特定文件，跳过

    test_cache_effectiveness
    echo ""

    test_database_performance
    echo ""

    # 并发测试 (可选)
    # test_concurrent_requests "/files/list?path=/" 10 100

    # 生成报告
    generate_report

    echo ""
    log_info "性能测试完成!"
}

# 运行
main "$@"