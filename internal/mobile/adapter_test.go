package mobile

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestNormalizePagination(t *testing.T) {
	a := NewAdapter()

	// 默认值
	l, o := a.NormalizePagination(0, 0)
	if l != 20 || o != 0 {
		t.Errorf("默认分页: 期望 limit=20 offset=0, 得到 limit=%d offset=%d", l, o)
	}

	// 上限限制
	l, o = a.NormalizePagination(200, 0)
	if l != 100 {
		t.Errorf("上限限制: 期望 limit=100, 得到 limit=%d", l)
	}

	// 负数偏移
	l, o = a.NormalizePagination(10, -5)
	if o != 0 {
		t.Errorf("负数偏移: 期望 offset=0, 得到 offset=%d", o)
	}

	// 正常值
	l, o = a.NormalizePagination(50, 10)
	if l != 50 || o != 10 {
		t.Errorf("正常值: 期望 limit=50 offset=10, 得到 limit=%d offset=%d", l, o)
	}
}

func TestBuildPaginatedResponse(t *testing.T) {
	a := NewAdapter()

	items := []string{"a", "b", "c"}
	resp := a.BuildPaginatedResponse(items, 100, 20, 0)

	if resp.Total != 100 {
		t.Errorf("Total: 期望 100, 得到 %d", resp.Total)
	}
	if !resp.HasMore {
		t.Error("HasMore 应为 true")
	}
	if resp.NextOffset != 20 {
		t.Errorf("NextOffset: 期望 20, 得到 %d", resp.NextOffset)
	}
	if resp.ServerTime == 0 {
		t.Error("ServerTime 不应为 0")
	}

	// 最后一页
	resp2 := a.BuildPaginatedResponse(items, 100, 20, 80)
	if resp2.HasMore {
		t.Error("最后一页 HasMore 应为 false")
	}
	if resp2.NextOffset != 0 {
		t.Errorf("最后一页 NextOffset: 期望 0, 得到 %d", resp2.NextOffset)
	}
}

func TestGenerateETag(t *testing.T) {
	a := NewAdapter()

	data := map[string]string{"key": "value"}
	etag := a.GenerateETag(data)

	if etag == "" {
		t.Error("ETag 不应为空")
	}

	// 相同数据应产生相同ETag
	etag2 := a.GenerateETag(data)
	if etag != etag2 {
		t.Error("相同数据应产生相同 ETag")
	}

	// 不同数据应产生不同ETag
	data2 := map[string]string{"key": "other"}
	etag3 := a.GenerateETag(data2)
	if etag == etag3 {
		t.Error("不同数据应产生不同 ETag")
	}
}

func TestRecordAndGetChanges(t *testing.T) {
	a := NewAdapter()

	deviceID := "test-device"
	before := time.Now()

	a.RecordChange(deviceID, "file", "file-1", "create", nil)
	a.RecordChange(deviceID, "file", "file-2", "update", nil)
	a.RecordChange(deviceID, "share", "share-1", "create", nil)

	// 获取所有变更
	changes := a.GetChanges(deviceID, "", before)
	if len(changes) != 3 {
		t.Errorf("期望 3 条变更, 得到 %d", len(changes))
	}

	// 按类型过滤
	changes = a.GetChanges(deviceID, "file", before)
	if len(changes) != 2 {
		t.Errorf("file 变更: 期望 2 条, 得到 %d", len(changes))
	}

	// 不存在的设备
	changes = a.GetChanges("nonexistent", "", before)
	if len(changes) != 0 {
		t.Errorf("不存在的设备: 期望 0 条, 得到 %d", len(changes))
	}
}

func TestSubmitOfflineActions(t *testing.T) {
	a := NewAdapter()

	actions := []OfflineAction{
		{ID: "1", Type: "create", Resource: "file", Timestamp: time.Now(), ClientID: "client-1"},
		{ID: "2", Type: "update", Resource: "share", Timestamp: time.Now(), ClientID: "client-1"},
		{ID: "", Type: "delete", Resource: "file", Timestamp: time.Now(), ClientID: "client-1"}, // 无效
	}

	results := a.SubmitOfflineActions(actions)

	if len(results) != 3 {
		t.Fatalf("期望 3 个结果, 得到 %d", len(results))
	}
	if !results[0].Success {
		t.Error("第一个操作应成功")
	}
	if results[2].Success {
		t.Error("第三个操作应失败（缺少ID）")
	}
}

func TestGetPendingActions(t *testing.T) {
	a := NewAdapter()

	actions := []OfflineAction{
		{ID: "1", Type: "create", Resource: "file", Timestamp: time.Now(), ClientID: "client-1"},
		{ID: "2", Type: "update", Resource: "share", Timestamp: time.Now(), ClientID: "client-1"},
		{ID: "3", Type: "create", Resource: "file", Timestamp: time.Now(), ClientID: "client-2"},
	}
	a.SubmitOfflineActions(actions)

	// client-1 的操作
	pending := a.GetPendingActions("client-1", 0)
	if len(pending) != 2 {
		t.Errorf("client-1: 期望 2 个待处理, 得到 %d", len(pending))
	}

	// 带限制
	pending = a.GetPendingActions("client-1", 1)
	if len(pending) != 1 {
		t.Errorf("client-1 with limit 1: 期望 1 个, 得到 %d", len(pending))
	}

	// client-2
	pending = a.GetPendingActions("client-2", 0)
	if len(pending) != 1 {
		t.Errorf("client-2: 期望 1 个, 得到 %d", len(pending))
	}
}

func TestSlimResponse(t *testing.T) {
	data := map[string]any{
		"name":  "test",
		"empty": "",
		"nil":   nil,
		"arr":   []any{},
		"sub": map[string]any{
			"keep":  "yes",
			"empty": "",
		},
	}

	result := SlimResponse(data)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("结果应为 map")
	}

	if _, exists := resultMap["empty"]; exists {
		t.Error("空字符串字段应被移除")
	}
	if _, exists := resultMap["nil"]; exists {
		t.Error("nil 字段应被移除")
	}
	if _, exists := resultMap["arr"]; exists {
		t.Error("空数组字段应被移除")
	}
	if resultMap["name"] != "test" {
		t.Error("非空字段应保留")
	}
}

func TestParsePagination(t *testing.T) {
	// 空参数
	r := &http.Request{}
	r.URL, _ = url.Parse("http://localhost")
	limit, offset := ParsePagination(r)
	if limit != 20 || offset != 0 {
		t.Errorf("默认值: 期望 20/0, 得到 %d/%d", limit, offset)
	}

	// 自定义参数
	r.URL, _ = url.Parse("http://localhost?limit=50&offset=10")
	limit, offset = ParsePagination(r)
	if limit != 50 || offset != 10 {
		t.Errorf("自定义: 期望 50/10, 得到 %d/%d", limit, offset)
	}

	// 超出上限
	r.URL, _ = url.Parse("http://localhost?limit=200")
	limit, _ = ParsePagination(r)
	if limit != 100 {
		t.Errorf("上限: 期望 100, 得到 %d", limit)
	}
}
