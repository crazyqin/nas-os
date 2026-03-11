#!/bin/bash
# NAS-OS 测试运行脚本
# 支持：单元测试、集成测试、E2E 测试、性能基准测试

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPORT_DIR="${PROJECT_ROOT}/tests/reports/output"
COVERAGE_FILE="${REPORT_DIR}/coverage.out"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# 帮助信息
show_help() {
    echo "NAS-OS 测试套件 v1.0"
    echo ""
    echo "用法: $0 [命令] [选项]"
    echo ""
    echo "命令:"
    echo "  unit          运行单元测试"
    echo "  integration   运行集成测试"
    echo "  e2e           运行端到端测试"
    echo "  benchmark     运行性能基准测试"
    echo "  all           运行所有测试"
    echo "  coverage      生成覆盖率报告"
    echo "  report        生成测试报告"
    echo "  clean         清理测试产物"
    echo "  help          显示帮助信息"
    echo ""
    echo "选项:"
    echo "  -v, --verbose    详细输出"
    echo "  -r, --race       启用竞态检测"
    echo "  -c, --cover      启用覆盖率"
    echo "  -p, --parallel   并行测试数"
    echo "  -o, --output     报告输出目录"
    echo ""
    echo "示例:"
    echo "  $0 unit                    # 运行单元测试"
    echo "  $0 all -v -r               # 运行所有测试，详细输出，竞态检测"
    echo "  $0 coverage                # 生成覆盖率报告"
    echo "  $0 benchmark               # 运行性能测试"
}

# 初始化
init() {
    echo -e "${BLUE}🔧 初始化测试环境...${NC}"
    mkdir -p "${REPORT_DIR}"
    cd "${PROJECT_ROOT}"
}

# 运行单元测试
run_unit_tests() {
    echo -e "${BLUE}🧪 运行单元测试...${NC}"
    
    local args="-v"
    if [ "$RACE" = "true" ]; then
        args="$args -race"
    fi
    if [ "$COVER" = "true" ]; then
        args="$args -coverprofile=${COVERAGE_FILE}"
    fi
    
    go test $args ./internal/... ./pkg/... 2>&1 | tee "${REPORT_DIR}/unit_${TIMESTAMP}.log"
    
    echo -e "${GREEN}✅ 单元测试完成${NC}"
}

# 运行集成测试
run_integration_tests() {
    echo -e "${BLUE}🔗 运行集成测试...${NC}"
    
    local args="-v"
    if [ "$RACE" = "true" ]; then
        args="$args -race"
    fi
    
    go test $args ./tests/integration/... 2>&1 | tee "${REPORT_DIR}/integration_${TIMESTAMP}.log"
    
    echo -e "${GREEN}✅ 集成测试完成${NC}"
}

# 运行 E2E 测试
run_e2e_tests() {
    echo -e "${BLUE}🚀 运行端到端测试...${NC}"
    
    export NAS_OS_E2E=1
    
    local args="-v"
    if [ "$RACE" = "true" ]; then
        args="$args -race"
    fi
    
    go test $args ./tests/e2e/... 2>&1 | tee "${REPORT_DIR}/e2e_${TIMESTAMP}.log"
    
    echo -e "${GREEN}✅ E2E 测试完成${NC}"
}

# 运行性能基准测试
run_benchmark_tests() {
    echo -e "${BLUE}⚡ 运行性能基准测试...${NC}"
    
    go test -bench=. -benchmem -run=^$ ./tests/benchmark/... 2>&1 | tee "${REPORT_DIR}/benchmark_${TIMESTAMP}.log"
    
    echo -e "${GREEN}✅ 性能基准测试完成${NC}"
}

# 运行所有测试
run_all_tests() {
    echo -e "${BLUE}🎯 运行所有测试...${NC}"
    
    run_unit_tests
    run_integration_tests
    run_e2e_tests
    run_benchmark_tests
    
    echo -e "${GREEN}✅ 所有测试完成${NC}"
}

# 生成覆盖率报告
generate_coverage() {
    echo -e "${BLUE}📊 生成覆盖率报告...${NC}"
    
    go test -v -coverprofile=${COVERAGE_FILE} ./...
    
    # 生成 HTML 报告
    go tool cover -html=${COVERAGE_FILE} -o "${REPORT_DIR}/coverage.html"
    
    # 输出覆盖率统计
    echo ""
    echo -e "${YELLOW}覆盖率统计:${NC}"
    go tool cover -func=${COVERAGE_FILE} | tail -1
    
    echo ""
    echo -e "${GREEN}📄 覆盖率报告已生成:${NC}"
    echo "   HTML: ${REPORT_DIR}/coverage.html"
}

# 生成测试报告
generate_report() {
    echo -e "${BLUE}📋 生成测试报告...${NC}"
    
    # 运行测试并收集结果
    go test -v -json ./... > "${REPORT_DIR}/test_results.json" 2>&1 || true
    
    # 调用报告生成器
    go run ./tests/reports/generate.go -output "${REPORT_DIR}"
    
    echo -e "${GREEN}📄 测试报告已生成:${NC}"
    echo "   ${REPORT_DIR}/test-report.html"
    echo "   ${REPORT_DIR}/test-report.md"
    echo "   ${REPORT_DIR}/test-report.json"
}

# 清理测试产物
clean() {
    echo -e "${YELLOW}🧹 清理测试产物...${NC}"
    
    rm -rf "${REPORT_DIR}"
    rm -f coverage.out coverage.html
    rm -f "${PROJECT_ROOT}"/*.test
    rm -f "${PROJECT_ROOT}"/tests/**/*.log
    
    echo -e "${GREEN}✅ 清理完成${NC}"
}

# 解析参数
VERBOSE=false
RACE=false
COVER=false
PARALLEL=4
OUTPUT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        -c|--cover)
            COVER=true
            shift
            ;;
        -p|--parallel)
            PARALLEL="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT="$2"
            REPORT_DIR="$2"
            shift 2
            ;;
        help|--help|-h)
            show_help
            exit 0
            ;;
        *)
            COMMAND="$1"
            shift
            ;;
    esac
done

# 执行命令
init

case "${COMMAND}" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    e2e)
        run_e2e_tests
        ;;
    benchmark)
        run_benchmark_tests
        ;;
    all)
        run_all_tests
        ;;
    coverage)
        generate_coverage
        ;;
    report)
        generate_report
        ;;
    clean)
        clean
        ;;
    "")
        show_help
        ;;
    *)
        echo -e "${RED}未知命令: ${COMMAND}${NC}"
        show_help
        exit 1
        ;;
esac