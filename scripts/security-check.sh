#!/bin/bash
#
# NAS-OS 安全检查脚本
# 功能：运行 Gosec 和 govulncheck，生成报告并发送通知
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
REPORTS_DIR="$PROJECT_DIR/reports/security"
DATE=$(date +%Y%m%d_%H%M%S)

# 颜色定义
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# 创建报告目录
mkdir -p "$REPORTS_DIR"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查工具是否安装
check_tools() {
    log_info "检查安全工具..."
    
    if ! command -v gosec &> /dev/null; then
        if [ -f "$HOME/go/bin/gosec" ]; then
            GOSEC="$HOME/go/bin/gosec"
        else
            log_error "gosec 未安装，运行：go install github.com/securego/gosec/v2/cmd/gosec@latest"
            exit 1
        fi
    else
        GOSEC="gosec"
    fi
    
    if ! command -v govulncheck &> /dev/null; then
        if [ -f "$HOME/go/bin/govulncheck" ]; then
            GOVULNCHECK="$HOME/go/bin/govulncheck"
        else
            log_error "govulncheck 未安装，运行：go install golang.org/x/vuln/cmd/govulncheck@latest"
            exit 1
        fi
    else
        GOVULNCHECK="govulncheck"
    fi
    
    log_info "工具检查完成"
}

# 运行 Gosec 扫描
run_gosec() {
    log_info "运行 Gosec 代码安全扫描..."
    
    cd "$PROJECT_DIR"
    
    # 生成文本报告
    GOSEC_REPORT="$REPORTS_DIR/gosec_${DATE}.txt"
    GOSEC_SARIF="$REPORTS_DIR/gosec_${DATE}.sarif"
    
    if $GOSEC -fmt=text ./... > "$GOSEC_REPORT" 2>&1; then
        log_info "✓ Gosec 扫描完成 - 未发现问题"
    else
        ISSUE_COUNT=$(grep -c "^\[" "$GOSEC_REPORT" || echo "0")
        log_warn "⚠ Gosec 发现 $ISSUE_COUNT 个安全问题"
        log_info "报告：$GOSEC_REPORT"
    fi
    
    # 生成 SARIF 格式（用于 CI/CD）
    $GOSEC -fmt=sarif -out="$GOSEC_SARIF" ./... 2>/dev/null || true
    
    echo "$GOSEC_REPORT"
}

# 运行 govulncheck
run_vulncheck() {
    log_info "运行依赖漏洞检查..."
    
    cd "$PROJECT_DIR"
    
    VULN_REPORT="$REPORTS_DIR/vulncheck_${DATE}.txt"
    
    if $GOVULNCHECK ./... > "$VULN_REPORT" 2>&1; then
        log_info "✓ 依赖检查完成 - 未发现漏洞"
    else
        EXIT_CODE=$?
        if [ $EXIT_CODE -eq 3 ]; then
            VULN_COUNT=$(grep -c "^Vulnerability #" "$VULN_REPORT" || echo "0")
            log_warn "⚠ 发现 $VULN_COUNT 个依赖漏洞"
            log_info "报告：$VULN_REPORT"
        else
            log_error "govulncheck 执行失败 (exit code: $EXIT_CODE)"
        fi
    fi
    
    echo "$VULN_REPORT"
}

# 发送通知
send_notification() {
    local gosec_report="$1"
    local vuln_report="$2"
    
    log_info "生成安全摘要..."
    
    SUMMARY_FILE="$REPORTS_DIR/security_summary_${DATE}.md"
    
    # 统计问题
    GOSEC_HIGH=$(grep -c "Severity: HIGH" "$gosec_report" 2>/dev/null || echo "0")
    GOSEC_MEDIUM=$(grep -c "Severity: MEDIUM" "$gosec_report" 2>/dev/null || echo "0")
    VULN_COUNT=$(grep -c "^Vulnerability #" "$vuln_report" 2>/dev/null || echo "0")
    
    cat > "$SUMMARY_FILE" << EOF
# 安全检查报告

**日期:** $(date '+%Y-%m-%d %H:%M:%S')
**项目:** NAS-OS

## 摘要

| 检查项 | 高危 | 中危 | 低危 |
|--------|------|------|------|
| 代码安全 (Gosec) | $GOSEC_HIGH | $GOSEC_MEDIUM | 0 |
| 依赖漏洞 | $VULN_COUNT | 0 | 0 |

## 详情

### Gosec 扫描
$(head -50 "$gosec_report" | grep -E "^\[.*\] - G[0-9]+" || echo "无问题")

### 依赖漏洞
$(grep -A5 "^Vulnerability #" "$vuln_report" | head -30 || echo "无问题")

## 建议操作

EOF

    if [ "$GOSEC_HIGH" -gt 0 ] || [ "$VULN_COUNT" -gt 0 ]; then
        cat >> "$SUMMARY_FILE" << EOF
⚠️ **需要立即关注：**

1. 审查高危问题并分配修复任务
2. 更新 Go 版本至 1.26.1+ 修复标准库漏洞
3. 修复路径遍历和命令注入风险

详细报告：
- Gosec: $gosec_report
- Vulncheck: $vuln_report
EOF
    else
        echo "✅ 所有安全检查通过！" >> "$SUMMARY_FILE"
    fi
    
    log_info "安全摘要：$SUMMARY_FILE"
    
    # 如果配置了通知渠道，发送通知
    if [ -n "$NOTIFY_CHANNEL" ]; then
        log_info "发送通知到：$NOTIFY_CHANNEL"
        # 这里可以集成 Discord/Slack/邮件等通知
        # 示例：curl -X POST "$NOTIFY_CHANNEL" -d "content=$(cat $SUMMARY_FILE)"
    fi
    
    echo "$SUMMARY_FILE"
}

# 主函数
main() {
    log_info "=========================================="
    log_info "  NAS-OS 安全检查"
    log_info "=========================================="
    
    NOTIFY=false
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --notify|-n)
                NOTIFY=true
                shift
                ;;
            --help|-h)
                echo "用法：$0 [--notify]"
                echo "  --notify, -n  发送通知"
                exit 0
                ;;
            *)
                log_error "未知参数：$1"
                exit 1
                ;;
        esac
    done
    
    check_tools
    GOSEC_REPORT=$(run_gosec)
    VULN_REPORT=$(run_vulncheck)
    
    if [ "$NOTIFY" = true ]; then
        send_notification "$GOSEC_REPORT" "$VULN_REPORT"
    fi
    
    log_info "=========================================="
    log_info "  安全检查完成"
    log_info "=========================================="
}

main "$@"
