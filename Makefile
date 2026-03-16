# NAS-OS Makefile
# 构建、测试、部署自动化

.PHONY: all build test clean run docker help ci

# 变量
BINARY_NAME=nasd
CLI_NAME=nasctl
GO=go
GOFLAGS=-ldflags="-w -s"
DOCKER_IMAGE=ghcr.io/nas-os/nas-os
DOCKER_TAG=latest

# 版本信息 (v2.123.0)
VERSION_FILE=VERSION
VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "v0.0.0")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(shell go version | awk '{print $$3}' 2>/dev/null || echo "unknown")

# v2.123.0 新增: GOPROXY 配置
GOPROXY ?= https://proxy.golang.org,direct

# 默认目标
all: build

# ========== 构建 ==========
build:
	@echo "🔨 编译 nasd..."
	GOPROXY=$(GOPROXY) $(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/nasd
	@echo "🔨 编译 nasctl..."
	GOPROXY=$(GOPROXY) $(GO) build $(GOFLAGS) -o $(CLI_NAME) ./cmd/nasctl
	@echo "✅ 构建完成"

# 带版本信息的构建
build-version:
	@echo "🔨 编译带版本信息的二进制..."
	GOPROXY=$(GOPROXY) $(GO) build -ldflags="-w -s -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o $(BINARY_NAME) ./cmd/nasd
	GOPROXY=$(GOPROXY) $(GO) build -ldflags="-w -s -X main.Version=$(VERSION)" -o $(CLI_NAME) ./cmd/nasctl
	@echo "✅ 构建完成: $(VERSION) ($(GIT_COMMIT))"

build-debug:
	@echo "🔨 编译调试版本..."
	$(GO) build -race -o $(BINARY_NAME)-debug ./cmd/nasd

# 跨平台构建
build-all:
	@echo "🌍 构建多平台二进制..."
	GOPROXY=$(GOPROXY) GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/nasd
	GOPROXY=$(GOPROXY) GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/nasd
	GOPROXY=$(GOPROXY) GOOS=linux GOARCH=arm GOARM=7 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-armv7 ./cmd/nasd
	@echo "✅ 多平台构建完成"

# ========== 测试 ==========
test:
	@echo "🧪 运行测试..."
	$(GO) test -v -timeout 10m -parallel 4 ./...

test-integration:
	@echo "🔗 运行集成测试..."
	$(GO) test -v -timeout 15m ./tests/integration/...

test-e2e:
	@echo "🚀 运行 E2E 测试..."
	NAS_OS_E2E=1 $(GO) test -v ./tests/e2e/...

test-benchmark:
	@echo "⚡ 运行性能基准测试..."
	$(GO) test -bench=. -benchmem -run=^$ ./tests/benchmark/...

test-report:
	@echo "📋 生成测试报告..."
	@mkdir -p tests/reports/output
	$(GO) test -v -json ./... > tests/reports/output/test_results.json 2>&1 || true
	@echo "✅ 测试报告已生成：tests/reports/output/"

test-suite:
	@echo "🎯 运行完整测试套件..."
	@./scripts/test.sh all -v -r -c

test-coverage:
	@echo "📊 生成覆盖率报告..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "📄 覆盖率报告：coverage.html"

test-race:
	@echo "🏃 竞态检测..."
	$(GO) test -race ./...

test-all: test test-integration
	@echo "✅ 所有测试完成"

# ========== 代码质量 ==========
lint:
	@echo "🔍 代码检查..."
	golangci-lint run --timeout 5m

fmt:
	@echo "📐 格式化代码..."
	gofmt -w .

tidy:
	@echo "📦 整理依赖..."
	$(GO) mod tidy

# ========== 运行 ==========
run: build
	@echo "🚀 启动 NAS-OS..."
	sudo ./$(BINARY_NAME)

run-dev:
	@echo "🚀 开发模式启动..."
	$(GO) run ./cmd/nasd

# ========== Docker ==========
docker-build:
	@echo "🐳 构建 Docker 镜像..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# 多架构 Docker 构建 (v2.86.0)
docker-buildx:
	@echo "🐳 构建多架构 Docker 镜像 (amd64, arm64, armv7)..."
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-buildx-push:
	@echo "🐳 构建并推送多架构 Docker 镜像..."
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --push -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run:
	@echo "🐳 启动容器..."
	docker-compose up -d

docker-logs:
	docker-compose logs -f

docker-stop:
	docker-compose down

docker-clean:
	@echo "🧹 清理 Docker 资源..."
	docker-compose down -v
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true

# ========== 清理 ==========
clean:
	@echo "🧹 清理构建产物..."
	rm -f $(BINARY_NAME) $(CLI_NAME)
	rm -f $(BINARY_NAME)-* $(CLI_NAME)-*
	rm -f coverage.out coverage.html
	@echo "✅ 清理完成"

# ========== 安装 ==========
install: build
	@echo "📦 安装到系统..."
	sudo install -m 755 $(BINARY_NAME) /usr/local/bin/
	sudo install -m 755 $(CLI_NAME) /usr/local/bin/
	sudo mkdir -p /etc/nas-os
	sudo cp configs/default.yaml /etc/nas-os/config.yaml
	@echo "✅ 安装完成"

uninstall:
	@echo "🗑️ 卸载..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	sudo rm -f /usr/local/bin/$(CLI_NAME)
	sudo systemctl stop nas-os || true
	sudo systemctl disable nas-os || true
	sudo rm -f /etc/systemd/system/nas-os.service
	@echo "✅ 卸载完成 (保留配置和数据)"

# ========== 插件 ==========
plugin-build:
	@echo "🔧 构建示例插件..."
	@echo "提示：插件需要在独立的 go module 中构建"
	@echo ""
	@echo "示例："
	@echo "  cd plugins/filemanager-enhance"
	@echo "  go mod init filemanager-enhance"
	@echo "  go build -buildmode=plugin -o filemanager-enhance.so"

plugin-install:
	@echo "📦 安装插件目录..."
	sudo mkdir -p /opt/nas/plugins
	sudo mkdir -p /etc/nas-os/plugins
	sudo mkdir -p /var/lib/nas-os/plugins
	sudo cp -r plugins/* /opt/nas/plugins/
	@echo "✅ 插件目录已创建"

# ========== 帮助 ==========
help:
	@echo "NAS-OS Makefile 命令:"
	@echo ""
	@echo "  构建:"
	@echo "    make build          - 编译二进制文件"
	@echo "    make build-version  - 编译带版本信息的二进制"
	@echo "    make build-all      - 跨平台构建"
	@echo "    make build-debug    - 调试版本"
	@echo "    make quick          - 快速构建 (跳过测试)"
	@echo ""
	@echo "  测试:"
	@echo "    make test           - 运行单元测试"
	@echo "    make test-integration - 运行集成测试"
	@echo "    make test-e2e       - 运行 E2E 测试"
	@echo "    make test-benchmark - 运行性能基准测试"
	@echo "    make test-report    - 生成测试报告"
	@echo "    make test-suite     - 运行完整测试套件"
	@echo "    make test-coverage  - 生成覆盖率报告"
	@echo "    make test-race      - 竞态检测"
	@echo ""
	@echo "  代码质量:"
	@echo "    make lint           - 代码检查"
	@echo "    make fmt            - 格式化代码"
	@echo "    make tidy           - 整理依赖"
	@echo "    make check          - 快速检查 (fmt + lint)"
	@echo "    make pre-commit     - 预提交检查 (tidy + fmt + lint + test)"
	@echo "    make all-checks     - 完整检查流程"
	@echo ""
	@echo "  开发辅助 (v2.122.0):"
	@echo "    make dev-setup      - 设置开发环境"
	@echo "    make dev            - 热重载开发服务器"
	@echo "    make stats          - 代码统计"
	@echo "    make todos          - 查找 TODO/FIXME"
	@echo "    make deps           - 显示依赖"
	@echo "    make deps-update    - 更新依赖"
	@echo ""
	@echo "  文档:"
	@echo "    make swagger        - 生成 Swagger/OpenAPI 文档"
	@echo "    make swagger-html   - 生成静态 HTML 文档"
	@echo "    make docs-export    - 导出文档 (HTML/PDF/Markdown)"
	@echo ""
	@echo "  运行:"
	@echo "    make run            - 启动服务"
	@echo "    make run-dev        - 开发模式"
	@echo ""
	@echo "  Docker:"
	@echo "    make docker-build   - 构建镜像"
	@echo "    make docker-buildx  - 构建多架构镜像 (amd64, arm64, armv7)"
	@echo "    make docker-buildx-push - 构建并推送多架构镜像"
	@echo "    make docker-run     - 启动容器"
	@echo "    make docker-logs    - 查看日志"
	@echo "    make docker-stop    - 停止容器"
	@echo ""
	@echo "  安装:"
	@echo "    make install        - 安装到系统"
	@echo "    make uninstall      - 卸载"
	@echo ""
	@echo "  插件:"
	@echo "    make plugin-build   - 构建示例插件（提示）"
	@echo "    make plugin-install - 安装插件目录"
	@echo ""
	@echo "  监控:"
	@echo "    make monitor-up     - 启动监控栈"
	@echo "    make monitor-down   - 停止监控栈"
	@echo "    make monitor-logs   - 查看监控日志"
	@echo "    make alert-validate - 验证告警规则"
	@echo "    make prometheus-validate - 验证 Prometheus 配置"
	@echo "    make alertmanager-validate - 验证 Alertmanager 配置"
	@echo "    make monitor-validate - 验证所有监控配置"
	@echo ""
	@echo "  健康检查:"
	@echo "    make health         - 执行完整健康检查"
	@echo "    make health-quick   - 快速健康检查"
	@echo "    make health-json    - JSON 格式输出"
	@echo "    make health-prometheus - Prometheus 格式输出"
	@echo "    make health-monitor - 持续监控模式"
	@echo ""
	@echo "  快速状态 (v2.75.0):"
	@echo "    make quick-status   - 快速状态检查"
	@echo "    make quick-status-json - JSON 格式输出"
	@echo ""
	@echo "  版本管理 (v2.122.0):"
	@echo "    make version        - 显示版本信息"
	@echo "    make version-info   - 详细版本信息"
	@echo "    make version-info-json - JSON 格式版本信息"
	@echo "    make version-check  - 检查更新"
	@echo "    make version-json   - JSON 格式版本信息"
	@echo "    make version-status - 检查版本状态"
	@echo ""
	@echo "  服务管理 (v2.75.0):"
	@echo "    make service-start  - 启动服务"
	@echo "    make service-stop   - 停止服务"
	@echo "    make service-restart - 重启服务"
	@echo "    make service-status - 服务状态"
	@echo "    make service-logs   - 查看日志"
	@echo ""
	@echo "  备份监控 (v2.59.0):"
	@echo "    make backup-monitor - 执行备份监控检查"
	@echo "    make backup-monitor-status - 查看备份状态"
	@echo "    make backup-monitor-daemon - 启动备份监控守护进程"
	@echo "    make backup-verify  - 验证最新备份"
	@echo "    make backup-report  - 生成备份报告"
	@echo ""
	@echo "  磁盘健康 (v2.59.0):"
	@echo "    make disk-health    - 检查磁盘健康状态"
	@echo "    make disk-health-json - JSON 格式输出"
	@echo "    make disk-health-monitor - 启动磁盘健康监控"
	@echo "    make disk-smart-test - 运行 SMART 短测试"
	@echo "    make disk-smart-test-long - 运行 SMART 长测试"
	@echo ""
	@echo "  日志分析 (v2.122.0):"
	@echo "    make log-analyze    - 分析日志文件"
	@echo "    make log-analyze-json - JSON 格式输出"
	@echo "    make log-report     - 生成日志报告"
	@echo ""
	@echo "  其他:"
	@echo "    make clean          - 清理构建产物"
	@echo "    make help           - 显示帮助"

# ========== API 文档 ==========

# 生成 Swagger/OpenAPI 文档
swagger:
	@echo "📚 生成 Swagger/OpenAPI 文档..."
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/nasd/main.go -o docs/swagger --parseDependency --parseInternal
	@echo "✅ 文档生成完成："
	@echo "   - docs/swagger/swagger.json"
	@echo "   - docs/swagger/swagger.yaml"
	@echo "   - docs/swagger/docs.go"
	@echo ""
	@echo "📖 访问 Swagger UI: http://localhost:8080/swagger/index.html"

# 生成静态 HTML 文档 (使用 redoc)
swagger-html:
	@echo "📄 生成静态 HTML 文档..."
	@which npx > /dev/null || (echo "❌ 需要安装 Node.js/npm" && exit 1)
	@if [ ! -f docs/swagger/swagger.json ]; then $(MAKE) swagger; fi
	@mkdir -p docs/html
	npx redoc-cli bundle docs/swagger/swagger.json -o docs/html/api-docs.html --title "NAS-OS API 文档"
	@echo "✅ 静态文档生成完成：docs/html/api-docs.html"

# 导出多种格式文档
docs-export: swagger
	@echo "📦 导出多种格式文档..."
	@mkdir -p docs/export
	
	# JSON 格式
	@cp docs/swagger/swagger.json docs/export/openapi.json
	@echo "✅ JSON: docs/export/openapi.json"
	
	# YAML 格式
	@cp docs/swagger/swagger.yaml docs/export/openapi.yaml
	@echo "✅ YAML: docs/export/openapi.yaml"
	
	# Markdown 格式
	@echo "📝 生成 Markdown 文档..."
	@which npx > /dev/null || (echo "⚠️ 跳过 Markdown (需要 Node.js)" && exit 0)
	npx widdershins docs/swagger/swagger.json -o docs/export/API.md --summary || true
	@echo "✅ Markdown: docs/export/API.md"
	
	# HTML 格式
	@$(MAKE) swagger-html
	@cp docs/html/api-docs.html docs/export/api-docs.html 2>/dev/null || true
	@echo "✅ HTML: docs/export/api-docs.html"
	
	@echo ""
	@echo "📋 文档导出完成："
	@ls -la docs/export/

# 启动文档服务器
docs-serve: swagger
	@echo "🌐 启动文档服务器..."
	@echo "📖 Swagger UI: http://localhost:8081/swagger/"
	@echo "📖 ReDoc: http://localhost:8081/docs/"
	@echo "按 Ctrl+C 停止"
	@cd docs/swagger && python3 -m http.server 8081 || python -m SimpleHTTPServer 8081

# ========== 监控 ==========
monitor-up:
	@echo "📊 启动监控栈..."
	docker-compose -f docker-compose.yml -f monitoring/docker-compose.monitoring.yml up -d 2>/dev/null || \
		docker-compose up -d
	@echo "✅ 监控服务已启动"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana: http://localhost:3000"
	@echo "   Alertmanager: http://localhost:9093"

monitor-down:
	@echo "📊 停止监控栈..."
	docker-compose -f docker-compose.yml -f monitoring/docker-compose.monitoring.yml down 2>/dev/null || \
		docker-compose down
	@echo "✅ 监控服务已停止"

monitor-logs:
	docker-compose logs -f prometheus alertmanager grafana 2>/dev/null || \
		docker-compose logs -f

# 验证告警规则
alert-validate:
	@echo "🔍 验证 Prometheus 告警规则..."
	@which promtool > /dev/null || (echo "安装 promtool..." && go install github.com/prometheus/prometheus/cmd/promtool@latest)
	promtool check rules monitoring/alerts.yml
	@echo "✅ 告警规则验证通过"

# 验证 Prometheus 配置
prometheus-validate:
	@echo "🔍 验证 Prometheus 配置..."
	@which promtool > /dev/null || (echo "安装 promtool..." && go install github.com/prometheus/prometheus/cmd/promtool@latest)
	promtool check config monitoring/prometheus.yml
	@echo "✅ Prometheus 配置验证通过"

# 验证 Alertmanager 配置
alertmanager-validate:
	@echo "🔍 验证 Alertmanager 配置..."
	@which amtool > /dev/null || (echo "安装 amtool..." && go install github.com/prometheus/alertmanager/cmd/amtool@latest)
	amtool check-config monitoring/alertmanager.yml
	@echo "✅ Alertmanager 配置验证通过"

# 验证所有监控配置
monitor-validate: alert-validate prometheus-validate alertmanager-validate
	@echo "✅ 所有监控配置验证通过"

# ========== 健康检查 ==========
health:
	@echo "🏥 执行系统健康检查..."
	@./scripts/health-check.sh full

health-quick:
	@echo "🏥 执行快速健康检查..."
	@./scripts/health-check.sh quick

health-json:
	@echo "🏥 执行健康检查 (JSON 格式)..."
	@./scripts/health-check.sh full --json

health-prometheus:
	@echo "🏥 执行健康检查 (Prometheus 格式)..."
	@OUTPUT_FORMAT=prometheus ./scripts/health-check.sh full

health-monitor:
	@echo "🏥 启动健康监控模式..."
	@./scripts/health-check.sh monitor 30

# ========== 快速状态 (v2.75.0) ==========
quick-status:
	@echo "⚡ 快速状态检查..."
	@./scripts/quick-status.sh

quick-status-json:
	@echo "⚡ 快速状态检查 (JSON)..."
	@./scripts/quick-status.sh --json

# ========== 版本信息 (v2.75.0) ==========
version:
	@./scripts/version-info.sh

version-check:
	@./scripts/version-info.sh --check

version-json:
	@./scripts/version-info.sh --json

# ========== 服务管理 (v2.75.0) ==========
service-start:
	@./scripts/service.sh start

service-stop:
	@./scripts/service.sh stop

service-restart:
	@./scripts/service.sh restart

service-status:
	@./scripts/service.sh status

service-logs:
	@./scripts/service.sh logs

# ========== 备份监控 (v2.59.0) ==========
backup-monitor:
	@echo "💾 执行备份监控检查..."
	@./scripts/backup-monitor.sh

backup-monitor-status:
	@echo "💾 查看备份监控状态..."
	@./scripts/backup-monitor.sh --status

backup-monitor-daemon:
	@echo "💾 启动备份监控守护进程..."
	@./scripts/backup-monitor.sh --daemon

backup-verify:
	@echo "💾 验证最新备份..."
	@./scripts/backup-monitor.sh --verify

backup-report:
	@echo "💾 生成备份报告..."
	@./scripts/backup-monitor.sh --report

# ========== 磁盘健康检查 (v2.59.0) ==========
disk-health:
	@echo "💿 检查磁盘健康状态..."
	@./scripts/disk-health-check.sh

disk-health-json:
	@echo "💿 检查磁盘健康状态 (JSON)..."
	@./scripts/disk-health-check.sh --json

disk-health-monitor:
	@echo "💿 启动磁盘健康监控..."
	@./scripts/disk-health-check.sh --monitor

disk-smart-test:
	@echo "💿 运行 SMART 短测试..."
	@./scripts/disk-health-check.sh --test short

disk-smart-test-long:
	@echo "💿 运行 SMART 长测试..."
	@./scripts/disk-health-check.sh --test long

# ========== 日志分析 (v2.122.0) ==========
log-analyze:
	@echo "📋 分析日志文件..."
	@./scripts/log-analyzer.sh

log-analyze-json:
	@echo "📋 分析日志文件 (JSON)..."
	@./scripts/log-analyzer.sh --json

log-report:
	@echo "📋 生成日志报告..."
	@./scripts/log-analyzer.sh --report /var/log/nas-os/reports/log-report-$(shell date +%Y%m%d).txt

# ========== 版本管理 (v2.122.0) ==========

# 显示版本信息
version-info:
	@echo "NAS-OS 版本信息"
	@echo "================"
	@echo "版本:     $(VERSION)"
	@echo "提交:     $(GIT_COMMIT)"
	@echo "分支:     $(GIT_BRANCH)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Go版本:   $(GO_VERSION)"

# 版本 JSON 输出
version-info-json:
	@echo "{\"version\": \"$(VERSION)\", \"commit\": \"$(GIT_COMMIT)\", \"branch\": \"$(GIT_BRANCH)\", \"build_time\": \"$(BUILD_TIME)\", \"go_version\": \"$(GO_VERSION)\"}"

# 检查是否有未提交的更改
version-status:
	@echo "🔍 检查版本状态..."
	@if git diff-index --quiet HEAD --; then \
		echo "✅ 工作目录干净"; \
	else \
		echo "⚠️ 有未提交的更改"; \
		git status --short; \
	fi
	@echo ""
	@echo "当前版本: $(VERSION)"
	@echo "最近标签: $$(git describe --tags --abbrev=0 2>/dev/null || echo '无')"

# ========== 开发辅助 (v2.122.0) ==========

# 开发环境设置
dev-setup:
	@echo "🔧 设置开发环境..."
	@command -v golangci-lint >/dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@command -v swag >/dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	@command -v air >/dev/null || go install github.com/cosmtrek/air@latest
	@echo "✅ 开发工具安装完成"
	@echo ""
	@echo "已安装工具:"
	@echo "  - golangci-lint: 代码检查"
	@echo "  - swag: Swagger 文档生成"
	@echo "  - air: 热重载开发服务器"

# 热重载开发
dev:
	@echo "🔥 启动热重载开发服务器..."
	@if command -v air >/dev/null; then \
		air -c .air.toml; \
	else \
		echo "❌ 未安装 air，请运行 make dev-setup"; \
	fi

# 快速检查 (代码格式 + 简单 lint)
check: fmt lint
	@echo "✅ 快速检查完成"

# 预提交检查
pre-commit: tidy fmt lint test
	@echo "✅ 预提交检查完成"

# 显示依赖
deps:
	@echo "📦 项目依赖:"
	$(GO) list -m all | head -20

# 更新依赖
deps-update:
	@echo "📦 更新依赖..."
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "✅ 依赖已更新"

# 代码统计
stats:
	@echo "📊 代码统计:"
	@echo "  Go 文件: $$(find . -name '*.go' -not -path './vendor/*' | wc -l)"
	@echo "  代码行数: $$(find . -name '*.go' -not -path './vendor/*' -exec cat {} \; | wc -l)"
	@echo "  测试文件: $$(find . -name '*_test.go' -not -path './vendor/*' | wc -l)"
	@echo "  测试代码行数: $$(find . -name '*_test.go' -not -path './vendor/*' -exec cat {} \; | wc -l)"

# 查找 TODO/FIXME
todos:
	@echo "📝 待办事项:"
	@grep -rn --color=always 'TODO\|FIXME\|XXX\|HACK' --include='*.go' . 2>/dev/null | head -30 || echo "无待办事项"

# 快速构建 (跳过测试)
quick: build
	@echo "✅ 快速构建完成"

# 完整构建流程
all-checks: tidy fmt lint test
	@echo "✅ 所有检查通过"

# ========== CI/CD 辅助 (v2.123.0) ==========

# CI 环境检查
ci-check:
	@echo "🔍 CI 环境检查..."
	@./scripts/ci-helper.sh check

# CI 环境准备
ci-prep:
	@echo "🔧 CI 环境准备..."
	@./scripts/ci-helper.sh prep-test

# CI 缓存清理
ci-cache-clean:
	@echo "🧹 CI 缓存清理..."
	@./scripts/ci-helper.sh cache-clean

# CI 构建报告
ci-report:
	@echo "📊 CI 构建报告..."
	@./scripts/ci-helper.sh report --output ci-report.json

# 构建时间追踪
ci-timing:
	@echo "⏱️ 构建时间追踪..."
	@echo "构建开始时间: $(BUILD_TIME)"
	@echo "当前时间: $$(date -u '+%Y-%m-%d_%H:%M:%S')"

# 测试分片辅助（用于 CI）
test-shard:
	@echo "🧪 运行测试分片 $(SHARD_INDEX)/$(SHARD_TOTAL)..."
	@if [ -z "$(SHARD_INDEX)" ] || [ -z "$(SHARD_TOTAL)" ]; then \
		echo "❌ 请设置 SHARD_INDEX 和 SHARD_TOTAL 环境变量"; \
		exit 1; \
	fi
	$(GO) test -v -race -coverprofile=coverage-shard-$(SHARD_INDEX).out -covermode=atomic \
		$$(go list ./... | grep -v /tests/e2e | grep -v /tests/fixtures | grep -v /tests/reports | grep -v /tests/benchmark | grep -v /plugins/ | \
			awk "NR % $(SHARD_TOTAL) == $(SHARD_INDEX)")

# 合并测试分片覆盖率
test-shard-merge:
	@echo "📦 合并测试分片覆盖率..."
	@echo "mode: atomic" > coverage.out
	@for f in coverage-shard-*.out; do \
		if [ -f "$$f" ]; then \
			tail -n +2 "$$f" >> coverage.out; \
		fi; \
	done
	@echo "✅ 覆盖率合并完成: coverage.out"

# Docker 镜像构建（minimal 版本，distroless）
docker-minimal:
	@echo "🐳 构建 Docker 镜像 (minimal, distroless)..."
	docker build -f Dockerfile -t $(DOCKER_IMAGE):minimal .
	docker tag $(DOCKER_IMAGE):minimal $(DOCKER_IMAGE):$(DOCKER_TAG)

# Docker 镜像构建（full 版本，alpine）
docker-full:
	@echo "🐳 构建 Docker 镜像 (full, alpine)..."
	docker build -f Dockerfile.full -t $(DOCKER_IMAGE):full .

# Docker 多架构构建
docker-buildx-all:
	@echo "🐳 构建多架构 Docker 镜像 (minimal + full)..."
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -f Dockerfile -t $(DOCKER_IMAGE):minimal .
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -f Dockerfile.full -t $(DOCKER_IMAGE):full .

# 镜像大小检查
docker-size:
	@echo "📊 Docker 镜像大小..."
	@docker images $(DOCKER_IMAGE) --format "table {{.Tag}}\t{{.Size}}" 2>/dev/null || echo "镜像不存在"
