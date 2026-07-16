package selfserviceportal

import (
	"testing"
	"time"
)

func TestQuotaRequestFlow(t *testing.T) {
	portal := NewPortal()

	// 测试提交配额申请
	t.Run("SubmitQuotaRequest", func(t *testing.T) {
		req, err := portal.SubmitQuotaRequest("user1", 100, 50, "需要更多空间")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			t.Fatal("request should not be nil")
		}
		if req.UserID != "user1" {
			t.Errorf("expected user1, got %s", req.UserID)
		}
		// 50 < 100 * 50% = 50, 应该自动审批
		if req.Status != TicketApproved {
			t.Errorf("expected approved, got %s", req.Status)
		}
	})

	// 测试获取配额申请
	t.Run("GetQuotaRequest", func(t *testing.T) {
		reqs := portal.ListQuotaRequests("user1")
		if len(reqs) == 0 {
			t.Fatal("should have at least one request")
		}

		req, err := portal.GetQuotaRequest(reqs[0].ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.UserID != "user1" {
			t.Errorf("expected user1, got %s", req.UserID)
		}
	})

	// 测试手动审批流程
	t.Run("ManualApprovalFlow", func(t *testing.T) {
		// 提交一个不会自动审批的申请 (200 > 100 * 50%)
		req, err := portal.SubmitQuotaRequest("user2", 100, 200, "大容量需求")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Status != TicketPending {
			t.Errorf("expected pending, got %s", req.Status)
		}

		// 审批通过
		err = portal.ApproveQuotaRequest(req.ID, "admin1", "同意")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 验证状态
		req, _ = portal.GetQuotaRequest(req.ID)
		if req.Status != TicketApproved {
			t.Errorf("expected approved, got %s", req.Status)
		}

		// 验证审批记录
		approvals := portal.GetApprovals(req.ID)
		if len(approvals) == 0 {
			t.Fatal("should have approval record")
		}
		if approvals[0].ApproverID != "admin1" {
			t.Errorf("expected admin1, got %s", approvals[0].ApproverID)
		}
	})

	// 测试拒绝流程
	t.Run("RejectFlow", func(t *testing.T) {
		req, _ := portal.SubmitQuotaRequest("user3", 50, 100, "需要空间")
		// 等待状态变为 pending（如果不是自动审批）
		if req.Status == TicketPending {
			err := portal.RejectQuotaRequest(req.ID, "admin2", "不批准")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			req, _ = portal.GetQuotaRequest(req.ID)
			if req.Status != TicketRejected {
				t.Errorf("expected rejected, got %s", req.Status)
			}
		}
	})

	// 测试无效状态转换
	t.Run("InvalidStatusTransition", func(t *testing.T) {
		req, _ := portal.SubmitQuotaRequest("user4", 100, 300, "需要空间")
		if req.Status == TicketPending {
			portal.ApproveQuotaRequest(req.ID, "admin1", "")
			err := portal.ApproveQuotaRequest(req.ID, "admin1", "")
			if err != ErrInvalidStatus {
				t.Errorf("expected ErrInvalidStatus, got %v", err)
			}
		}
	})
}

func TestPermissionRequestFlow(t *testing.T) {
	portal := NewPortal()

	// 测试提交权限申请
	t.Run("SubmitPermissionRequest", func(t *testing.T) {
		expiresAt := time.Now().Add(24 * time.Hour)
		req, err := portal.SubmitPermissionRequest("user1", "/shared/docs", PermRead, true, &expiresAt, "临时读取")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Status != TicketPending {
			t.Errorf("expected pending, got %s", req.Status)
		}
		if !req.Temporary {
			t.Error("expected temporary to be true")
		}
	})

	// 测试审批流程
	t.Run("ApprovalFlow", func(t *testing.T) {
		req, _ := portal.SubmitPermissionRequest("user1", "/shared/data", PermWrite, false, nil, "写入权限")
		err := portal.ApprovePermissionRequest(req.ID, "admin1", "同意")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req, _ = portal.GetPermissionRequest(req.ID)
		if req.Status != TicketApproved {
			t.Errorf("expected approved, got %s", req.Status)
		}
	})

	// 测试列表功能
	t.Run("ListRequests", func(t *testing.T) {
		portal.SubmitPermissionRequest("user2", "/shared/test", PermRead, false, nil, "测试")
		portal.SubmitPermissionRequest("user2", "/shared/test2", PermWrite, false, nil, "测试2")

		requests := portal.ListPermissionRequests("user2")
		if len(requests) < 2 {
			t.Errorf("expected at least 2 requests, got %d", len(requests))
		}
	})
}

func TestBackupRestoreFlow(t *testing.T) {
	portal := NewPortal()

	// 测试创建恢复点
	t.Run("CreateRestorePoint", func(t *testing.T) {
		rp := portal.CreateRestorePoint("user1", "/home/user1/documents", 1024*1024)
		if rp == nil {
			t.Fatal("restore point should not be nil")
		}
		if rp.UserID != "user1" {
			t.Errorf("expected user1, got %s", rp.UserID)
		}
	})

	// 测试列表恢复点
	t.Run("ListRestorePoints", func(t *testing.T) {
		portal.CreateRestorePoint("user1", "/home/user1/photos", 2048*1024)
		points := portal.ListRestorePoints("user1")
		if len(points) < 2 {
			t.Errorf("expected at least 2 restore points, got %d", len(points))
		}
	})

	// 测试提交恢复请求
	t.Run("SubmitRestoreRequest", func(t *testing.T) {
		points := portal.ListRestorePoints("user1")
		if len(points) == 0 {
			t.Fatal("should have restore points")
		}

		req, err := portal.SubmitRestoreRequest("user1", points[0].ID, "/home/user1/restored")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Status != TicketPending {
			t.Errorf("expected pending, got %s", req.Status)
		}
	})

	// 测试审批恢复请求
	t.Run("ApproveRestoreRequest", func(t *testing.T) {
		points := portal.ListRestorePoints("user1")
		req, _ := portal.SubmitRestoreRequest("user1", points[0].ID, "/tmp/restore")

		err := portal.ApproveRestoreRequest(req.ID, "admin1", "同意恢复")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// 测试无效恢复点
	t.Run("InvalidRestorePoint", func(t *testing.T) {
		_, err := portal.SubmitRestoreRequest("user1", "nonexistent", "/tmp/restore")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestIssueTicketFlow(t *testing.T) {
	portal := NewPortal()

	// 测试提交工单
	t.Run("SubmitIssueTicket", func(t *testing.T) {
		ticket := portal.SubmitIssueTicket("user1", "无法访问共享", "无法访问 /shared/public 文件夹", "access", "high")
		if ticket == nil {
			t.Fatal("ticket should not be nil")
		}
		if ticket.Status != TicketPending {
			t.Errorf("expected pending, got %s", ticket.Status)
		}
		if ticket.Priority != "high" {
			t.Errorf("expected high, got %s", ticket.Priority)
		}
	})

	// 测试分配工单
	t.Run("AssignIssueTicket", func(t *testing.T) {
		ticket := portal.SubmitIssueTicket("user1", "磁盘错误", "看到磁盘错误日志", "hardware", "medium")
		err := portal.AssignIssueTicket(ticket.ID, "tech1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ticket, _ = portal.GetIssueTicket(ticket.ID)
		if ticket.AssignedTo != "tech1" {
			t.Errorf("expected tech1, got %s", ticket.AssignedTo)
		}
	})

	// 测试解决工单
	t.Run("ResolveIssueTicket", func(t *testing.T) {
		ticket := portal.SubmitIssueTicket("user1", "权限问题", "无法写入共享文件夹", "permission", "low")
		err := portal.ResolveIssueTicket(ticket.ID, "已修复权限配置")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ticket, _ = portal.GetIssueTicket(ticket.ID)
		if ticket.Status != TicketResolved {
			t.Errorf("expected resolved, got %s", ticket.Status)
		}
		if ticket.Resolution != "已修复权限配置" {
			t.Errorf("unexpected resolution: %s", ticket.Resolution)
		}
	})

	// 测试重复解决
	t.Run("DuplicateResolve", func(t *testing.T) {
		ticket := portal.SubmitIssueTicket("user2", "测试工单", "测试描述", "test", "low")
		portal.ResolveIssueTicket(ticket.ID, "已解决")

		err := portal.ResolveIssueTicket(ticket.ID, "再次解决")
		if err != ErrInvalidStatus {
			t.Errorf("expected ErrInvalidStatus, got %v", err)
		}
	})

	// 测试列表功能
	t.Run("ListTickets", func(t *testing.T) {
		portal.SubmitIssueTicket("user3", "工单1", "描述1", "cat1", "low")
		portal.SubmitIssueTicket("user3", "工单2", "描述2", "cat2", "medium")
		portal.SubmitIssueTicket("user3", "工单3", "描述3", "cat3", "high")

		tickets := portal.ListIssueTickets("user3")
		if len(tickets) < 3 {
			t.Errorf("expected at least 3 tickets, got %d", len(tickets))
		}
	})
}

func TestUserStats(t *testing.T) {
	portal := NewPortal()

	// 设置配额
	portal.SetUserQuota("user1", 100, 60)

	// 创建一些数据
	portal.SubmitIssueTicket("user1", "工单1", "描述", "cat", "low")
	portal.SubmitIssueTicket("user1", "工单2", "描述", "cat", "medium")
	portal.ResolveIssueTicket(portal.ListIssueTickets("user1")[0].ID, "已解决")
	portal.CreateRestorePoint("user1", "/path", 1024)

	stats := portal.GetUserStats("user1")
	if stats.TotalQuotaGB != 100 {
		t.Errorf("expected 100, got %d", stats.TotalQuotaGB)
	}
	if stats.UsedQuotaGB != 60 {
		t.Errorf("expected 60, got %d", stats.UsedQuotaGB)
	}
	if stats.UsagePercent != 60 {
		t.Errorf("expected 60, got %f", stats.UsagePercent)
	}
	if stats.RestorePoints != 1 {
		t.Errorf("expected 1, got %d", stats.RestorePoints)
	}
	if stats.ResolvedTickets != 1 {
		t.Errorf("expected 1 resolved, got %d", stats.ResolvedTickets)
	}
}

func TestNotifications(t *testing.T) {
	portal := NewPortal()

	// 创建一些操作来触发通知
	portal.SubmitQuotaRequest("user1", 100, 50, "测试")
	portal.SubmitIssueTicket("user1", "工单", "描述", "cat", "low")

	notifs := portal.ListNotifications("user1")
	if len(notifs) < 2 {
		t.Errorf("expected at least 2 notifications, got %d", len(notifs))
	}

	// 标记已读
	if len(notifs) > 0 {
		err := portal.MarkNotificationRead("user1", notifs[0].ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 验证已读状态
		notifs = portal.ListNotifications("user1")
		if !notifs[0].Read {
			t.Error("notification should be marked as read")
		}
	}
}

func TestAutoApprovalRules(t *testing.T) {
	portal := NewPortal()

	// 默认规则
	rules := portal.ListAutoApprovalRules()
	if len(rules) == 0 {
		t.Fatal("should have default rule")
	}

	// 添加新规则
	portal.AddAutoApprovalRule(&AutoApprovalRule{
		Name:                "Large request rule",
		Description:         "Auto approve if less than 20% of current",
		MaxAutoApproveGB:    50,
		MaxPercentOfCurrent: 20,
		Enabled:             true,
	})

	rules = portal.ListAutoApprovalRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestConcurrentAccess(t *testing.T) {
	portal := NewPortal()

	// 并发测试
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			userID := "user" + string(rune('0'+id))
			portal.SubmitQuotaRequest(userID, 100, 50, "并发测试")
			portal.SubmitIssueTicket(userID, "并发工单", "描述", "cat", "low")
			portal.GetUserStats(userID)
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据完整性
	for i := 0; i < 10; i++ {
		userID := "user" + string(rune('0'+i))
		stats := portal.GetUserStats(userID)
		if stats == nil {
			t.Errorf("stats should not be nil for %s", userID)
		}
	}
}

func TestNotFoundErrors(t *testing.T) {
	portal := NewPortal()

	// 测试不存在的配额申请
	_, err := portal.GetQuotaRequest("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 测试不存在的权限申请
	_, err = portal.GetPermissionRequest("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 测试不存在的工单
	_, err = portal.GetIssueTicket("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 测试审批不存在的申请
	err = portal.ApproveQuotaRequest("nonexistent", "admin", "")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// 测试标记不存在的通知
	err = portal.MarkNotificationRead("user1", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
