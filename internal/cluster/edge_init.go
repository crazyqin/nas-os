package cluster

import (
	"go.uber.org/zap"
)

// EdgeServices 边缘计算服务集合
type EdgeServices struct {
	NodeManager   *EdgeNodeManager
	TaskScheduler *TaskScheduler
	ResultAgg     *ResultAggregator
	LoadBalancer  *EdgeLoadBalancer
	API           *EdgeAPI
}

// EdgeRootConfig 边缘计算总配置
type EdgeRootConfig struct {
	Enabled       bool                `json:"enabled"`
	NodeID        string              `json:"node_id"`
	DataDir       string              `json:"data_dir"`
	NodeManager   EdgeNodeConfig      `json:"node_manager"`
	TaskScheduler TaskSchedulerConfig `json:"task_scheduler"`
	ResultAgg     ResultAggregatorConfig `json:"result_aggregator"`
	LoadBalancer  EdgeLBConfig        `json:"load_balancer"`
}

// InitializeEdgeComputing 初始化边缘计算服务
func InitializeEdgeComputing(config EdgeRootConfig, logger *zap.Logger, cluster *ClusterManager) (*EdgeServices, error) {
	if !config.Enabled {
		logger.Info("边缘计算功能已禁用")
		return nil, nil
	}

	logger.Info("初始化边缘计算服务")

	// 1. 初始化边缘节点管理器
	nodeConfig := config.NodeManager
	nodeConfig.DataDir = config.DataDir + "/nodes"
	nodeConfig.NodeID = config.NodeID

	nodeManager, err := NewEdgeNodeManager(nodeConfig, logger, cluster)
	if err != nil {
		return nil, err
	}

	if err := nodeManager.Initialize(); err != nil {
		return nil, err
	}

	// 2. 初始化任务调度器
	taskConfig := config.TaskScheduler
	taskConfig.DataDir = config.DataDir + "/tasks"

	taskScheduler, err := NewTaskScheduler(taskConfig, logger)
	if err != nil {
		nodeManager.Shutdown()
		return nil, err
	}

	if err := taskScheduler.Initialize(); err != nil {
		nodeManager.Shutdown()
		return nil, err
	}

	// 3. 初始化结果聚合器
	resultConfig := config.ResultAgg
	resultConfig.DataDir = config.DataDir + "/results"

	resultAgg, err := NewResultAggregator(resultConfig, logger)
	if err != nil {
		nodeManager.Shutdown()
		taskScheduler.Shutdown()
		return nil, err
	}

	if err := resultAgg.Initialize(); err != nil {
		nodeManager.Shutdown()
		taskScheduler.Shutdown()
		return nil, err
	}

	// 4. 初始化边缘负载均衡器
	edgeLB, err := NewEdgeLoadBalancer(config.LoadBalancer, nodeManager, logger)
	if err != nil {
		nodeManager.Shutdown()
		taskScheduler.Shutdown()
		resultAgg.Shutdown()
		return nil, err
	}

	if err := edgeLB.Initialize(); err != nil {
		nodeManager.Shutdown()
		taskScheduler.Shutdown()
		resultAgg.Shutdown()
		return nil, err
	}

	// 5. 设置模块间依赖
	nodeManager.SetTaskScheduler(taskScheduler)
	taskScheduler.SetEdgeManager(nodeManager)
	taskScheduler.SetResultAggregator(resultAgg)

	// 6. 创建 API 处理器
	api := NewEdgeAPI(nodeManager, taskScheduler, resultAgg, edgeLB, logger)

	services := &EdgeServices{
		NodeManager:   nodeManager,
		TaskScheduler: taskScheduler,
		ResultAgg:     resultAgg,
		LoadBalancer:  edgeLB,
		API:           api,
	}

	logger.Info("边缘计算服务初始化完成")
	return services, nil
}

// ShutdownEdgeComputing 关闭边缘计算服务
func ShutdownEdgeComputing(services *EdgeServices) error {
	if services == nil {
		return nil
	}

	var lastErr error

	if services.LoadBalancer != nil {
		if err := services.LoadBalancer.Shutdown(); err != nil {
			lastErr = err
		}
	}

	if services.ResultAgg != nil {
		if err := services.ResultAgg.Shutdown(); err != nil {
			lastErr = err
		}
	}

	if services.TaskScheduler != nil {
		if err := services.TaskScheduler.Shutdown(); err != nil {
			lastErr = err
		}
	}

	if services.NodeManager != nil {
		if err := services.NodeManager.Shutdown(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// DefaultEdgeConfig 默认边缘计算配置
func DefaultEdgeConfig() EdgeRootConfig {
	return EdgeRootConfig{
		Enabled: true,
		DataDir: "/var/lib/nas-os/edge",
		NodeManager: EdgeNodeConfig{
			HeartbeatInterval: 10e9, // 10秒
			HeartbeatTimeout:  30e9, // 30秒
			MaxNodes:         100,
			AutoRegister:     true,
		},
		TaskScheduler: TaskSchedulerConfig{
			MaxConcurrent:    100,
			TaskTimeout:      300,
			RetryAttempts:    3,
			ScheduleInterval: 5,
		},
		ResultAgg: ResultAggregatorConfig{
			MaxResults:     10000,
			Timeout:        300,
			ProcessWorkers: 4,
		},
		LoadBalancer: EdgeLBConfig{
			Strategy:          EdgeLBStrategyLeastLoad,
			HealthCheckInt:   10e9, // 10秒
			HealthTimeout:    5e9,  // 5秒
			MaxRetry:         3,
			StickySession:    false,
			LocationWeight:   0.3,
			ResourceWeight:   0.4,
			LatencyWeight:    0.2,
			CapabilityWeight: 0.1,
		},
	}
}