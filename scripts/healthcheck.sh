#!/bin/sh
# NAS-OS 健康检查脚本
# 用于 Docker 健康检查和 Kubernetes 探针
#
# v2.35.0 新增

set -e

# 配置
HEALTH_ENDPOINT="${HEALTH_ENDPOINT:-http://localhost:8080/api/v1/health}"
METRICS_ENDPOINT="${METRICS_ENDPOINT:-http://localhost:8080/api/v1/metrics}"
TIMEOUT="${TIMEOUT:-5}"

# 检查进程
check_process() {
    if pgrep -x nasd > /dev/null 2>&1; then
        return 0
    fi
    if [ -f /var/run/nasd.pid ]; then
        pid=$(cat /var/run/nasd.pid)
        if kill -0 "$pid" 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

# 检查 HTTP 端点
check_http() {
    url="$1"
    if curl -sf --max-time "$TIMEOUT" "$url" > /dev/null 2>&1; then
        return 0
    fi
    return 1
}

# 主健康检查
main() {
    errors=0

    # 1. 进程检查
    if ! check_process; then
        echo "ERROR: nasd process not running"
        errors=$((errors + 1))
    fi

    # 2. 健康端点检查
    if ! check_http "$HEALTH_ENDPOINT"; then
        echo "ERROR: health endpoint not responding"
        errors=$((errors + 1))
    fi

    # 3. 指标端点检查（可选，用于 Prometheus）
    if ! check_http "$METRICS_ENDPOINT"; then
        echo "WARN: metrics endpoint not responding"
        # 不计入错误，metrics 是可选的
    fi

    if [ $errors -gt 0 ]; then
        echo "Health check failed with $errors error(s)"
        exit 1
    fi

    echo "Health check passed"
    exit 0
}

# 就绪检查（用于 Kubernetes readinessProbe）
ready() {
    # 检查服务是否完全启动
    if ! check_http "$HEALTH_ENDPOINT"; then
        echo "Service not ready"
        exit 1
    fi

    # 检查健康状态是否为 healthy
    status=$(curl -sf --max-time "$TIMEOUT" "$HEALTH_ENDPOINT" 2>/dev/null | \
             grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
    
    if [ "$status" = "healthy" ]; then
        echo "Service ready"
        exit 0
    elif [ "$status" = "degraded" ]; then
        echo "Service degraded but ready"
        exit 0
    else
        echo "Service not ready: status=$status"
        exit 1
    fi
}

# 存活检查（用于 Kubernetes livenessProbe）
live() {
    if check_process; then
        echo "Service alive"
        exit 0
    fi
    echo "Service not alive"
    exit 1
}

# 使用方式
case "$1" in
    ready)
        ready
        ;;
    live)
        live
        ;;
    *)
        main
        ;;
esac