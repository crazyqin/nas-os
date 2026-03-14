#!/bin/bash
# NAS-OS 数据库备份脚本
# 支持 SQLite 数据库的备份、恢复和清理
#
# v2.38.0 新增

set -e

# 配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DB_PATH="${DB_PATH:-/var/lib/nas-os/nas-os.db}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
RETENTION_COUNT="${RETENTION_COUNT:-10}"

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

# 检查依赖
check_dependencies() {
    if ! command -v sqlite3 &> /dev/null; then
        log_error "sqlite3 未安装"
        exit 1
    fi
    
    if ! command -v gzip &> /dev/null; then
        log_warn "gzip 未安装，备份将不压缩"
        COMPRESS=false
    else
        COMPRESS=true
    fi
}

# 确保备份目录存在
ensure_backup_dir() {
    if [ ! -d "$BACKUP_DIR" ]; then
        mkdir -p "$BACKUP_DIR"
        log_info "创建备份目录: $BACKUP_DIR"
    fi
}

# 检查数据库是否需要备份
check_db_integrity() {
    if [ ! -f "$DB_PATH" ]; then
        log_error "数据库文件不存在: $DB_PATH"
        exit 1
    fi
    
    log_info "检查数据库完整性..."
    if ! sqlite3 "$DB_PATH" "PRAGMA integrity_check;" | grep -q "ok"; then
        log_error "数据库完整性检查失败"
        exit 1
    fi
    log_success "数据库完整性检查通过"
}

# 执行备份
cmd_backup() {
    check_dependencies
    ensure_backup_dir
    check_db_integrity
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_file="${BACKUP_DIR}/nas-os_${timestamp}.db"
    
    log_info "开始备份数据库..."
    log_info "源文件: $DB_PATH"
    log_info "目标文件: $backup_file"
    
    # 使用 SQLite 的在线备份 API
    sqlite3 "$DB_PATH" ".backup '${backup_file}'"
    
    if [ $? -eq 0 ]; then
        # 压缩备份
        if [ "$COMPRESS" = true ]; then
            gzip -f "$backup_file"
            backup_file="${backup_file}.gz"
            log_success "备份已压缩"
        fi
        
        # 计算校验和
        local checksum=$(sha256sum "$backup_file" | cut -d' ' -f1)
        echo "$checksum  $(basename $backup_file)" > "${backup_file}.sha256"
        
        # 获取文件大小
        local size=$(du -h "$backup_file" | cut -f1)
        
        log_success "备份完成: $backup_file"
        log_info "文件大小: $size"
        log_info "校验和: $checksum"
        
        # 清理旧备份
        cmd_cleanup
        
        return 0
    else
        log_error "备份失败"
        return 1
    fi
}

# 恢复数据库
cmd_restore() {
    local backup_file="$1"
    
    if [ -z "$backup_file" ]; then
        log_error "请指定备份文件"
        echo "用法: $0 restore <backup_file>"
        exit 1
    fi
    
    # 查找备份文件
    if [ ! -f "$backup_file" ]; then
        # 尝试在备份目录中查找
        if [ -f "${BACKUP_DIR}/${backup_file}" ]; then
            backup_file="${BACKUP_DIR}/${backup_file}"
        elif [ -f "${BACKUP_DIR}/${backup_file}.gz" ]; then
            backup_file="${BACKUP_DIR}/${backup_file}.gz"
        else
            log_error "备份文件不存在: $backup_file"
            exit 1
        fi
    fi
    
    log_warn "即将从备份恢复数据库"
    log_info "备份文件: $backup_file"
    log_info "目标位置: $DB_PATH"
    
    read -p "确认继续？(yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        log_info "操作已取消"
        exit 0
    fi
    
    # 备份当前数据库
    if [ -f "$DB_PATH" ]; then
        local pre_backup="${DB_PATH}.pre-restore.$(date +%Y%m%d_%H%M%S)"
        log_info "备份当前数据库到: $pre_backup"
        cp "$DB_PATH" "$pre_backup"
    fi
    
    # 解压（如果需要）
    local temp_db="$DB_PATH"
    if [[ "$backup_file" == *.gz ]]; then
        temp_db="/tmp/nas-os-restore-$$.db"
        log_info "解压备份文件..."
        gunzip -c "$backup_file" > "$temp_db"
    fi
    
    # 验证备份完整性
    log_info "验证备份完整性..."
    if ! sqlite3 "$temp_db" "PRAGMA integrity_check;" 2>/dev/null | grep -q "ok"; then
        log_error "备份文件损坏或无效"
        rm -f "$temp_db"
        exit 1
    fi
    
    # 恢复数据库
    log_info "恢复数据库..."
    cp "$temp_db" "$DB_PATH"
    
    # 清理临时文件
    if [ "$temp_db" != "$DB_PATH" ]; then
        rm -f "$temp_db"
    fi
    
    log_success "数据库恢复完成"
}

# 列出备份
cmd_list() {
    ensure_backup_dir
    
    echo ""
    echo "=== 数据库备份列表 ==="
    echo "备份目录: $BACKUP_DIR"
    echo ""
    
    if [ -z "$(ls -A $BACKUP_DIR 2>/dev/null)" ]; then
        echo "  (无备份文件)"
        return 0
    fi
    
    printf "%-30s %-12s %-20s\n" "文件名" "大小" "创建时间"
    printf "%-30s %-12s %-20s\n" "------" "----" "--------"
    
    for file in $(ls -t ${BACKUP_DIR}/*.db* 2>/dev/null | grep -v '\.sha256$'); do
        local name=$(basename $file)
        local size=$(du -h "$file" | cut -f1)
        local mtime=$(stat -c %y "$file" 2>/dev/null | cut -d'.' -f1 || stat -f "%Sm" "$file")
        printf "%-30s %-12s %-20s\n" "$name" "$size" "$mtime"
    done
    
    echo ""
    local count=$(ls ${BACKUP_DIR}/*.db* 2>/dev/null | grep -v '\.sha256$' | wc -l)
    echo "总计: $count 个备份"
    echo ""
}

# 清理旧备份
cmd_cleanup() {
    ensure_backup_dir
    
    log_info "清理旧备份..."
    log_info "保留策略: 最近 ${RETENTION_COUNT} 个备份，或 ${RETENTION_DAYS} 天内的备份"
    
    local deleted=0
    local files=($(ls -t ${BACKUP_DIR}/nas-os_*.db* 2>/dev/null | grep -v '\.sha256$'))
    local total=${#files[@]}
    
    if [ $total -le $RETENTION_COUNT ]; then
        log_info "备份数量 ($total) 未超过保留数量 ($RETENTION_COUNT)，无需清理"
        return 0
    fi
    
    # 删除超过保留数量的旧备份
    local to_delete=$((total - RETENTION_COUNT))
    log_info "将删除 $to_delete 个旧备份"
    
    for ((i=RETENTION_COUNT; i<total; i++)); do
        local file="${files[$i]}"
        local sha_file="${file}.sha256"
        
        rm -f "$file" "$sha_file"
        log_info "已删除: $(basename $file)"
        deleted=$((deleted + 1))
    done
    
    log_success "清理完成，删除了 $deleted 个旧备份"
}

# 验证备份
cmd_verify() {
    local backup_file="$1"
    
    if [ -z "$backup_file" ]; then
        # 验证所有备份
        ensure_backup_dir
        log_info "验证所有备份..."
        
        local errors=0
        for file in ${BACKUP_DIR}/nas-os_*.db*; do
            if [[ "$file" != *.sha256 ]] && [ -f "$file" ]; then
                if ! verify_backup "$file"; then
                    errors=$((errors + 1))
                fi
            fi
        done
        
        if [ $errors -eq 0 ]; then
            log_success "所有备份验证通过"
        else
            log_error "发现 $errors 个备份验证失败"
            exit 1
        fi
        return 0
    fi
    
    # 验证单个备份
    verify_backup "$backup_file"
}

verify_backup() {
    local file="$1"
    
    if [ ! -f "$file" ]; then
        log_error "备份文件不存在: $file"
        return 1
    fi
    
    log_info "验证: $(basename $file)"
    
    # 验证校验和
    local sha_file="${file}.sha256"
    if [ -f "$sha_file" ]; then
        if sha256sum -c "$sha_file" --quiet 2>/dev/null; then
            log_success "校验和验证通过"
        else
            log_error "校验和验证失败"
            return 1
        fi
    fi
    
    # 验证数据库完整性
    local temp_file="$file"
    if [[ "$file" == *.gz ]]; then
        temp_file="/tmp/nas-os-verify-$$.db"
        gunzip -c "$file" > "$temp_file"
    fi
    
    if sqlite3 "$temp_file" "PRAGMA integrity_check;" 2>/dev/null | grep -q "ok"; then
        log_success "数据库完整性验证通过"
    else
        log_error "数据库完整性验证失败"
        [ "$temp_file" != "$file" ] && rm -f "$temp_file"
        return 1
    fi
    
    # 清理临时文件
    [ "$temp_file" != "$file" ] && rm -f "$temp_file"
    return 0
}

# 自动备份（用于 cron）
cmd_auto() {
    log_info "执行自动备份..."
    
    # 检查是否需要备份
    local last_backup=$(ls -t ${BACKUP_DIR}/nas-os_*.db* 2>/dev/null | head -1)
    if [ -n "$last_backup" ]; then
        local last_mtime=$(stat -c %Y "$last_backup" 2>/dev/null || stat -f %m "$last_backup")
        local now=$(date +%s)
        local age_hours=$(( (now - last_mtime) / 3600 ))
        
        if [ $age_hours -lt 24 ]; then
            log_info "上次备份在 $age_hours 小时前，跳过本次备份"
            return 0
        fi
    fi
    
    cmd_backup
}

# 帮助信息
show_help() {
    cat <<EOF
NAS-OS 数据库备份工具

用法: $0 <command> [args]

命令:
  backup       创建数据库备份
  restore      从备份恢复数据库
  list         列出所有备份
  cleanup      清理旧备份
  verify       验证备份完整性
  auto         自动备份（用于 cron）

环境变量:
  DB_PATH          数据库文件路径 (默认: /var/lib/nas-os/nas-os.db)
  BACKUP_DIR       备份目录 (默认: /var/lib/nas-os/backups)
  RETENTION_DAYS   保留天数 (默认: 30)
  RETENTION_COUNT  保留数量 (默认: 10)

示例:
  $0 backup
  $0 restore nas-os_20260315_120000.db.gz
  $0 list
  $0 verify

Cron 示例 (每天凌晨 2 点自动备份):
  0 2 * * * /path/to/db-backup.sh auto >> /var/log/nas-os/backup.log 2>&1
EOF
}

# 主入口
case "${1:-}" in
    backup)
        cmd_backup
        ;;
    restore)
        cmd_restore "$2"
        ;;
    list)
        cmd_list
        ;;
    cleanup)
        cmd_cleanup
        ;;
    verify)
        cmd_verify "$2"
        ;;
    auto)
        cmd_auto
        ;;
    -h|--help|help)
        show_help
        ;;
    *)
        show_help
        exit 1
        ;;
esac