#!/bin/bash
# NAS-OS 数据库迁移脚本
# 支持 SQLite 数据库的版本化迁移
#
# v2.36.0 新增

set -e

# 配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATIONS_DIR="$PROJECT_ROOT/scripts/migrations"
DB_PATH="${DB_PATH:-/var/lib/nas-os/nas-os.db}"
MIGRATION_TABLE="_migrations"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 sqlite3 是否安装
check_sqlite() {
    if ! command -v sqlite3 &> /dev/null; then
        log_error "sqlite3 未安装"
        exit 1
    fi
}

# 确保迁移表存在
ensure_migration_table() {
    sqlite3 "$DB_PATH" "CREATE TABLE IF NOT EXISTS $MIGRATION_TABLE (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        version TEXT NOT NULL UNIQUE,
        name TEXT NOT NULL,
        executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        checksum TEXT
    );"
}

# 获取已执行的迁移
get_executed_migrations() {
    sqlite3 "$DB_PATH" "SELECT version FROM $MIGRATION_TABLE ORDER BY version;"
}

# 获取待执行的迁移
get_pending_migrations() {
    local executed=$(get_executed_migrations)
    for file in "$MIGRATIONS_DIR"/V*.sql; do
        if [ -f "$file" ]; then
            local version=$(basename "$file" | grep -oP '^V\d+')
            if ! echo "$executed" | grep -q "^$version$"; then
                echo "$file"
            fi
        fi
    done
}

# 计算文件校验和
get_checksum() {
    sha256sum "$1" | cut -d' ' -f1
}

# 执行单个迁移
execute_migration() {
    local file="$1"
    local basename=$(basename "$file")
    local version=$(echo "$basename" | grep -oP '^V\d+')
    local name=$(echo "$basename" | sed 's/^V[0-9]*__\(.*\)\.sql$/\1/')
    local checksum=$(get_checksum "$file")

    log_info "执行迁移: $basename"
    
    # 开始事务
    sqlite3 "$DB_PATH" <<EOF
BEGIN TRANSACTION;

-- 执行迁移 SQL
.read $file

-- 记录迁移
INSERT INTO $MIGRATION_TABLE (version, name, checksum) VALUES ('$version', '$name', '$checksum');

COMMIT;
EOF

    if [ $? -eq 0 ]; then
        log_success "迁移完成: $basename"
    else
        log_error "迁移失败: $basename"
        exit 1
    fi
}

# 执行所有待执行的迁移
cmd_up() {
    check_sqlite
    ensure_migration_table
    
    local pending=$(get_pending_migrations)
    if [ -z "$pending" ]; then
        log_info "没有待执行的迁移"
        return 0
    fi
    
    log_info "发现 $(echo "$pending" | wc -l) 个待执行的迁移"
    
    for file in $pending; do
        execute_migration "$file"
    done
    
    log_success "所有迁移执行完成"
}

# 回滚最后一次迁移
cmd_down() {
    check_sqlite
    ensure_migration_table
    
    local last_version=$(sqlite3 "$DB_PATH" "SELECT version FROM $MIGRATION_TABLE ORDER BY version DESC LIMIT 1;")
    
    if [ -z "$last_version" ]; then
        log_info "没有可回滚的迁移"
        return 0
    fi
    
    local rollback_file="$MIGRATIONS_DIR/rollback/${last_version}__rollback.sql"
    
    if [ ! -f "$rollback_file" ]; then
        log_error "找不到回滚脚本: $rollback_file"
        exit 1
    fi
    
    log_info "回滚迁移: $last_version"
    
    sqlite3 "$DB_PATH" <<EOF
BEGIN TRANSACTION;

.read $rollback_file

DELETE FROM $MIGRATION_TABLE WHERE version = '$last_version';

COMMIT;
EOF

    if [ $? -eq 0 ]; then
        log_success "回滚完成"
    else
        log_error "回滚失败"
        exit 1
    fi
}

# 查看迁移状态
cmd_status() {
    check_sqlite
    ensure_migration_table
    
    echo ""
    echo "=== 迁移状态 ==="
    echo "数据库: $DB_PATH"
    echo ""
    
    echo "已执行的迁移:"
    sqlite3 -header -column "$DB_PATH" "SELECT version, name, executed_at FROM $MIGRATION_TABLE ORDER BY version;"
    
    echo ""
    echo "待执行的迁移:"
    local pending=$(get_pending_migrations)
    if [ -z "$pending" ]; then
        echo "  (无)"
    else
        for file in $pending; do
            echo "  - $(basename "$file")"
        done
    fi
    echo ""
}

# 创建新的迁移文件
cmd_create() {
    local description="$1"
    if [ -z "$description" ]; then
        log_error "请提供迁移描述"
        echo "用法: $0 create \"migration_description\""
        exit 1
    fi
    
    # 获取下一个版本号
    local last_version=$(ls "$MIGRATIONS_DIR"/V*.sql 2>/dev/null | grep -oP 'V\d+' | sort -V | tail -1 | grep -oP '\d+' || echo "000")
    local next_version=$(printf "%03d" $((10#$last_version + 1)))
    
    local filename="V${next_version}__${description}.sql"
    local filepath="$MIGRATIONS_DIR/$filename"
    
    cat > "$filepath" <<EOF
-- Migration: $description
-- Version: $next_version
-- Created: $(date -Iseconds)

-- TODO: 在此添加迁移 SQL

EOF
    
    log_success "创建迁移文件: $filepath"
}

# 验证迁移完整性
cmd_verify() {
    check_sqlite
    ensure_migration_table
    
    log_info "验证迁移完整性..."
    
    local errors=0
    
    # 检查每个已执行的迁移是否仍然存在
    for version in $(get_executed_migrations); do
        local file=$(ls "$MIGRATIONS_DIR"/${version}*.sql 2>/dev/null | head -1)
        if [ -z "$file" ]; then
            log_error "迁移文件丢失: $version"
            errors=$((errors + 1))
            continue
        fi
        
        # 验证校验和
        local stored_checksum=$(sqlite3 "$DB_PATH" "SELECT checksum FROM $MIGRATION_TABLE WHERE version = '$version';")
        local current_checksum=$(get_checksum "$file")
        
        if [ "$stored_checksum" != "$current_checksum" ]; then
            log_warn "校验和不匹配: $version (迁移文件可能已被修改)"
        fi
    done
    
    if [ $errors -eq 0 ]; then
        log_success "迁移验证通过"
    else
        log_error "发现 $errors 个错误"
        exit 1
    fi
}

# 重置数据库（危险操作）
cmd_reset() {
    log_warn "即将删除所有数据并重新运行迁移！"
    read -p "确认继续？(yes/no): " confirm
    
    if [ "$confirm" != "yes" ]; then
        log_info "操作已取消"
        exit 0
    fi
    
    # 备份
    local backup_file="${DB_PATH}.backup.$(date +%Y%m%d_%H%M%S)"
    cp "$DB_PATH" "$backup_file"
    log_info "已备份数据库到: $backup_file"
    
    # 删除数据库
    rm -f "$DB_PATH"
    log_info "已删除数据库"
    
    # 重新运行迁移
    cmd_up
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 数据库迁移工具

用法: $0 <command> [args]

命令:
  up       执行所有待执行的迁移
  down     回滚最后一次迁移
  status   查看迁移状态
  create   创建新的迁移文件
  verify   验证迁移完整性
  reset    重置数据库（危险操作）

环境变量:
  DB_PATH  数据库文件路径 (默认: /var/lib/nas-os/nas-os.db)

示例:
  $0 up
  $0 create "add_users_table"
  $0 status
EOF
}

# 主入口
case "${1:-}" in
    up)
        cmd_up
        ;;
    down)
        cmd_down
        ;;
    status)
        cmd_status
        ;;
    create)
        cmd_create "$2"
        ;;
    verify)
        cmd_verify
        ;;
    reset)
        cmd_reset
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac