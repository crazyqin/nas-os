package stateful

import (
	"context"
	"testing"
	"time"
)

func TestNewStatefulFailoverManager(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir:         t.TempDir(),
		SnapshotInterval: 1 * time.Second,
		SyncInterval:     1 * time.Second,
		FailoverTimeout:  10 * time.Second,
	}

	mgr, err := NewStatefulFailoverManager(cfg)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	if mgr == nil {
		t.Fatal("管理器为空")
	}
	if mgr.localNode.NodeID != "node-1" {
		t.Errorf("期望本地节点ID为node-1，实际为%s", mgr.localNode.NodeID)
	}
}

func TestRegisterAndUnregisterSession(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		StateDir:    t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)

	session := &SessionState{
		SessionID: "sess-001",
		ClientIP:  "192.168.1.100",
		Username:  "testuser",
		ShareName: "share1",
		NodeID:    "node-1",
	}

	if err := mgr.RegisterSession(session); err != nil {
		t.Fatalf("注册会话失败: %v", err)
	}

	got := mgr.registry.Get("sess-001")
	if got == nil {
		t.Fatal("会话未找到")
	}
	if got.SessionID != "sess-001" {
		t.Errorf("期望SessionID=sess-001，实际为%s", got.SessionID)
	}

	mgr.UnregisterSession("sess-001")
	if mgr.registry.Get("sess-001") != nil {
		t.Error("会话应已删除")
	}
}

func TestFindBestTarget(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 5},
			{NodeID: "node-3", Address: "192.168.1.3", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)

	// 设置健康分数
	mgr.peerNodes["node-2"].HealthScore = 90
	mgr.peerNodes["node-3"].HealthScore = 80

	target := mgr.findBestTarget()
	if target == nil {
		t.Fatal("未找到目标节点")
	}
	// node-2: score=90+(100-5)=185, node-3: score=80+(100-10)=170
	if target.NodeID != "node-2" {
		t.Errorf("期望选择node-2，实际选择%s", target.NodeID)
	}
}

func TestGetStatus(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		LocalNodeID: "node-1",
		Peers: []PeerConfig{
			{NodeID: "node-2", Address: "192.168.1.2", Port: 9445, Priority: 10},
		},
		StateDir: t.TempDir(),
	}
	mgr, _ := NewStatefulFailoverManager(cfg)

	status := mgr.GetStatus()
	if status.ClusterName != "test-cluster" {
		t.Errorf("期望ClusterName=test-cluster，实际为%s", status.ClusterName)
	}
	if status.LocalNodeID != "node-1" {
		t.Errorf("期望LocalNodeID=node-1，实际为%s", status.LocalNodeID)
	}
	if _, ok := status.PeerStatuses["node-2"]; !ok {
		t.Error("缺少node-2的对等状态")
	}
}

func TestSessionStateRegistry(t *testing.T) {
	reg := NewSessionStateRegistry()

	sessions := []*SessionState{
		{SessionID: "s1", ClientIP: "10.0.0.1", NodeID: "n1", ShareName: "share1"},
		{SessionID: "s2", ClientIP: "10.0.0.2", NodeID: "n1", ShareName: "share2"},
		{SessionID: "s3", ClientIP: "10.0.0.1", NodeID: "n2", ShareName: "share1"},
	}

	for _, s := range sessions {
		reg.Add(s)
	}

	if reg.Size() != 3 {
		t.Errorf("期望Size=3，实际为%d", reg.Size())
	}

	node1Sessions := reg.GetByNode("n1")
	if len(node1Sessions) != 2 {
		t.Errorf("期望n1有2个会话，实际有%d个", len(node1Sessions))
	}

	clientSessions := reg.GetByClient("10.0.0.1")
	if len(clientSessions) != 2 {
		t.Errorf("期望10.0.0.1有2个会话，实际有%d个", len(clientSessions))
	}

	shareSessions := reg.GetByShare("share1")
	if len(shareSessions) != 2 {
		t.Errorf("期望share1有2个会话，实际有%d个", len(shareSessions))
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	reg := NewSessionStateRegistry()

	reg.Add(&SessionState{
		SessionID:    "s1",
		ClientIP:     "10.0.0.1",
		NodeID:       "n1",
		LastActivity: time.Now().Add(-2 * time.Hour),
	})
	reg.Add(&SessionState{
		SessionID:    "s2",
		ClientIP:     "10.0.0.2",
		NodeID:       "n1",
		LastActivity: time.Now(),
	})

	removed := reg.CleanupExpired(1 * time.Hour)
	if removed != 1 {
		t.Errorf("期望清理1个，实际清理%d个", removed)
	}
	if reg.Size() != 1 {
		t.Errorf("期望剩余1个，实际剩余%d个", reg.Size())
	}
}

func TestValidateSessionState(t *testing.T) {
	tests := []struct {
		name    string
		session *SessionState
		wantErr bool
	}{
		{
			name:    "空SessionID",
			session: &SessionState{ClientIP: "10.0.0.1", NodeID: "n1"},
			wantErr: true,
		},
		{
			name:    "空ClientIP",
			session: &SessionState{SessionID: "s1", NodeID: "n1"},
			wantErr: true,
		},
		{
			name:    "空NodeID",
			session: &SessionState{SessionID: "s1", ClientIP: "10.0.0.1"},
			wantErr: true,
		},
		{
			name:    "合法会话",
			session: &SessionState{SessionID: "s1", ClientIP: "10.0.0.1", NodeID: "n1"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionState(tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSessionState() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := &StatefulFailoverConfig{
		Enabled:          true,
		ClusterName:      "test-cluster",
		LocalNodeID:      "node-1",
		StateDir:         t.TempDir(),
		SnapshotInterval: 100 * time.Millisecond,
		SyncInterval:     100 * time.Millisecond,
		FailoverTimeout:  5 * time.Second,
	}
	mgr, _ := NewStatefulFailoverManager(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := mgr.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 注册一个会话
	mgr.RegisterSession(&SessionState{
		SessionID: "sess-test",
		ClientIP:  "192.168.1.100",
		NodeID:    "node-1",
	})

	// 等待至少一个快照周期
	time.Sleep(200 * time.Millisecond)

	if err := mgr.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}

	_ = ctx
}
