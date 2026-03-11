# NAS-OS Makefile
# 构建、测试、部署自动化

.PHONY: all build test clean run docker help

# 变量
BINARY_NAME=nasd
CLI_NAME=nasctl
GO=go
GOFLAGS=-ldflags="-w -s"
DOCKER_IMAGE=nas-os
DOCKER_TAG=latest

# 默认目标
all: build

# ========== 构建 ==========
build:
	@echo "🔨 编译 nasd..."
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/nasd
	@echo "🔨 编译 nasctl..."
	$(GO) build $(GOFLAGS) -o $(CLI_NAME) ./cmd/nasctl
	@echo "✅ 构建完成"

build-debug:
	@echo "🔨 编译调试版本..."
	$(GO) build -race -o $(BINARY_NAME)-debug ./cmd/nasd

# 跨平台构建
build-all:
	@echo "🌍 构建多平台二进制..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/nasd
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/nasd
	GOOS=linux GOARCH=arm GOARM=7 $(GO) build $(GOFLAGS) -o $(BINARY_NAME)-linux-armv7 ./cmd/nasd
	@echo "✅ 多平台构建完成"

# ========== 测试 ==========
test:
	@echo "🧪 运行测试..."
	$(GO) test -v ./...

test-integration:
	@echo "🔗 运行集成测试..."
	$(GO) test -v ./tests/integration/...

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
	@echo "    make build-all      - 跨平台构建"
	@echo "    make build-debug    - 调试版本"
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
