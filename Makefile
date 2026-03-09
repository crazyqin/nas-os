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

test-coverage:
	@echo "📊 生成覆盖率报告..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "📄 覆盖率报告：coverage.html"

test-race:
	@echo "🏃 竞态检测..."
	$(GO) test -race ./...

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
	@echo "    make test           - 运行测试"
	@echo "    make test-coverage  - 生成覆盖率报告"
	@echo "    make test-race      - 竞态检测"
	@echo ""
	@echo "  代码质量:"
	@echo "    make lint           - 代码检查"
	@echo "    make fmt            - 格式化代码"
	@echo "    make tidy           - 整理依赖"
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
	@echo "  其他:"
	@echo "    make clean          - 清理构建产物"
	@echo "    make help           - 显示帮助"
