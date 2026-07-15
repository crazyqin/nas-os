package deployorch

import (
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	o := NewOrchestrator()
	if o == nil {
		t.Fatal("NewOrchestrator 返回 nil")
	}
}

func TestAddRemoveNode(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{
		ID: "n1", Name: "node-1", Role: RoleMaster,
		Status: StatusOnline, Address: "192.168.1.10",
		SSHPort: 22, CPUCores: 8, MemoryGB: 16, StorageTB: 20,
	})

	if len(o.GetNodes()) != 1 {
		t.Errorf("节点数应为 1, 实际 %d", len(o.GetNodes()))
	}

	err := o.RemoveNode("n1")
	if err != nil {
		t.Fatalf("移除节点失败: %v", err)
	}
	if len(o.GetNodes()) != 0 {
		t.Errorf("移除后节点数应为 0, 实际 %d", len(o.GetNodes()))
	}
}

func TestHeartbeat(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{
		ID: "n1", Name: "node-1", Role: RoleWorker,
		Status: StatusOffline, Address: "192.168.1.10",
	})

	err := o.Heartbeat("n1")
	if err != nil {
		t.Fatalf("心跳失败: %v", err)
	}

	nodes := o.GetHealthyNodes()
	if len(nodes) != 1 {
		t.Errorf("健康节点应为 1, 实际 %d", len(nodes))
	}
}

func TestHealthyNodes(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{ID: "n1", Name: "node-1", Role: RoleWorker, Status: StatusOnline, Address: "192.168.1.10"})
	o.AddNode(&Node{ID: "n2", Name: "node-2", Role: RoleStorage, Status: StatusOnline, Address: "192.168.1.11"})
	o.AddNode(&Node{ID: "n3", Name: "node-3", Role: RoleEdge, Status: StatusOffline, Address: "192.168.1.12"})

	healthy := o.GetHealthyNodes()
	if len(healthy) != 2 {
		t.Errorf("健康节点应为 2, 实际 %d", len(healthy))
	}
}

func TestSaveTemplate(t *testing.T) {
	o := NewOrchestrator()
	o.SaveTemplate(&DeployTemplate{
		ID: "tmpl-1", Name: "web-server",
		Services: []*ServiceDef{
			{Name: "nginx", Image: "nginx:latest", Replicas: 2},
		},
	})

	if len(o.templates) != 1 {
		t.Errorf("模板数应为 1, 实际 %d", len(o.templates))
	}
}

func TestDeploy(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{
		ID: "n1", Name: "node-1", Role: RoleWorker,
		Status: StatusOnline, Address: "192.168.1.10",
	})
	o.SaveTemplate(&DeployTemplate{
		ID: "tmpl-1", Name: "web-server",
		Services: []*ServiceDef{
			{Name: "nginx", Image: "nginx:latest", Replicas: 2},
		},
	})

	dep, err := o.Deploy("tmpl-1", "n1")
	if err != nil {
		t.Fatalf("部署失败: %v", err)
	}
	if dep.Status != DeployRunning {
		t.Errorf("初始状态应为 running, 实际 %s", dep.Status)
	}

	time.Sleep(2 * time.Second)

	updated, _ := o.GetDeployment(dep.ID)
	if updated.Status != DeploySuccess {
		t.Errorf("异步部署后状态应为 success, 实际 %s", updated.Status)
	}
}

func TestDeployInvalidTemplate(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{ID: "n1", Name: "node-1", Status: StatusOnline, Address: "192.168.1.10"})

	_, err := o.Deploy("invalid", "n1")
	if err == nil {
		t.Error("无效模板应返回错误")
	}
}

func TestDeployOfflineNode(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{ID: "n1", Name: "node-1", Status: StatusOffline, Address: "192.168.1.10"})
	o.SaveTemplate(&DeployTemplate{ID: "tmpl-1", Name: "test"})

	_, err := o.Deploy("tmpl-1", "n1")
	if err == nil {
		t.Error("离线节点应返回错误")
	}
}

func TestRollback(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{ID: "n1", Name: "node-1", Status: StatusOnline, Address: "192.168.1.10"})
	o.SaveTemplate(&DeployTemplate{
		ID: "tmpl-1", Name: "test",
		Services: []*ServiceDef{{Name: "nginx", Replicas: 1}},
	})

	dep, _ := o.Deploy("tmpl-1", "n1")
	err := o.Rollback(dep.ID)
	if err != nil {
		t.Fatalf("回滚失败: %v", err)
	}

	time.Sleep(1 * time.Second)
	updated, _ := o.GetDeployment(dep.ID)
	if updated.Status != DeployCancelled {
		t.Errorf("回滚后状态应为 cancelled, 实际 %s", updated.Status)
	}
}

func TestListDeployments(t *testing.T) {
	o := NewOrchestrator()
	o.AddNode(&Node{ID: "n1", Name: "node-1", Status: StatusOnline, Address: "192.168.1.10"})
	o.SaveTemplate(&DeployTemplate{ID: "tmpl-1", Name: "test"})

	o.Deploy("tmpl-1", "n1")
	o.Deploy("tmpl-1", "n1")

	deps := o.ListDeployments()
	if len(deps) != 2 {
		t.Errorf("部署应为 2, 实际 %d", len(deps))
	}
}

func TestFormatNodeList(t *testing.T) {
	o := NewOrchestrator()
	output := o.FormatNodeList()
	if output != "无注册节点" {
		t.Error("无节点时应有提示")
	}

	o.AddNode(&Node{
		ID: "n1", Name: "node-1", Role: RoleMaster,
		Status: StatusOnline, Address: "192.168.1.10",
		CPUCores: 8, MemoryGB: 16, StorageTB: 20,
	})
	output = o.FormatNodeList()
	if output == "无注册节点" {
		t.Error("有节点时不应返回空提示")
	}
}
