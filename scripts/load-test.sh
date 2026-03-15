#!/bin/bash
# NAS-OS 压力测试脚本

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULT_DIR="/home/mrafter/clawd/logs/load-test"
mkdir -p "$RESULT_DIR"

echo "============================================"
echo "NAS-OS 压力测试"
echo "============================================"
echo "目标 URL: $BASE_URL"
echo "结果目录：$RESULT_DIR"
echo ""

# 检查 hey 是否可用
HEY_BIN="$HOME/go/bin/hey"
if [ ! -f "$HEY_BIN" ]; then
    echo "❌ hey 工具未找到，请先安装：go install github.com/rakyll/hey@latest"
    exit 1
fi

echo "✅ hey 工具就绪：$HEY_BIN"
echo ""

# 测试 1: 健康检查端点 (高并发)
echo "📊 测试 1: 健康检查端点 (/api/v1/system/health)"
echo "并发数：100, 请求数：1000"
$HEY_BIN -c 100 -n 1000 "$BASE_URL/api/v1/system/health" > "$RESULT_DIR/health-100c-1000n.txt" 2>&1
echo "✅ 完成 - 结果保存至：$RESULT_DIR/health-100c-1000n.txt"
echo ""

# 测试 2: 系统信息端点 (中等并发)
echo "📊 测试 2: 系统信息端点 (/api/v1/system/info)"
echo "并发数：50, 请求数：500"
$HEY_BIN -c 50 -n 500 "$BASE_URL/api/v1/system/info" > "$RESULT_DIR/info-50c-500n.txt" 2>&1
echo "✅ 完成 - 结果保存至：$RESULT_DIR/info-50c-500n.txt"
echo ""

# 测试 3: 卷列表端点 (低并发)
echo "📊 测试 3: 卷列表端点 (/api/v1/volumes)"
echo "并发数：20, 请求数：200"
$HEY_BIN -c 20 -n 200 "$BASE_URL/api/v1/volumes" > "$RESULT_DIR/volumes-20c-200n.txt" 2>&1
echo "✅ 完成 - 结果保存至：$RESULT_DIR/volumes-20c-200n.txt"
echo ""

# 测试 4: 压力测试 (持续 30 秒)
echo "📊 测试 4: 持续压力测试 (30 秒)"
echo "并发数：50, 持续时间：30s"
$HEY_BIN -c 50 -z 30s "$BASE_URL/api/v1/system/health" > "$RESULT_DIR/stress-50c-30s.txt" 2>&1
echo "✅ 完成 - 结果保存至：$RESULT_DIR/stress-50c-30s.txt"
echo ""

# 测试 5: 大文件上传测试 (如果有测试文件)
TEST_FILE="/tmp/test-upload-100mb.bin"
if [ -f "$TEST_FILE" ]; then
    echo "📊 测试 5: 大文件上传测试 (100MB)"
    curl -s -w "\n上传时间：%{time_total}s\n上传速度：%{speed_upload} bytes/s\nHTTP 状态：%{http_code}\n" \
        -X POST "$BASE_URL/api/v1/test/upload" \
        -H "Content-Type: application/octet-stream" \
        --data-binary @"$TEST_FILE" > "$RESULT_DIR/upload-100mb.txt" 2>&1
    echo "✅ 完成 - 结果保存至：$RESULT_DIR/upload-100mb.txt"
else
    echo "⏭️  跳过上传测试 (测试文件不存在)"
fi
echo ""

# 测试 6: 大文件下载测试
echo "📊 测试 6: 大文件下载测试"
echo "⚠️  需要实际文件端点，暂跳过"
echo ""

echo "============================================"
echo "🎉 压力测试完成!"
echo "============================================"
echo ""
echo "结果文件列表:"
ls -la "$RESULT_DIR"/*.txt 2>/dev/null || echo "无结果文件"
echo ""
echo "查看结果示例:"
echo "  cat $RESULT_DIR/health-100c-1000n.txt"
