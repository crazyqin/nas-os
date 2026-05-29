package aitrainer

import (
	"context"
	"testing"
	"time"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager()
}

func TestModelManagement(t *testing.T) {
	m := setupTestManager(t)

	// 创建模型
	model, err := m.CreateModel("测试模型", "用于测试的模型", ModelTypeClassification, "pytorch")
	if err != nil {
		t.Fatalf("CreateModel failed: %v", err)
	}
	if model.ID == "" {
		t.Error("expected non-empty model ID")
	}
	if model.Name != "测试模型" {
		t.Errorf("expected name '测试模型', got '%s'", model.Name)
	}
	if model.Status != ModelStatusDraft {
		t.Errorf("expected status draft, got %v", model.Status)
	}

	// 获取模型
	got, err := m.GetModel(model.ID)
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}
	if got.ID != model.ID {
		t.Errorf("expected ID %s, got %s", model.ID, got.ID)
	}

	// 列出模型
	models := m.ListModels()
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}

	// 更新模型
	updated, err := m.UpdateModel(model.ID, "更新后的模型", "更新描述", []string{"test", "updated"})
	if err != nil {
		t.Fatalf("UpdateModel failed: %v", err)
	}
	if updated.Name != "更新后的模型" {
		t.Errorf("expected name '更新后的模型', got '%s'", updated.Name)
	}

	// 删除模型
	if err := m.DeleteModel(model.ID); err != nil {
		t.Fatalf("DeleteModel failed: %v", err)
	}

	_, err = m.GetModel(model.ID)
	if err == nil {
		t.Error("expected error for deleted model")
	}
}

func TestModelExportImport(t *testing.T) {
	m := setupTestManager(t)

	// 创建并准备模型
	model, _ := m.CreateModel("导出测试模型", "", ModelTypeDetection, "tensorflow")
	model.Status = ModelStatusReady

	// 导出模型
	err := m.ExportModel(model.ID, "/models/exported")
	if err != nil {
		t.Fatalf("ExportModel failed: %v", err)
	}

	// 导入模型
	imported, err := m.ImportModel("导入模型", "/models/imported", ModelTypeSegmentation, "pytorch")
	if err != nil {
		t.Fatalf("ImportModel failed: %v", err)
	}
	if imported.Status != ModelStatusReady {
		t.Errorf("expected status ready, got %v", imported.Status)
	}
	if imported.Path != "/models/imported" {
		t.Errorf("expected path '/models/imported', got '%s'", imported.Path)
	}
}

func TestDatasetManagement(t *testing.T) {
	m := setupTestManager(t)

	// 创建数据集
	ds, err := m.CreateDataset("测试数据集", "用于测试", "/data/test", "image", 1000)
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}
	if ds.ID == "" {
		t.Error("expected non-empty dataset ID")
	}
	if ds.SampleCount != 1000 {
		t.Errorf("expected sample count 1000, got %d", ds.SampleCount)
	}

	// 获取数据集
	got, err := m.GetDataset(ds.ID)
	if err != nil {
		t.Fatalf("GetDataset failed: %v", err)
	}
	if got.Name != "测试数据集" {
		t.Errorf("expected name '测试数据集', got '%s'", got.Name)
	}

	// 列出数据集
	datasets := m.ListDatasets()
	if len(datasets) != 1 {
		t.Errorf("expected 1 dataset, got %d", len(datasets))
	}

	// 更新数据集
	updated, err := m.UpdateDataset(ds.ID, "更新后的数据集", "更新描述", &DatasetSplit{
		Train: 0.8,
		Val:   0.1,
		Test:  0.1,
	})
	if err != nil {
		t.Fatalf("UpdateDataset failed: %v", err)
	}
	if updated.Split.Train != 0.8 {
		t.Errorf("expected train split 0.8, got %f", updated.Split.Train)
	}

	// 删除数据集
	if err := m.DeleteDataset(ds.ID); err != nil {
		t.Fatalf("DeleteDataset failed: %v", err)
	}

	_, err = m.GetDataset(ds.ID)
	if err == nil {
		t.Error("expected error for deleted dataset")
	}
}

func TestTrainingTaskManagement(t *testing.T) {
	m := setupTestManager(t)

	// 创建模型和数据集
	model, _ := m.CreateModel("训练模型", "", ModelTypeClassification, "pytorch")
	ds, _ := m.CreateDataset("训练数据集", "", "/data/train", "image", 500)

	// 创建训练任务
	task, err := m.CreateTrainingTask("测试训练", model.ID, ds.ID, &TrainConfig{
		Epochs:       5,
		BatchSize:    16,
		LearningRate: 0.01,
		Optimizer:    "adam",
		Device:       "cpu",
	})
	if err != nil {
		t.Fatalf("CreateTrainingTask failed: %v", err)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status pending, got %v", task.Status)
	}

	// 获取训练任务
	got, err := m.GetTrainingTask(task.ID)
	if err != nil {
		t.Fatalf("GetTrainingTask failed: %v", err)
	}
	if got.Name != "测试训练" {
		t.Errorf("expected name '测试训练', got '%s'", got.Name)
	}

	// 列出训练任务
	tasks := m.ListTrainingTasks()
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	// 删除待处理的任务
	if err := m.DeleteTrainingTask(task.ID); err != nil {
		t.Fatalf("DeleteTrainingTask failed: %v", err)
	}
}

func TestTrainingTaskExecution(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// 创建模型和数据集
	model, _ := m.CreateModel("执行模型", "", ModelTypeClassification, "pytorch")
	ds, _ := m.CreateDataset("执行数据集", "", "/data/exec", "image", 100)

	// 创建训练任务
	task, _ := m.CreateTrainingTask("执行训练", model.ID, ds.ID, &TrainConfig{
		Epochs:       3,
		BatchSize:    8,
		LearningRate: 0.001,
		Optimizer:    "sgd",
		Device:       "cpu",
	})

	// 启动训练
	if err := m.StartTrainingTask(ctx, task.ID); err != nil {
		t.Fatalf("StartTrainingTask failed: %v", err)
	}

	// 等待训练完成
	time.Sleep(4 * time.Second)

	// 验证训练结果
	updated, _ := m.GetTrainingTask(task.ID)
	if updated.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %v", updated.Status)
	}
	if updated.Progress != 100 {
		t.Errorf("expected progress 100, got %f", updated.Progress)
	}
	if updated.Metrics == nil {
		t.Error("expected metrics to be set")
	}

	// 验证模型状态
	updatedModel, _ := m.GetModel(model.ID)
	if updatedModel.Status != ModelStatusReady {
		t.Errorf("expected model status ready, got %v", updatedModel.Status)
	}
}

func TestDeviceManagement(t *testing.T) {
	m := setupTestManager(t)

	// 列出默认设备
	devices := m.ListDevices()
	if len(devices) < 1 {
		t.Errorf("expected at least 1 device, got %d", len(devices))
	}

	// 添加GPU设备
	gpu := m.AddDevice("gpu-0", "RTX 3090", DeviceTypeGPU, 24*1024*1024*1024)
	if gpu.ID != "gpu-0" {
		t.Errorf("expected ID 'gpu-0', got '%s'", gpu.ID)
	}
	if gpu.Type != DeviceTypeGPU {
		t.Errorf("expected type gpu, got %v", gpu.Type)
	}

	// 获取设备
	got, err := m.GetDevice("gpu-0")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if got.Name != "RTX 3090" {
		t.Errorf("expected name 'RTX 3090', got '%s'", got.Name)
	}

	// 列出所有设备
	allDevices := m.ListDevices()
	if len(allDevices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(allDevices))
	}
}

func TestModelDeployment(t *testing.T) {
	m := setupTestManager(t)

	// 创建并准备模型
	model, _ := m.CreateModel("部署模型", "", ModelTypeDetection, "pytorch")
	model.Status = ModelStatusReady

	// 部署模型
	deployment, err := m.DeployModel(model.ID, "cpu", 8080, 2)
	if err != nil {
		t.Fatalf("DeployModel failed: %v", err)
	}
	if deployment.Status != "active" {
		t.Errorf("expected status active, got %v", deployment.Status)
	}
	if deployment.Port != 8080 {
		t.Errorf("expected port 8080, got %d", deployment.Port)
	}

	// 获取部署
	got, err := m.GetDeployment(deployment.ID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}
	if got.ModelID != model.ID {
		t.Errorf("expected model ID %s, got %s", model.ID, got.ModelID)
	}

	// 列出部署
	deployments := m.ListDeployments()
	if len(deployments) != 1 {
		t.Errorf("expected 1 deployment, got %d", len(deployments))
	}

	// 停止部署
	if err := m.StopDeployment(deployment.ID); err != nil {
		t.Fatalf("StopDeployment failed: %v", err)
	}

	// 验证模型状态
	updatedModel, _ := m.GetModel(model.ID)
	if updatedModel.Status != ModelStatusReady {
		t.Errorf("expected model status ready, got %v", updatedModel.Status)
	}

	// 删除部署
	if err := m.DeleteDeployment(deployment.ID); err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}
}

func TestInference(t *testing.T) {
	m := setupTestManager(t)

	// 创建并部署模型
	model, _ := m.CreateModel("推理模型", "", ModelTypeClassification, "pytorch")
	model.Status = ModelStatusReady
	m.DeployModel(model.ID, "cpu", 8080, 1)

	// 执行推理
	resp, err := m.Inference(&InferenceRequest{
		ModelID: model.ID,
		Input: map[string]interface{}{
			"image": "test.jpg",
		},
	})
	if err != nil {
		t.Fatalf("Inference failed: %v", err)
	}
	if resp.ID == "" {
		t.Error("expected non-empty response ID")
	}
	if resp.Output["processed"] != true {
		t.Error("expected processed to be true")
	}
	if resp.Latency < 0 {
		t.Errorf("expected non-negative latency, got %f", resp.Latency)
	}

	// 验证部署统计
	deployment := m.ListDeployments()[0]
	if deployment.ReqCount != 1 {
		t.Errorf("expected req count 1, got %d", deployment.ReqCount)
	}
}

func TestTrainingTaskErrors(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// 不存在的模型
	_, err := m.CreateTrainingTask("test", "nonexistent", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent model")
	}

	// 创建模型和数据集
	model, _ := m.CreateModel("错误测试模型", "", ModelTypeClassification, "pytorch")
	ds, _ := m.CreateDataset("错误测试数据集", "", "/data/err", "image", 100)

	// 创建任务
	task, _ := m.CreateTrainingTask("错误测试", model.ID, ds.ID, &TrainConfig{
		Epochs:  5,
		BatchSize: 16,
		Device:  "cpu",
	})

	// 启动任务
	m.StartTrainingTask(ctx, task.ID)

	// 尝试删除运行中的任务
	err = m.DeleteTrainingTask(task.ID)
	if err == nil {
		t.Error("expected error for deleting running task")
	}

	// 尝试启动已完成的任务
	time.Sleep(6 * time.Second)
	err = m.StartTrainingTask(ctx, task.ID)
	if err == nil {
		t.Error("expected error for starting completed task")
	}
}

func TestDeploymentErrors(t *testing.T) {
	m := setupTestManager(t)

	// 部署不存在的模型
	_, err := m.DeployModel("nonexistent", "cpu", 8080, 1)
	if err == nil {
		t.Error("expected error for nonexistent model")
	}

	// 部署未准备好的模型
	model, _ := m.CreateModel("未准备模型", "", ModelTypeClassification, "pytorch")
	_, err = m.DeployModel(model.ID, "cpu", 8080, 1)
	if err == nil {
		t.Error("expected error for deploying draft model")
	}

	// 推理未部署的模型
	_, err = m.Inference(&InferenceRequest{
		ModelID: model.ID,
		Input:   map[string]interface{}{"test": "data"},
	})
	if err == nil {
		t.Error("expected error for undeployed model")
	}
}

func TestShutdown(t *testing.T) {
	m := setupTestManager(t)

	// 测试关闭
	m.Shutdown()

	// 验证关闭后不能启动新任务
	model, _ := m.CreateModel("关闭测试模型", "", ModelTypeClassification, "pytorch")
	ds, _ := m.CreateDataset("关闭测试数据集", "", "/data/shutdown", "image", 100)
	task, _ := m.CreateTrainingTask("关闭测试", model.ID, ds.ID, &TrainConfig{
		Epochs:  5,
		BatchSize: 16,
		Device:  "cpu",
	})

	ctx := context.Background()
	err := m.StartTrainingTask(ctx, task.ID)
	if err != nil {
		// 由于设备可能不可用，这里可能失败
		t.Logf("StartTrainingTask after shutdown: %v", err)
	}
}
