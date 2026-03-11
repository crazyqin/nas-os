#!/bin/bash
# 备份增强功能测试脚本

set -e

echo "======================================"
echo "OpenClaw 备份增强功能测试"
echo "======================================"
echo ""

# 测试目录
TEST_DIR="/tmp/backup-test-$$"
SOURCE_DIR="$TEST_DIR/source"
BACKUP_DIR="$TEST_DIR/backups"

# 清理函数
cleanup() {
    echo ""
    echo "清理测试目录..."
    rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# 创建测试环境
echo "1. 创建测试环境..."
mkdir -p "$SOURCE_DIR/data"
mkdir -p "$BACKUP_DIR"

# 创建测试文件
for i in {1..10}; do
    dd if=/dev/urandom of="$SOURCE_DIR/data/file$i.dat" bs=1K count=100 2>/dev/null
done
echo "   ✅ 创建 10 个测试文件 (每个 100KB)"

# 创建一些文档
echo "测试文档" > "$SOURCE_DIR/data/readme.txt"
echo "配置文件" > "$SOURCE_DIR/data/config.yaml"

echo ""
echo "2. 测试增量备份..."

# 第一次备份（完整）
echo "   第一次备份（完整）..."
cd /home/mrafter/clawd/nas-os
go run cmd/backup/main.go incremental create \
    --source "$SOURCE_DIR" \
    --name testdata \
    --dest "$BACKUP_DIR"

# 修改一些文件
sleep 1
echo "修改内容" >> "$SOURCE_DIR/data/file1.dat"
echo "新文件" > "$SOURCE_DIR/data/newfile.txt"

# 第二次备份（增量）
echo "   第二次备份（增量）..."
go run cmd/backup/main.go incremental create \
    --source "$SOURCE_DIR" \
    --name testdata \
    --dest "$BACKUP_DIR"

# 列出备份
echo ""
echo "3. 列出备份..."
go run cmd/backup/main.go incremental list \
    --name testdata \
    --dest "$BACKUP_DIR"

echo ""
echo "4. 测试加密..."
# 找到最新备份
LATEST=$(readlink "$BACKUP_DIR/testdata/latest")
echo "   最新备份：$LATEST"

# 创建测试密码文件
echo "test-password-123" > "$TEST_DIR/.password"

# 加密
echo "   加密备份..."
go run cmd/backup/main.go encrypt \
    --input "$BACKUP_DIR/testdata/$LATEST.tar.gz" \
    --output "$BACKUP_DIR/testdata/$LATEST.tar.gz.enc" \
    --password-file "$TEST_DIR/.password"

echo "   ✅ 加密完成"

# 验证校验和
echo ""
echo "5. 验证校验和..."
go run cmd/backup/main.go verify \
    --file "$BACKUP_DIR/testdata/$LATEST.tar.gz.enc"

echo ""
echo "6. 测试解密..."
go run cmd/backup/main.go decrypt \
    --input "$BACKUP_DIR/testdata/$LATEST.tar.gz.enc" \
    --output "$TEST_DIR/decrypted.tar.gz" \
    --password-file "$TEST_DIR/.password"

echo "   ✅ 解密完成"

echo ""
echo "7. 测试恢复..."
RESTORE_DIR="$TEST_DIR/restored"

# 预览恢复
echo "   预览恢复..."
go run cmd/backup/main.go restore preview \
    --backup "testdata/$LATEST" \
    --dest "$BACKUP_DIR"

# 实际恢复
echo "   执行恢复..."
go run cmd/backup/main.go restore full \
    --backup "testdata/$LATEST" \
    --target "$RESTORE_DIR" \
    --dest "$BACKUP_DIR" \
    --overwrite \
    --verify

echo ""
echo "8. 验证恢复结果..."
ORIGINAL_COUNT=$(find "$SOURCE_DIR" -type f | wc -l)
RESTORED_COUNT=$(find "$RESTORE_DIR" -type f | wc -l)

echo "   原始文件数：$ORIGINAL_COUNT"
echo "   恢复文件数：$RESTORED_COUNT"

if [ "$ORIGINAL_COUNT" -eq "$RESTORED_COUNT" ]; then
    echo "   ✅ 文件数量匹配"
else
    echo "   ❌ 文件数量不匹配"
    exit 1
fi

echo ""
echo "9. 健康检查..."
go run cmd/backup/main.go health

echo ""
echo "======================================"
echo "✅ 所有测试通过!"
echo "======================================"
