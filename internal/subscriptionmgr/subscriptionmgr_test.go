package subscriptionmgr

import (
	"fmt"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	subs, err := m.ListSubscriptions()
	if err != nil {
		t.Fatalf("ListSubscriptions() error: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subs))
	}
}

func TestAddSubscription(t *testing.T) {
	m := NewManager()

	// 正常添加
	sub := Subscription{
		ID:          "sub-001",
		ServiceName: "AWS S3",
		Type:        TypeCloudStorage,
		Provider:    "AWS",
		Cost:        99.9,
		ExpiryDate:  time.Now().AddDate(0, 1, 0),
	}
	if err := m.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error: %v", err)
	}

	// 重复ID
	if err := m.AddSubscription(sub); err != ErrDuplicateID {
		t.Fatalf("expected ErrDuplicateID, got %v", err)
	}

	// 缺少必要字段
	if err := m.AddSubscription(Subscription{ID: "sub-x"}); err != ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
	if err := m.AddSubscription(Subscription{ServiceName: "test"}); err != ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestGetSubscription(t *testing.T) {
	m := NewManager()
	sub := Subscription{
		ID:          "sub-001",
		ServiceName: "Cloudflare CDN",
		Type:        TypeCDN,
		Cost:        20,
		ExpiryDate:  time.Now().AddDate(1, 0, 0),
	}
	_ = m.AddSubscription(sub)

	got, err := m.GetSubscription("sub-001")
	if err != nil {
		t.Fatalf("GetSubscription() error: %v", err)
	}
	if got.ServiceName != "Cloudflare CDN" {
		t.Fatalf("expected service name 'Cloudflare CDN', got '%s'", got.ServiceName)
	}

	_, err = m.GetSubscription("nonexistent")
	if err != ErrSubscriptionNotFound {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestListSubscriptions(t *testing.T) {
	m := NewManager()
	for i := 0; i < 5; i++ {
		_ = m.AddSubscription(Subscription{
			ID:          fmt.Sprintf("sub-%03d", i),
			ServiceName: fmt.Sprintf("Service %d", i),
			Type:        TypeOther,
			Cost:        float64(i * 10),
			ExpiryDate:  time.Now().AddDate(0, 3, 0),
		})
	}

	subs, err := m.ListSubscriptions()
	if err != nil {
		t.Fatalf("ListSubscriptions() error: %v", err)
	}
	if len(subs) != 5 {
		t.Fatalf("expected 5 subscriptions, got %d", len(subs))
	}
}

func TestUpdateSubscription(t *testing.T) {
	m := NewManager()
	_ = m.AddSubscription(Subscription{
		ID:          "sub-001",
		ServiceName: "Old Name",
		Type:        TypeDNS,
		Cost:        50,
		ExpiryDate:  time.Now().AddDate(0, 6, 0),
	})

	// 正常更新
	err := m.UpdateSubscription(Subscription{
		ID:          "sub-001",
		ServiceName: "New Name",
		Cost:        75,
	})
	if err != nil {
		t.Fatalf("UpdateSubscription() error: %v", err)
	}
	got, _ := m.GetSubscription("sub-001")
	if got.ServiceName != "New Name" {
		t.Fatalf("expected 'New Name', got '%s'", got.ServiceName)
	}
	if got.Cost != 75 {
		t.Fatalf("expected cost 75, got %f", got.Cost)
	}

	// 更新不存在的订阅
	err = m.UpdateSubscription(Subscription{ID: "nonexistent"})
	if err != ErrSubscriptionNotFound {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}

	// 缺少ID
	err = m.UpdateSubscription(Subscription{ServiceName: "test"})
	if err != ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestDeleteSubscription(t *testing.T) {
	m := NewManager()
	_ = m.AddSubscription(Subscription{
		ID:          "sub-001",
		ServiceName: "Test",
		Type:        TypeVPN,
		Cost:        30,
		ExpiryDate:  time.Now().AddDate(0, 1, 0),
	})

	err := m.DeleteSubscription("sub-001")
	if err != nil {
		t.Fatalf("DeleteSubscription() error: %v", err)
	}

	_, err = m.GetSubscription("sub-001")
	if err != ErrSubscriptionNotFound {
		t.Fatalf("expected ErrSubscriptionNotFound after delete, got %v", err)
	}

	err = m.DeleteSubscription("nonexistent")
	if err != ErrSubscriptionNotFound {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestGetCostSummary(t *testing.T) {
	m := NewManager()

	// 添加不同类型的订阅
	subs := []Subscription{
		{ID: "s1", ServiceName: "S3", Type: TypeCloudStorage, Cost: 120, BillingCycle: "monthly", ExpiryDate: time.Now().AddDate(0, 6, 0)},
		{ID: "s2", ServiceName: "CDN", Type: TypeCDN, Cost: 600, BillingCycle: "yearly", ExpiryDate: time.Now().AddDate(0, 6, 0)},
		{ID: "s3", ServiceName: "VPN", Type: TypeVPN, Cost: 30, BillingCycle: "monthly", ExpiryDate: time.Now().AddDate(0, 6, 0)},
		{ID: "s4", ServiceName: "Expired", Type: TypeDNS, Cost: 100, BillingCycle: "monthly", ExpiryDate: time.Now().AddDate(-1, 0, 0), Status: StatusExpired},
	}
	for _, s := range subs {
		_ = m.AddSubscription(s)
	}

	summary, err := m.GetCostSummary()
	if err != nil {
		t.Fatalf("GetCostSummary() error: %v", err)
	}

	// s1: 120/月, s2: 600/12=50/月, s3: 30/月 = 200/月
	if summary.MonthlyTotal != 200 {
		t.Fatalf("expected MonthlyTotal 200, got %f", summary.MonthlyTotal)
	}
	if summary.YearlyTotal != 2400 {
		t.Fatalf("expected YearlyTotal 2400, got %f", summary.YearlyTotal)
	}

	if len(summary.ByType) != 3 {
		t.Fatalf("expected 3 types, got %d", len(summary.ByType))
	}

	cs := summary.ByType[TypeCloudStorage]
	if cs.MonthlyCost != 120 {
		t.Fatalf("expected CloudStorage monthly 120, got %f", cs.MonthlyCost)
	}
	if cs.Count != 1 {
		t.Fatalf("expected CloudStorage count 1, got %d", cs.Count)
	}
}

func TestGetExpiringSubscriptions(t *testing.T) {
	m := NewManager()

	_ = m.AddSubscription(Subscription{
		ID:         "exp-soon",
		ServiceName: "Expiring Soon",
		Type:       TypeBackup,
		Cost:       50,
		ExpiryDate: time.Now().AddDate(0, 0, 5), // 5天后到期
	})
	_ = m.AddSubscription(Subscription{
		ID:         "exp-later",
		ServiceName: "Expiring Later",
		Type:       TypeBackup,
		Cost:       50,
		ExpiryDate: time.Now().AddDate(0, 3, 0), // 3个月后到期
	})

	// 7天内到期
	subs, err := m.GetExpiringSubscriptions(7)
	if err != nil {
		t.Fatalf("GetExpiringSubscriptions() error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 expiring subscription, got %d", len(subs))
	}
	if subs[0].ID != "exp-soon" {
		t.Fatalf("expected 'exp-soon', got '%s'", subs[0].ID)
	}

	// 无效参数
	_, err = m.GetExpiringSubscriptions(-1)
	if err != ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}

	// 365天内到期
	subs, err = m.GetExpiringSubscriptions(365)
	if err != nil {
		t.Fatalf("GetExpiringSubscriptions() error: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 expiring subscriptions, got %d", len(subs))
	}
}

func TestCalculateTotalStorageCapacity(t *testing.T) {
	m := NewManager()

	_ = m.AddSubscription(Subscription{
		ID:                "s1",
		ServiceName:       "S3",
		Type:              TypeCloudStorage,
		Cost:              100,
		StorageCapacityGB: 500,
		ExpiryDate:        time.Now().AddDate(0, 6, 0),
	})
	_ = m.AddSubscription(Subscription{
		ID:                "s2",
		ServiceName:       "GDrive",
		Type:              TypeCloudStorage,
		Cost:              50,
		StorageCapacityGB: 200,
		ExpiryDate:        time.Now().AddDate(0, 6, 0),
	})
	_ = m.AddSubscription(Subscription{
		ID:                "s3",
		ServiceName:       "Expired",
		Type:              TypeCloudStorage,
		Cost:              30,
		StorageCapacityGB: 1000,
		Status:            StatusExpired,
		ExpiryDate:        time.Now().AddDate(-1, 0, 0),
	})

	total, err := m.CalculateTotalStorageCapacity()
	if err != nil {
		t.Fatalf("CalculateTotalStorageCapacity() error: %v", err)
	}
	if total != 700 {
		t.Fatalf("expected 700 GB, got %d", total)
	}
}

func TestGetSubscriptionsByType(t *testing.T) {
	m := NewManager()

	_ = m.AddSubscription(Subscription{ID: "s1", ServiceName: "S3", Type: TypeCloudStorage, Cost: 100, ExpiryDate: time.Now().AddDate(0, 1, 0)})
	_ = m.AddSubscription(Subscription{ID: "s2", ServiceName: "GCS", Type: TypeCloudStorage, Cost: 80, ExpiryDate: time.Now().AddDate(0, 1, 0)})
	_ = m.AddSubscription(Subscription{ID: "s3", ServiceName: "CDN", Type: TypeCDN, Cost: 50, ExpiryDate: time.Now().AddDate(0, 1, 0)})

	subs, err := m.GetSubscriptionsByType(TypeCloudStorage)
	if err != nil {
		t.Fatalf("GetSubscriptionsByType() error: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 CloudStorage subscriptions, got %d", len(subs))
	}

	subs, err = m.GetSubscriptionsByType(TypeVPN)
	if err != nil {
		t.Fatalf("GetSubscriptionsByType() error: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 VPN subscriptions, got %d", len(subs))
	}
}

func TestConcurrency(t *testing.T) {
	m := NewManager()
	done := make(chan bool)

	// 并发写入
	for i := 0; i < 100; i++ {
		go func(idx int) {
			sub := Subscription{
				ID:          fmt.Sprintf("sub-%03d", idx),
				ServiceName: fmt.Sprintf("Service %d", idx),
				Type:        TypeOther,
				Cost:        float64(idx),
				ExpiryDate:  time.Now().AddDate(0, 1, 0),
			}
			_ = m.AddSubscription(sub)
			done <- true
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}

	subs, _ := m.ListSubscriptions()
	if len(subs) != 100 {
		t.Fatalf("expected 100 subscriptions after concurrent writes, got %d", len(subs))
	}

	// 并发读取
	for i := 0; i < 100; i++ {
		go func(idx int) {
			_, _ = m.GetSubscription(fmt.Sprintf("sub-%03d", idx))
			done <- true
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
