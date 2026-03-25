package cluster

import (
	"go.uber.org/zap"
)

// Services 集群服务集合.
type Services struct {
	Manager *Manager
	Sync    *StorageSync
	LB      *LoadBalancer
	HA      *HighAvailability
	API     *API
}

// RootConfig 集群总配置.
type RootConfig struct {
	Enabled      bool                `json:"enabled"`
	NodeID       string              `json:"node_id"`
	DataDir      string              `json:"data_dir"`
	Cluster      SimpleClusterConfig `json:"cluster"`
	Sync         SyncConfig          `json:"sync"`
	LoadBalancer LBConfig            `json:"load_balancer"`
	HA           HAConfig            `json:"ha"`
}

// InitializeCluster 初始化集群服务.
func InitializeCluster(config RootConfig, logger *zap.Logger) (*Services, error) {
	if !config.Enabled {
		logger.Info("集群功能已禁用")
		return nil, nil
	}

	logger.Info("初始化集群服务")

	// 初始化集群管理器
	clusterMgr, err := NewManager(SimpleClusterConfig{
		Name:              "nas-os-cluster",
		NodeID:            config.NodeID,
		DiscoveryPort:     8081,
		HeartbeatInterval: 5,
		HeartbeatTimeout:  15,
		DataDir:           config.DataDir + "/cluster",
	}, logger)
	if err != nil {
		return nil, err
	}

	if err := clusterMgr.Initialize(); err != nil {
		return nil, err
	}

	// 初始化存储同步
	syncMgr, err := NewStorageSync(SyncConfig{
		DataDir:    config.DataDir + "/sync",
		MaxRetries: 3,
		RetryDelay: 60,
	}, logger, clusterMgr)
	if err != nil {
		_ = clusterMgr.Shutdown()
		return nil, err
	}

	if err := syncMgr.Initialize(); err != nil {
		_ = clusterMgr.Shutdown()
		return nil, err
	}

	// 初始化负载均衡
	lb, err := NewLoadBalancer(LBConfig{
		Algorithm:      "round-robin",
		HealthCheckURL: "/api/v1/health",
		HealthInterval: 10,
		HealthTimeout:  5,
		MaxFailures:    3,
	}, logger, clusterMgr)
	if err != nil {
		_ = clusterMgr.Shutdown()
		_ = syncMgr.Shutdown()
		return nil, err
	}

	if err := lb.Initialize(); err != nil {
		_ = clusterMgr.Shutdown()
		_ = syncMgr.Shutdown()
		return nil, err
	}

	// 初始化高可用
	ha, err := NewHighAvailability(HAConfig{
		NodeID:           config.NodeID,
		DataDir:          config.DataDir + "/ha",
		BindPort:         8082,
		HeartbeatTimeout: 3000,
		ElectionTimeout:  5000,
	}, logger)
	if err != nil {
		_ = clusterMgr.Shutdown()
		_ = syncMgr.Shutdown()
		_ = lb.Shutdown()
		return nil, err
	}

	if err := ha.Initialize(); err != nil {
		_ = clusterMgr.Shutdown()
		_ = syncMgr.Shutdown()
		_ = lb.Shutdown()
		return nil, err
	}

	// 创建 API 处理器
	api := NewAPI(clusterMgr, syncMgr, lb, ha, logger)

	services := &Services{
		Manager: clusterMgr,
		Sync:    syncMgr,
		LB:      lb,
		HA:      ha,
		API:     api,
	}

	logger.Info("集群服务初始化完成")
	return services, nil
}

// ShutdownCluster 关闭集群服务.
func ShutdownCluster(services *Services) error {
	if services == nil {
		return nil
	}

	var lastErr error

	if services.HA != nil {
		if err := services.HA.Shutdown(); err != nil {
			lastErr = err
		}
	}

	if services.LB != nil {
		if err := services.LB.Shutdown(); err != nil {
			lastErr = err
		}
	}

	if services.Sync != nil {
		if err := services.Sync.Shutdown(); err != nil {
			lastErr = err
		}
	}

	if services.Manager != nil {
		if err := services.Manager.Shutdown(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
