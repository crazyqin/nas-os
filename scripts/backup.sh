#!/bin/bash
# NAS-OS 系统备份脚本
# 支持完整备份、增量备份，多种备份目标
#
# v2.50.0 工部开发
# - 支持完整备份和增量备份
# - 支持多种备份目标（本地/S3/SFTP）
# - 备份完整性校验
# - 备份日志记录
# - 备份告警通知
# - 支持配置文件

set -e

# 脚本信息
SCRIPT_VERSION="2.50.0"
SCRIPT_NAME="NAS-OS Backup"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认配置
BACKUP_NAME="${BACKUP_NAME:-nas-os}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/nas-os/backups}"
DATA_DIR="${DATA_DIR:-/var/lib/nas-os}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nas-os}"
LOG_DIR="${LOG_DIR:-/var/log/nas-os}"

# 备份类型
BACKUP_TYPE="${BACKUP_TYPE:-full}"  # full | incremental

# 保留策略
RETENTION_DAYS="${RETENTION_DAYS:-30}"
RETENTION_COUNT="${RETENTION_COUNT:-10}"

# 压缩配置
ENABLE_COMPRESS="${ENABLE_COMPRESS:-true}"
COMPRESS_LEVEL="${COMPRESS_LEVEL:-6}"

# 加密配置
ENABLE_ENCRYPTION="${ENABLE_ENCRYPTION:-false}"
ENCRYPTION_KEY_FILE="${ENCRYPTION_KEY_FILE:-/etc/nas-os/backup.key}"

# 校验配置
ENABLE_CHECKSUM="${ENABLE_CHECKSUM:-true}"

# 远程备份目标配置
BACKUP_TARGET="${BACKUP_TARGET:-local}"  # local | s3 | sftp

# S3 配置
S3_BUCKET="${S3_BUCKET:-}"
S3_REGION="${S3_REGION:-us-east-1}"
S3_ENDPOINT="${S3_ENDPOINT:-}"  # 用于兼容 S3 的存储
S3_ACCESS_KEY="${S3_ACCESS_KEY:-}"
S3_SECRET_KEY="${S3_SECRET_KEY:-}"

# SFTP 配置
SFTP_HOST="${SFTP_HOST:-}"
SFTP_PORT="${SFTP_PORT:-22}"
SFTP_USER="${SFTP_USER:-}"
SFTP_PATH="${SFTP_PATH:-/backup}"
SFTP_KEY_FILE="${SFTP_KEY_FILE:-/root/.ssh/id_rsa}"

# 通知配置
ENABLE_NOTIFICATION="${ENABLE_NOTIFICATION:-false}"
NOTIFICATION_WEBHOOK="${NOTIFICATION_WEBHOOK:-}"
NOTIFICATION_EMAIL="${NOTIFICATION_EMAIL:-}"
NOTIFICATION_DINGTALK="${NOTIFICATION_DINGTALK:-}"

# 日志配置
LOG_FILE="${LOG_FILE:-}"
LOG_MAX_SIZE="${LOG_MAX_SIZE:-10M}"
LOG_MAX_FILES="${LOG_MAX_FILES:-5}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 备份元数据
BACKUP_ID=""
BACKUP_START_TIME=""
BACKUP_END_TIME=""
BACKUP_SIZE=0
BACKUP_FILE=""
BACKUP_MANIFEST=""

# ============ 日志函数 ============
log_init() {
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    if [ -z "$LOG_FILE" ]; then
        LOG_FILE="$LOG_DIR/backup-$(date '+%Y%m%d').log"
    fi
    
    # 确保日志目录存在
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    
    # 日志轮转
    if [ -f "$LOG_FILE" ]; then
        local size
        size=$(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null || echo 0)
        local max_bytes
        max_bytes=$(echo "$LOG_MAX_SIZE" | sed 's/M/*1024*1024/;s/K/*1024/;s/G/*1024*1024*1024/' | bc)
        if [ "$size" -gt "$max_bytes" ]; then
            for i in $(seq $((LOG_MAX_FILES - 1)) -1 1); do
                [ -f "${LOG_FILE}.$i" ] && mv "${LOG_FILE}.$i" "${LOG_FILE}.$((i + 1))"
            done
            mv "$LOG_FILE" "${LOG_FILE}.1"
        fi
    fi
}

log() {
    local level="$1"
    local message="$2"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local log_line="[$timestamp] [$level] $message"
    
    # 写入日志文件
    echo "$log_line" >> "$LOG_FILE"
    
    # 控制台输出
    case "$level" in
        INFO)    echo -e "${BLUE}[INFO]${NC} $message" ;;
        SUCCESS) echo -e "${GREEN}[SUCCESS]${NC} $message" ;;
        WARN)    echo -e "${YELLOW}[WARN]${NC} $message" ;;
        ERROR)   echo -e "${RED}[ERROR]${NC} $message" ;;
        DEBUG)   [ "${DEBUG:-false}" = true ] && echo -e "${CYAN}[DEBUG]${NC} $message" ;;
    esac
}

log_info()    { log "INFO" "$1"; }
log_success()  { log "SUCCESS" "$1"; }
log_warn()     { log "WARN" "$1"; }
log_error()    { log "ERROR" "$1"; }
log_debug()    { log "DEBUG" "$1"; }

# ============ 帮助信息 ============
show_help() {
    cat << EOF
$SCRIPT_NAME v$SCRIPT_VERSION - NAS-OS 系统备份脚本

用法: $(basename "$0") [选项]

备份选项:
  -t, --type TYPE         备份类型: full (完整) 或 incremental (增量)
  -n, --name NAME         备份名称前缀 (默认: nas-os)
  -d, --dir DIR           备份存储目录 (默认: /var/lib/nas-os/backups)
  -c, --config FILE       配置文件路径
  
目标选项:
  --target TARGET         备份目标: local, s3, sftp (默认: local)
  
S3 选项:
  --s3-bucket BUCKET      S3 存储桶名称
  --s3-region REGION      S3 区域 (默认: us-east-1)
  --s3-endpoint URL       S3 兼容端点
  
SFTP 选项:
  --sftp-host HOST        SFTP 服务器地址
  --sftp-port PORT        SFTP 端口 (默认: 22)
  --sftp-user USER        SFTP 用户名
  --sftp-path PATH        SFTP 备份路径 (默认: /backup)

其他选项:
  --compress              启用压缩 (默认启用)
  --no-compress           禁用压缩
  --encrypt               启用加密
  --no-encrypt            禁用加密
  --notify                启用通知
  --no-notify             禁用通知
  -v, --verbose           详细输出
  --dry-run               模拟运行，不实际执行
  -h, --help              显示此帮助信息
  --version               显示版本信息

示例:
  # 完整备份到本地
  $(basename "$0") -t full

  # 增量备份到 S3
  $(basename "$0") -t incremental --target s3 --s3-bucket my-backups

  # 使用配置文件
  $(basename "$0") -c /etc/nas-os/backup.conf

配置文件格式 (backup.conf):
  BACKUP_TYPE=full
  BACKUP_DIR=/var/lib/nas-os/backups
  BACKUP_TARGET=s3
  S3_BUCKET=my-backups
  S3_REGION=us-east-1
  RETENTION_DAYS=30

EOF
    exit 0
}

show_version() {
    echo "$SCRIPT_NAME v$SCRIPT_VERSION"
    exit 0
}

# ============ 参数解析 ============
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -t|--type)
                BACKUP_TYPE="$2"
                shift 2
                ;;
            -n|--name)
                BACKUP_NAME="$2"
                shift 2
                ;;
            -d|--dir)
                BACKUP_DIR="$2"
                shift 2
                ;;
            -c|--config)
                load_config "$2"
                shift 2
                ;;
            --target)
                BACKUP_TARGET="$2"
                shift 2
                ;;
            --s3-bucket)
                S3_BUCKET="$2"
                shift 2
                ;;
            --s3-region)
                S3_REGION="$2"
                shift 2
                ;;
            --s3-endpoint)
                S3_ENDPOINT="$2"
                shift 2
                ;;
            --sftp-host)
                SFTP_HOST="$2"
                shift 2
                ;;
            --sftp-port)
                SFTP_PORT="$2"
                shift 2
                ;;
            --sftp-user)
                SFTP_USER="$2"
                shift 2
                ;;
            --sftp-path)
                SFTP_PATH="$2"
                shift 2
                ;;
            --compress)
                ENABLE_COMPRESS=true
                shift
                ;;
            --no-compress)
                ENABLE_COMPRESS=false
                shift
                ;;
            --encrypt)
                ENABLE_ENCRYPTION=true
                shift
                ;;
            --no-encrypt)
                ENABLE_ENCRYPTION=false
                shift
                ;;
            --notify)
                ENABLE_NOTIFICATION=true
                shift
                ;;
            --no-notify)
                ENABLE_NOTIFICATION=false
                shift
                ;;
            -v|--verbose)
                DEBUG=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                show_help
                ;;
            --version)
                show_version
                ;;
            *)
                log_error "未知选项: $1"
                echo "使用 -h 或 --help 查看帮助信息"
                exit 1
                ;;
        esac
    done
}

# ============ 配置加载 ============
load_config() {
    local config_file="$1"
    
    if [ ! -f "$config_file" ]; then
        log_error "配置文件不存在: $config_file"
        exit 1
    fi
    
    log_info "加载配置文件: $config_file"
    
    # 加载配置（跳过注释和空行）
    while IFS= read -r line || [ -n "$line" ]; do
        # 跳过注释和空行
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "${line// }" ]] && continue
        
        # 导出变量
        eval export "$line"
    done < "$config_file"
}

# ============ 依赖检查 ============
check_dependencies() {
    local missing=()
    
    # 基础依赖
    local required_cmds=("tar" "find" "date" "md5sum" "sha256sum")
    
    for cmd in "${required_cmds[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            missing+=("$cmd")
        fi
    done
    
    # 压缩依赖
    if [ "$ENABLE_COMPRESS" = true ]; then
        if ! command -v gzip &> /dev/null; then
            log_warn "gzip 未安装，将禁用压缩"
            ENABLE_COMPRESS=false
        fi
    fi
    
    # 加密依赖
    if [ "$ENABLE_ENCRYPTION" = true ]; then
        if ! command -v openssl &> /dev/null; then
            log_warn "openssl 未安装，将禁用加密"
            ENABLE_ENCRYPTION=false
        fi
    fi
    
    # S3 依赖
    if [ "$BACKUP_TARGET" = "s3" ]; then
        if ! command -v aws &> /dev/null && ! command -v s3cmd &> /dev/null && ! command -v rclone &> /dev/null; then
            missing+=("aws-cli or s3cmd or rclone (S3 支持)")
        fi
    fi
    
    # SFTP 依赖
    if [ "$BACKUP_TARGET" = "sftp" ]; then
        if ! command -v sftp &> /dev/null && ! command -v rsync &> /dev/null; then
            missing+=("sftp or rsync (SFTP 支持)")
        fi
    fi
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "缺少必要依赖: ${missing[*]}"
        log_error "请安装后重试"
        exit 1
    fi
    
    log_debug "依赖检查通过"
}

# ============ 环境准备 ============
prepare_environment() {
    # 创建备份目录
    if [ ! -d "$BACKUP_DIR" ]; then
        log_info "创建备份目录: $BACKUP_DIR"
        mkdir -p "$BACKUP_DIR"
    fi
    
    # 创建日志目录
    if [ ! -d "$LOG_DIR" ]; then
        mkdir -p "$LOG_DIR"
    fi
    
    # 创建临时目录
    TEMP_DIR=$(mktemp -d "${BACKUP_DIR}/.backup-XXXXXX")
    log_debug "临时目录: $TEMP_DIR"
    
    # 生成备份 ID
    BACKUP_ID="${BACKUP_NAME}-$(date '+%Y%m%d-%H%M%S')-${BACKUP_TYPE}"
    BACKUP_START_TIME=$(date '+%Y-%m-%d %H:%M:%S')
    BACKUP_FILE="${BACKUP_DIR}/${BACKUP_ID}.tar"
    BACKUP_MANIFEST="${BACKUP_DIR}/${BACKUP_ID}.manifest"
    
    log_debug "备份 ID: $BACKUP_ID"
}

# ============ 备份源准备 ============
prepare_backup_sources() {
    log_info "准备备份源..."
    
    BACKUP_SOURCES=()
    BACKUP_ITEMS=()
    
    # 数据目录
    if [ -d "$DATA_DIR" ]; then
        BACKUP_SOURCES+=("$DATA_DIR")
        BACKUP_ITEMS+=("数据目录: $DATA_DIR")
        log_debug "添加数据目录: $DATA_DIR"
    else
        log_warn "数据目录不存在: $DATA_DIR"
    fi
    
    # 配置目录
    if [ -d "$CONFIG_DIR" ]; then
        BACKUP_SOURCES+=("$CONFIG_DIR")
        BACKUP_ITEMS+=("配置目录: $CONFIG_DIR")
        log_debug "添加配置目录: $CONFIG_DIR"
    else
        log_warn "配置目录不存在: $CONFIG_DIR"
    fi
    
    # Docker 卷数据（如果存在）
    if [ -d "/var/lib/docker/volumes" ]; then
        # 只备份 NAS-OS 相关的卷
        local nas_volumes
        nas_volumes=$(find /var/lib/docker/volumes -maxdepth 1 -name "nas-*" -type d 2>/dev/null || true)
        if [ -n "$nas_volumes" ]; then
            while IFS= read -r vol; do
                [ -n "$vol" ] && BACKUP_SOURCES+=("$vol")
            done <<< "$nas_volumes"
            BACKUP_ITEMS+=("Docker 卷: nas-*")
        fi
    fi
    
    # 数据库文件（SQLite）
    local db_files
    db_files=$(find "$DATA_DIR" -name "*.db" -type f 2>/dev/null || true)
    if [ -n "$db_files" ]; then
        BACKUP_ITEMS+=("数据库文件")
        log_debug "发现数据库文件"
    fi
    
    if [ ${#BACKUP_SOURCES[@]} -eq 0 ]; then
        log_error "没有找到可备份的数据源"
        exit 1
    fi
    
    log_info "备份源准备完成: ${#BACKUP_SOURCES[@]} 个目录"
}

# ============ 完整备份 ============
do_full_backup() {
    log_info "开始完整备份..."
    
    local tar_opts=()
    tar_opts+=("-cf" "$BACKUP_FILE")
    
    # 排除备份目录本身
    tar_opts+=("--exclude" "$BACKUP_DIR")
    tar_opts+=("--exclude" "*.log")
    tar_opts+=("--exclude" "*.tmp")
    tar_opts+=("--exclude" ".backup-*")
    
    # 添加所有备份源
    for src in "${BACKUP_SOURCES[@]}"; do
        tar_opts+=("-C" "$(dirname "$src")" "$(basename "$src")")
    done
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] 将执行: tar ${tar_opts[*]}"
        return 0
    fi
    
    # 执行备份
    log_debug "执行: tar ${tar_opts[*]}"
    if ! tar "${tar_opts[@]}" 2>> "$LOG_FILE"; then
        log_error "备份创建失败"
        return 1
    fi
    
    # 压缩
    if [ "$ENABLE_COMPRESS" = true ]; then
        log_info "压缩备份文件..."
        if ! gzip -"$COMPRESS_LEVEL" -f "$BACKUP_FILE"; then
            log_error "压缩失败"
            return 1
        fi
        BACKUP_FILE="${BACKUP_FILE}.gz"
    fi
    
    # 加密
    if [ "$ENABLE_ENCRYPTION" = true ]; then
        log_info "加密备份文件..."
        if ! encrypt_file "$BACKUP_FILE"; then
            log_error "加密失败"
            return 1
        fi
        BACKUP_FILE="${BACKUP_FILE}.enc"
    fi
    
    # 记录文件大小
    BACKUP_SIZE=$(stat -f%z "$BACKUP_FILE" 2>/dev/null || stat -c%s "$BACKUP_FILE" 2>/dev/null || echo 0)
    
    log_success "完整备份完成: $(basename "$BACKUP_FILE") ($(format_size "$BACKUP_SIZE"))"
    return 0
}

# ============ 增量备份 ============
do_incremental_backup() {
    log_info "开始增量备份..."
    
    # 查找最近的完整备份
    local last_full
    last_full=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*-full.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)
    
    if [ -z "$last_full" ]; then
        log_warn "未找到完整备份，将执行完整备份"
        BACKUP_TYPE="full"
        do_full_backup
        return $?
    fi
    
    log_info "基于完整备份: $(basename "$last_full")"
    
    # 获取完整备份的时间戳
    local base_time
    base_time=$(stat -c%Y "$last_full" 2>/dev/null || stat -f%m "$last_full" 2>/dev/null)
    
    # 创建增量备份文件列表
    local files_list="$TEMP_DIR/incremental-files.txt"
    > "$files_list"
    
    log_info "查找变更文件..."
    
    # 查找变更的文件
    for src in "${BACKUP_SOURCES[@]}"; do
        if [ -d "$src" ]; then
            find "$src" -type f -newer "@$base_time" >> "$files_list" 2>/dev/null || true
        fi
    done
    
    local file_count
    file_count=$(wc -l < "$files_list")
    
    if [ "$file_count" -eq 0 ]; then
        log_info "没有变更文件，跳过增量备份"
        rm -f "$files_list"
        return 0
    fi
    
    log_info "发现 $file_count 个变更文件"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] 将备份 $file_count 个变更文件"
        head -20 "$files_list" | while read -r f; do
            log_debug "  - $f"
        done
        return 0
    fi
    
    # 创建增量备份
    local tar_opts=()
    tar_opts+=("-cf" "$BACKUP_FILE")
    tar_opts+=("-T" "$files_list")
    
    log_debug "执行: tar ${tar_opts[*]}"
    if ! tar "${tar_opts[@]}" 2>> "$LOG_FILE"; then
        log_error "增量备份创建失败"
        return 1
    fi
    
    # 压缩
    if [ "$ENABLE_COMPRESS" = true ]; then
        log_info "压缩备份文件..."
        gzip -"$COMPRESS_LEVEL" -f "$BACKUP_FILE"
        BACKUP_FILE="${BACKUP_FILE}.gz"
    fi
    
    # 加密
    if [ "$ENABLE_ENCRYPTION" = true ]; then
        log_info "加密备份文件..."
        encrypt_file "$BACKUP_FILE"
        BACKUP_FILE="${BACKUP_FILE}.enc"
    fi
    
    BACKUP_SIZE=$(stat -f%z "$BACKUP_FILE" 2>/dev/null || stat -c%s "$BACKUP_FILE" 2>/dev/null || echo 0)
    
    log_success "增量备份完成: $(basename "$BACKUP_FILE") ($(format_size "$BACKUP_SIZE"))"
    return 0
}

# ============ 校验 ============
create_checksum() {
    if [ "$ENABLE_CHECKSUM" = true ] && [ -f "$BACKUP_FILE" ]; then
        log_info "生成校验和..."
        local checksum_file="${BACKUP_FILE}.sha256"
        sha256sum "$BACKUP_FILE" > "$checksum_file"
        log_debug "校验文件: $checksum_file"
    fi
}

verify_backup() {
    if [ ! -f "$BACKUP_FILE" ]; then
        log_error "备份文件不存在: $BACKUP_FILE"
        return 1
    fi
    
    local checksum_file="${BACKUP_FILE}.sha256"
    if [ -f "$checksum_file" ]; then
        log_info "验证备份完整性..."
        if sha256sum -c "$checksum_file" &> /dev/null; then
            log_success "备份完整性验证通过"
            return 0
        else
            log_error "备份完整性验证失败"
            return 1
        fi
    else
        log_warn "未找到校验文件，跳过完整性验证"
        return 0
    fi
}

# ============ 加密/解密 ============
encrypt_file() {
    local input_file="$1"
    
    if [ ! -f "$ENCRYPTION_KEY_FILE" ]; then
        log_warn "加密密钥文件不存在，生成新密钥: $ENCRYPTION_KEY_FILE"
        openssl rand -hex 32 > "$ENCRYPTION_KEY_FILE"
        chmod 600 "$ENCRYPTION_KEY_FILE"
    fi
    
    local key
    key=$(cat "$ENCRYPTION_KEY_FILE")
    
    if ! openssl enc -aes-256-gcm -salt -pbkdf2 -in "$input_file" -out "${input_file}.enc" -pass pass:"$key"; then
        log_error "加密失败"
        return 1
    fi
    
    # 删除原文件
    rm -f "$input_file"
    
    log_debug "文件已加密: ${input_file}.enc"
    return 0
}

decrypt_file() {
    local input_file="$1"
    local output_file="$2"
    
    if [ ! -f "$ENCRYPTION_KEY_FILE" ]; then
        log_error "加密密钥文件不存在: $ENCRYPTION_KEY_FILE"
        return 1
    fi
    
    local key
    key=$(cat "$ENCRYPTION_KEY_FILE")
    
    if ! openssl enc -aes-256-gcm -d -pbkdf2 -in "$input_file" -out "$output_file" -pass pass:"$key"; then
        log_error "解密失败"
        return 1
    fi
    
    log_debug "文件已解密: $output_file"
    return 0
}

# ============ 远程同步 ============
sync_to_s3() {
    log_info "同步到 S3: s3://$S3_BUCKET"
    
    local s3_path="s3://${S3_BUCKET}/${BACKUP_NAME}/$(basename "$BACKUP_FILE")"
    
    if command -v aws &> /dev/null; then
        local aws_opts=()
        [ -n "$S3_REGION" ] && aws_opts+=("--region" "$S3_REGION")
        [ -n "$S3_ENDPOINT" ] && aws_opts+=("--endpoint-url" "$S3_ENDPOINT")
        
        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY-RUN] 将执行: aws s3 cp $BACKUP_FILE $s3_path"
            return 0
        fi
        
        if ! aws s3 cp "${aws_opts[@]}" "$BACKUP_FILE" "$s3_path"; then
            log_error "S3 上传失败"
            return 1
        fi
        
        # 上传校验文件
        if [ -f "${BACKUP_FILE}.sha256" ]; then
            aws s3 cp "${aws_opts[@]}" "${BACKUP_FILE}.sha256" "${s3_path}.sha256"
        fi
        
    elif command -v rclone &> /dev/null; then
        local remote_name="${S3_REMOTE:-s3}"
        local rclone_path="${remote_name}:${S3_BUCKET}/${BACKUP_NAME}/"
        
        if [ "$DRY_RUN" = true ]; then
            log_info "[DRY-RUN] 将执行: rclone copy $BACKUP_FILE $rclone_path"
            return 0
        fi
        
        if ! rclone copy "$BACKUP_FILE" "$rclone_path"; then
            log_error "rclone 上传失败"
            return 1
        fi
    else
        log_error "未找到 S3 客户端 (aws-cli 或 rclone)"
        return 1
    fi
    
    log_success "S3 同步完成"
    return 0
}

sync_to_sftp() {
    log_info "同步到 SFTP: $SFTP_HOST:$SFTP_PATH"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] 将执行: rsync/scp $BACKUP_FILE -> $SFTP_HOST:$SFTP_PATH"
        return 0
    fi
    
    # 确保远程目录存在
    ssh -p "$SFTP_PORT" -i "$SFTP_KEY_FILE" "${SFTP_USER}@${SFTP_HOST}" "mkdir -p $SFTP_PATH/$BACKUP_NAME" 2>> "$LOG_FILE"
    
    # 使用 rsync 或 scp
    if command -v rsync &> /dev/null; then
        if ! rsync -avz -e "ssh -p $SFTP_PORT -i $SFTP_KEY_FILE" \
            "$BACKUP_FILE" "${SFTP_USER}@${SFTP_HOST}:${SFTP_PATH}/${BACKUP_NAME}/"; then
            log_error "rsync 上传失败"
            return 1
        fi
    else
        if ! scp -P "$SFTP_PORT" -i "$SFTP_KEY_FILE" \
            "$BACKUP_FILE" "${SFTP_USER}@${SFTP_HOST}:${SFTP_PATH}/${BACKUP_NAME}/"; then
            log_error "scp 上传失败"
            return 1
        fi
    fi
    
    log_success "SFTP 同步完成"
    return 0
}

# ============ 清理旧备份 ============
cleanup_old_backups() {
    log_info "清理旧备份 (保留 $RETENTION_DAYS 天, 最多 $RETENTION_COUNT 个)..."
    
    local deleted=0
    
    # 按时间清理
    local old_backups
    old_backups=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -mtime +$RETENTION_DAYS 2>/dev/null || true)
    
    if [ -n "$old_backups" ]; then
        while IFS= read -r backup; do
            [ -z "$backup" ] && continue
            if [ "$DRY_RUN" = true ]; then
                log_info "[DRY-RUN] 将删除: $backup"
            else
                log_debug "删除旧备份: $backup"
                rm -f "$backup" "${backup}.sha256" "${backup}.manifest"
                ((deleted++))
            fi
        done <<< "$old_backups"
    fi
    
    # 按数量清理
    local current_count
    current_count=$(find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f 2>/dev/null | wc -l)
    
    if [ "$current_count" -gt "$RETENTION_COUNT" ]; then
        local to_delete=$((current_count - RETENTION_COUNT))
        log_info "备份数量超限 ($current_count > $RETENTION_COUNT)，删除 $to_delete 个最旧备份"
        
        find "$BACKUP_DIR" -name "${BACKUP_NAME}-*.tar*" -type f -printf '%T@ %p\n' 2>/dev/null | \
            sort -n | head -n "$to_delete" | while read -r _ backup; do
            if [ "$DRY_RUN" = true ]; then
                log_info "[DRY-RUN] 将删除: $backup"
            else
                log_debug "删除旧备份: $backup"
                rm -f "$backup" "${backup}.sha256" "${backup}.manifest"
                ((deleted++))
            fi
        done
    fi
    
    log_info "清理完成，删除 $deleted 个旧备份"
}

# ============ 清单生成 ============
create_manifest() {
    log_info "生成备份清单..."
    
    cat > "$BACKUP_MANIFEST" << EOF
# NAS-OS 备份清单
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')

[备份信息]
ID = $BACKUP_ID
类型 = $BACKUP_TYPE
名称 = $BACKUP_NAME
时间 = $BACKUP_START_TIME
主机 = $(hostname)
版本 = $SCRIPT_VERSION

[备份内容]
EOF
    
    for item in "${BACKUP_ITEMS[@]}"; do
        echo "  - $item" >> "$BACKUP_MANIFEST"
    done
    
    cat >> "$BACKUP_MANIFEST" << EOF

[文件信息]
备份文件 = $(basename "$BACKUP_FILE")
文件大小 = $(format_size "$BACKUP_SIZE")
压缩 = $ENABLE_COMPRESS
加密 = $ENABLE_ENCRYPTION
校验 = $([ -f "${BACKUP_FILE}.sha256" ] && cat "${BACKUP_FILE}.sha256" | cut -d' ' -f1 || echo "无")

[目标]
目标类型 = $BACKUP_TARGET
EOF
    
    case "$BACKUP_TARGET" in
        s3)
            echo "S3 存储桶 = $S3_BUCKET" >> "$BACKUP_MANIFEST"
            echo "S3 区域 = $S3_REGION" >> "$BACKUP_MANIFEST"
            ;;
        sftp)
            echo "SFTP 主机 = $SFTP_HOST" >> "$BACKUP_MANIFEST"
            echo "SFTP 路径 = $SFTP_PATH" >> "$BACKUP_MANIFEST"
            ;;
    esac
    
    log_debug "清单文件: $BACKUP_MANIFEST"
}

# ============ 通知 ============
send_notification() {
    local status="$1"
    local message="$2"
    
    if [ "$ENABLE_NOTIFICATION" != true ]; then
        return 0
    fi
    
    log_info "发送通知..."
    
    local title="NAS-OS 备份通知"
    local body
    body=$(cat << EOF
备份状态: $status
备份类型: $BACKUP_TYPE
备份名称: $BACKUP_ID
开始时间: $BACKUP_START_TIME
结束时间: $BACKUP_END_TIME
文件大小: $(format_size "$BACKUP_SIZE")

$message
EOF
)
    
    # Webhook 通知
    if [ -n "$NOTIFICATION_WEBHOOK" ]; then
        local payload
        payload=$(cat << EOF
{
    "title": "$title",
    "message": "$body",
    "status": "$status",
    "backup_id": "$BACKUP_ID",
    "backup_type": "$BACKUP_TYPE",
    "backup_size": "$BACKUP_SIZE",
    "timestamp": "$(date -Iseconds)"
}
EOF
)
        if command -v curl &> /dev/null; then
            curl -s -X POST -H "Content-Type: application/json" \
                -d "$payload" "$NOTIFICATION_WEBHOOK" >> "$LOG_FILE" 2>&1
        fi
    fi
    
    # 钉钉通知
    if [ -n "$NOTIFICATION_DINGTALK" ]; then
        local dingtalk_body
        dingtalk_body=$(cat << EOF
{
    "msgtype": "markdown",
    "markdown": {
        "title": "$title",
        "text": "## $title\n\n$body"
    }
}
EOF
)
        if command -v curl &> /dev/null; then
            curl -s -X POST -H "Content-Type: application/json" \
                -d "$dingtalk_body" "$NOTIFICATION_DINGTALK" >> "$LOG_FILE" 2>&1
        fi
    fi
    
    # 邮件通知
    if [ -n "$NOTIFICATION_EMAIL" ] && command -v mail &> /dev/null; then
        echo "$body" | mail -s "$title" "$NOTIFICATION_EMAIL"
    fi
    
    log_debug "通知发送完成"
}

# ============ 工具函数 ============
format_size() {
    local bytes=$1
    local units=("B" "KB" "MB" "GB" "TB")
    local unit=0
    local size=$bytes
    
    while [ "$size" -ge 1024 ] && [ $unit -lt 4 ]; do
        size=$(echo "scale=2; $size / 1024" | bc)
        ((unit++))
    done
    
    printf "%.2f %s" "$size" "${units[$unit]}"
}

# ============ 清理 ============
cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
        log_debug "清理临时目录: $TEMP_DIR"
    fi
}

# ============ 主流程 ============
main() {
    # 初始化
    log_init
    parse_args "$@"
    
    log_info "=========================================="
    log_info "$SCRIPT_NAME v$SCRIPT_VERSION"
    log_info "=========================================="
    log_info "备份类型: $BACKUP_TYPE"
    log_info "备份目标: $BACKUP_TARGET"
    log_info "备份目录: $BACKUP_DIR"
    
    # 检查
    check_dependencies
    prepare_environment
    prepare_backup_sources
    
    # 执行备份
    local backup_status="成功"
    local backup_message=""
    
    trap 'cleanup' EXIT
    
    case "$BACKUP_TYPE" in
        full)
            if ! do_full_backup; then
                backup_status="失败"
                backup_message="完整备份执行失败"
            fi
            ;;
        incremental)
            if ! do_incremental_backup; then
                backup_status="失败"
                backup_message="增量备份执行失败"
            fi
            ;;
        *)
            log_error "未知备份类型: $BACKUP_TYPE"
            exit 1
            ;;
    esac
    
    # 生成校验
    if [ "$backup_status" = "成功" ] && [ "$DRY_RUN" != true ]; then
        create_checksum
        verify_backup
        create_manifest
    fi
    
    # 远程同步
    if [ "$backup_status" = "成功" ] && [ "$BACKUP_TARGET" != "local" ] && [ "$DRY_RUN" != true ]; then
        case "$BACKUP_TARGET" in
            s3)   sync_to_s3 ;;
            sftp) sync_to_sftp ;;
        esac
    fi
    
    # 清理
    if [ "$DRY_RUN" != true ]; then
        cleanup_old_backups
    fi
    
    # 完成
    BACKUP_END_TIME=$(date '+%Y-%m-%d %H:%M:%S')
    
    log_info "=========================================="
    log_info "备份 $backup_status"
    log_info "开始: $BACKUP_START_TIME"
    log_info "结束: $BACKUP_END_TIME"
    log_info "大小: $(format_size "$BACKUP_SIZE")"
    log_info "=========================================="
    
    # 发送通知
    if [ "$DRY_RUN" != true ]; then
        send_notification "$backup_status" "$backup_message"
    fi
    
    # 返回状态
    if [ "$backup_status" = "成功" ]; then
        exit 0
    else
        exit 1
    fi
}

# 运行
main "$@"