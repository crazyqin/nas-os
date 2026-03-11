#!/bin/bash
# 下载中心功能测试脚本

set -e

API_BASE="http://localhost:8080/api/v1/downloader"

echo "======================================"
echo "  下载中心功能测试"
echo "======================================"
echo ""

# 测试 1: 获取统计信息
echo "📊 测试 1: 获取统计信息"
curl -s "$API_BASE/stats" | jq .
echo ""

# 测试 2: 创建 HTTP 下载任务
echo "📥 测试 2: 创建 HTTP 下载任务"
curl -s -X POST "$API_BASE/tasks" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://speed.hetzner.de/100MB.bin",
    "name": "测试下载 100MB",
    "type": "http"
  }' | jq .
echo ""

# 测试 3: 创建磁力链接任务
echo "🧲 测试 3: 创建磁力链接任务"
curl -s -X POST "$API_BASE/tasks" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "magnet:?xt=urn:btih:08ada5a7a6183aae1e09d831df6748d566095a10&dn=Sintel",
    "name": "Sintel 测试视频",
    "type": "magnet"
  }' | jq .
echo ""

# 测试 4: 列出所有任务
echo "📋 测试 4: 列出所有任务"
curl -s "$API_BASE/tasks" | jq .
echo ""

sleep 2

# 测试 5: 获取任务详情
echo "🔍 测试 5: 获取任务详情"
TASK_ID=$(curl -s "$API_BASE/tasks" | jq -r '.data[0].id')
if [ "$TASK_ID" != "null" ]; then
  curl -s "$API_BASE/tasks/$TASK_ID" | jq .
  echo ""
  
  # 测试 6: 暂停任务
  echo "⏸️  测试 6: 暂停任务"
  curl -s -X POST "$API_BASE/tasks/$TASK_ID/pause" | jq .
  echo ""
  
  sleep 1
  
  # 测试 7: 恢复任务
  echo "▶️  测试 7: 恢复任务"
  curl -s -X POST "$API_BASE/tasks/$TASK_ID/resume" | jq .
  echo ""
  
  # 测试 8: 更新限速
  echo "🐢 测试 8: 设置限速"
  curl -s -X PUT "$API_BASE/tasks/$TASK_ID" \
    -H "Content-Type: application/json" \
    -d '{
      "speed_limit": {
        "download_limit": 512,
        "upload_limit": 128,
        "enabled": true
      }
    }' | jq .
  echo ""
else
  echo "⚠️  没有任务可测试"
fi

echo ""
echo "======================================"
echo "  测试完成！"
echo "======================================"
echo ""
echo "Web UI 访问：http://localhost:8080/downloader"
echo "API 文档：http://localhost:8080/swagger/index.html"
