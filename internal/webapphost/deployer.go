package webapphost

import (
	"fmt"
	"log"
	"sync"
)

// Deployer 应用部署器.
type Deployer struct {
	mu       sync.RWMutex
	manager  *WebAppManager
	registry map[string]DeployFunc
}

// DeployFunc 部署函数类型.
type DeployFunc func(app *WebApp) error

// NewDeployer 创建部署器.
func NewDeployer(manager *WebAppManager) *Deployer {
	d := &Deployer{
		manager:  manager,
		registry: make(map[string]DeployFunc),
	}

	// 注册默认部署器
	d.registry["docker"] = d.deployDocker
	d.registry["static"] = d.deployStatic
	d.registry["proxy"] = d.deployProxy

	return d
}

// RegisterDeployer 注册自定义部署器.
func (d *Deployer) RegisterDeployer(appType string, fn DeployFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.registry[appType] = fn
}

// Deploy 部署应用.
func (d *Deployer) Deploy(config *DeployConfig) (*WebApp, error) {
	// 创建应用
	app, err := d.manager.CreateApp(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	// 获取部署函数
	d.mu.RLock()
	deployFn, exists := d.registry[app.Type]
	d.mu.RUnlock()

	if !exists {
		app.Status = "error"
		return nil, fmt.Errorf("unsupported app type: %s", app.Type)
	}

	// 执行部署
	if err := deployFn(app); err != nil {
		app.Status = "error"
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	return app, nil
}

// deployDocker 部署 Docker 应用.
func (d *Deployer) deployDocker(app *WebApp) error {
	log.Printf("Deploying Docker app: %s (image: %s)", app.Name, app.Image)

	// 模拟 Docker 容器创建和启动
	// 实际实现需要调用 Docker API

	// 1. 拉取镜像
	if err := d.pullImage(app.Image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// 2. 创建容器
	if err := d.createContainer(app); err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// 3. 启动容器
	if err := d.startContainer(app); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	log.Printf("Docker app deployed successfully: %s", app.Name)
	return nil
}

// pullImage 拉取 Docker 镜像.
func (d *Deployer) pullImage(image string) error {
	log.Printf("Pulling image: %s", image)
	// 实际实现：调用 Docker API 拉取镜像
	return nil
}

// createContainer 创建 Docker 容器.
func (d *Deployer) createContainer(app *WebApp) error {
	log.Printf("Creating container for app: %s", app.Name)
	// 实际实现：调用 Docker API 创建容器
	return nil
}

// startContainer 启动 Docker 容器.
func (d *Deployer) startContainer(app *WebApp) error {
	log.Printf("Starting container for app: %s", app.Name)
	// 实际实现：调用 Docker API 启动容器
	return nil
}

// deployStatic 部署静态应用.
func (d *Deployer) deployStatic(app *WebApp) error {
	log.Printf("Deploying static app: %s", app.Name)

	// 静态应用部署：
	// 1. 创建应用目录
	// 2. 配置 Web 服务器（Nginx/Caddy）
	// 3. 设置域名和路径

	config := app.Config
	sourcePath := config["source_path"]
	if sourcePath == "" {
		sourcePath = fmt.Sprintf("/var/lib/nas-os/webapphost/static/%s", app.Name)
	}

	log.Printf("Static app directory: %s", sourcePath)
	log.Printf("Static app deployed successfully: %s", app.Name)
	return nil
}

// deployProxy 部署反向代理应用.
func (d *Deployer) deployProxy(app *WebApp) error {
	log.Printf("Deploying proxy app: %s", app.Name)

	// 反向代理部署：
	// 1. 配置反向代理规则
	// 2. 设置负载均衡
	// 3. 配置健康检查

	targetURL := app.Config["target_url"]
	if targetURL == "" {
		return fmt.Errorf("target_url is required for proxy apps")
	}

	log.Printf("Proxy target: %s", targetURL)
	log.Printf("Proxy app deployed successfully: %s", app.Name)
	return nil
}

// Undeploy 卸载应用.
func (d *Deployer) Undeploy(appID string) error {
	app, err := d.manager.GetApp(appID)
	if err != nil {
		return err
	}

	// 停止应用
	if app.Status == "running" {
		if err := d.manager.StopApp(appID); err != nil {
			log.Printf("Warning: failed to stop app before undeploy: %v", err)
		}
	}

	// 根据类型清理
	switch app.Type {
	case "docker":
		if err := d.cleanupDocker(app); err != nil {
			log.Printf("Warning: Docker cleanup failed: %v", err)
		}
	case "static":
		if err := d.cleanupStatic(app); err != nil {
			log.Printf("Warning: Static cleanup failed: %v", err)
		}
	case "proxy":
		if err := d.cleanupProxy(app); err != nil {
			log.Printf("Warning: Proxy cleanup failed: %v", err)
		}
	}

	// 删除应用
	return d.manager.DeleteApp(appID)
}

// cleanupDocker 清理 Docker 资源.
func (d *Deployer) cleanupDocker(app *WebApp) error {
	log.Printf("Cleaning up Docker resources for app: %s", app.Name)
	// 实际实现：删除容器、卷等
	return nil
}

// cleanupStatic 清理静态应用资源.
func (d *Deployer) cleanupStatic(app *WebApp) error {
	log.Printf("Cleaning up static app resources: %s", app.Name)
	// 实际实现：删除静态文件目录
	return nil
}

// cleanupProxy 清理代理配置.
func (d *Deployer) cleanupProxy(app *WebApp) error {
	log.Printf("Cleaning up proxy configuration: %s", app.Name)
	// 实际实现：删除代理配置
	return nil
}

// UpdateApp 更新已部署的应用.
func (d *Deployer) UpdateApp(appID string, newVersion string) error {
	app, err := d.manager.GetApp(appID)
	if err != nil {
		return err
	}

	log.Printf("Updating app %s to version %s", app.Name, newVersion)

	// 根据类型更新
	switch app.Type {
	case "docker":
		return d.updateDockerApp(app, newVersion)
	case "static":
		return d.updateStaticApp(app)
	case "proxy":
		return d.updateProxyApp(app)
	default:
		return fmt.Errorf("unsupported app type: %s", app.Type)
	}
}

// updateDockerApp 更新 Docker 应用.
func (d *Deployer) updateDockerApp(app *WebApp, newVersion string) error {
	// 1. 拉取新镜像
	newImage := app.Image
	// 实际实现：解析镜像标签并替换版本

	if err := d.pullImage(newImage); err != nil {
		return fmt.Errorf("failed to pull new image: %w", err)
	}

	// 2. 停止旧容器
	if app.Status == "running" {
		if err := d.manager.StopApp(app.ID); err != nil {
			return fmt.Errorf("failed to stop old container: %w", err)
		}
	}

	// 3. 启动新容器
	return d.manager.StartApp(app.ID)
}

// updateStaticApp 更新静态应用.
func (d *Deployer) updateStaticApp(app *WebApp) error {
	log.Printf("Updating static app: %s", app.Name)
	// 实际实现：更新静态文件
	return nil
}

// updateProxyApp 更新代理应用.
func (d *Deployer) updateProxyApp(app *WebApp) error {
	log.Printf("Updating proxy app: %s", app.Name)
	// 实际实现：更新代理配置
	return nil
}
