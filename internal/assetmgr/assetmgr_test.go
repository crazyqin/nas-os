package assetmgr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestManager() *Manager {
	m := NewManager()
	m.AddAsset(&Asset{
		ID:           "asset-01",
		Name:         "Web服务器-01",
		Type:         TypeServer,
		Status:       StatusOnline,
		IPAddress:    "192.168.1.100",
		MACAddress:   "AA:BB:CC:DD:EE:01",
		Location:     "机房A-机柜01",
		Department:   "技术部",
		Owner:        "运维组",
		SerialNumber: "SN-2024-001",
		Manufacturer: "Dell",
		Model:        "PowerEdge R740",
		PurchaseDate: time.Now().AddDate(-2, 0, 0),
		WarrantyEnd:  time.Now().AddDate(1, 0, 0),
		Hardware: &HardwareInfo{
			CPUModel:     "Intel Xeon Gold 6248R",
			CPUCores:     24,
			CPUGHz:       3.0,
			RAMGB:        128,
			RAMType:      "DDR4",
			StorageDisks: []DiskInfo{{Model: "Samsung 983 DCT", Type: "nvme", SizeGB: 960, Interface: "nvme"}},
			NetworkPorts: 4,
			ChassisType:  "2U",
		},
		Software: &SoftwareInfo{
			OSName:    "Ubuntu",
			OSVersion: "22.04 LTS",
			Hostname:  "web-01",
			Applications: []Application{
				{Name: "Nginx", Version: "1.24.0", Vendor: "Nginx Inc"},
				{Name: "PostgreSQL", Version: "15.4", Vendor: "PostgreSQL"},
			},
		},
		Tags: map[string]string{"env": "prod", "tier": "web"},
	})
	m.AddAsset(&Asset{
		ID:           "asset-02",
		Name:         "核心交换机-01",
		Type:         TypeSwitch,
		Status:       StatusOnline,
		IPAddress:    "192.168.1.1",
		Location:     "机房A-机柜01",
		Manufacturer: "Cisco",
		Model:        "Catalyst 9300",
		PurchaseDate: time.Now().AddDate(-6, 0, 0),
		WarrantyEnd:  time.Now().AddDate(-1, 0, 0),
		Tags:         map[string]string{"env": "prod"},
	})
	return m
}

func TestAddAndGetAsset(t *testing.T) {
	m := newTestManager()
	asset, err := m.GetAsset("asset-01")
	require.NoError(t, err)
	assert.Equal(t, "Web服务器-01", asset.Name)
	assert.Equal(t, TypeServer, asset.Type)
	assert.NotNil(t, asset.Hardware)
	assert.Equal(t, 24, asset.Hardware.CPUCores)
}

func TestAssetNotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.GetAsset("nonexistent")
	assert.ErrorIs(t, err, ErrAssetNotFound)
}

func TestUpdateAsset(t *testing.T) {
	m := newTestManager()
	asset, _ := m.GetAsset("asset-01")
	asset.Name = "Web服务器-01-更新"
	err := m.UpdateAsset(asset)
	require.NoError(t, err)
	updated, _ := m.GetAsset("asset-01")
	assert.Equal(t, "Web服务器-01-更新", updated.Name)
}

func TestDeleteAsset(t *testing.T) {
	m := newTestManager()
	err := m.DeleteAsset("asset-01")
	require.NoError(t, err)
	_, err = m.GetAsset("asset-01")
	assert.ErrorIs(t, err, ErrAssetNotFound)
}

func TestListAssets(t *testing.T) {
	m := newTestManager()
	// 列出所有
	all := m.ListAssets("", "")
	assert.Len(t, all, 2)

	// 按类型筛选
	servers := m.ListAssets(TypeServer, "")
	assert.Len(t, servers, 1)
	assert.Equal(t, "Web服务器-01", servers[0].Name)
}

func TestSearchAssets(t *testing.T) {
	m := newTestManager()
	// 按名称搜索
	results := m.SearchAssets("Web服务器")
	assert.Len(t, results, 1)

	// 按IP搜索
	results = m.SearchAssets("192.168.1.100")
	assert.Len(t, results, 1)

	// 按序列号搜索
	results = m.SearchAssets("SN-2024")
	assert.Len(t, results, 1)
}

func TestAssetGroups(t *testing.T) {
	m := newTestManager()
	err := m.CreateGroup(&AssetGroup{
		ID:          "group-prod",
		Name:        "生产环境",
		Description: "生产环境设备",
	})
	require.NoError(t, err)

	err = m.AddAssetToGroup("group-prod", "asset-01")
	require.NoError(t, err)

	group, err := m.GetGroup("group-prod")
	require.NoError(t, err)
	assert.Len(t, group.AssetIDs, 1)

	// 重复添加不会增加
	err = m.AddAssetToGroup("group-prod", "asset-01")
	require.NoError(t, err)
	group, _ = m.GetGroup("group-prod")
	assert.Len(t, group.AssetIDs, 1)

	// 移除
	err = m.RemoveAssetFromGroup("group-prod", "asset-01")
	require.NoError(t, err)
	group, _ = m.GetGroup("group-prod")
	assert.Len(t, group.AssetIDs, 0)

	// 删除分组
	err = m.DeleteGroup("group-prod")
	require.NoError(t, err)
}

func TestMaintenanceSchedule(t *testing.T) {
	m := newTestManager()
	err := m.CreateMaintenanceSchedule(&MaintenanceSchedule{
		ID:           "sched-01",
		Name:         "季度巡检",
		AssetIDs:     []string{"asset-01"},
		Description:  "每季度例行巡检",
		IntervalDays: 90,
		AssignedTo:   "运维组",
	})
	require.NoError(t, err)

	schedule, err := m.GetMaintenanceSchedule("sched-01")
	require.NoError(t, err)
	assert.Equal(t, "季度巡检", schedule.Name)

	// 记录维护
	err = m.RecordMaintenance("sched-01", time.Now())
	require.NoError(t, err)
	schedule, _ = m.GetMaintenanceSchedule("sched-01")
	assert.False(t, schedule.LastMaintenance.IsZero())

	// 列出
	schedules := m.ListMaintenanceSchedules()
	assert.Len(t, schedules, 1)
}

func TestUpcomingMaintenance(t *testing.T) {
	m := newTestManager()
	m.CreateMaintenanceSchedule(&MaintenanceSchedule{
		ID:              "sched-upcoming",
		Name:            "即将到期维护",
		IntervalDays:    7,
		LastMaintenance: time.Now().AddDate(0, 0, -5),
	})
	// 手动设置NextMaintenance
	s, _ := m.GetMaintenanceSchedule("sched-upcoming")
	s.NextMaintenance = time.Now().AddDate(0, 0, 2)

	upcoming := m.GetUpcomingMaintenance(7)
	assert.NotEmpty(t, upcoming)
}

func TestAgingAssets(t *testing.T) {
	m := newTestManager()
	aging := m.GetAgingAssets(1) // 超过1年
	assert.Len(t, aging, 2)      // 两个资产都超过1年
}

func TestExpiredWarranty(t *testing.T) {
	m := newTestManager()
	expired := m.GetExpiredWarranty()
	assert.Len(t, expired, 1) // 交换机保修已过期
	assert.Equal(t, "核心交换机-01", expired[0].Name)
}

func TestAssetSummary(t *testing.T) {
	m := newTestManager()
	summary := m.GetAssetSummary()
	assert.Equal(t, 2, summary["total"])
	assert.Equal(t, 0, summary["groups"])
}

func TestHardwareInventory(t *testing.T) {
	m := newTestManager()
	hw := m.ListHardwareInventory()
	assert.Len(t, hw, 1) // 只有服务器有硬件信息
}

func TestSoftwareInventory(t *testing.T) {
	m := newTestManager()
	sw := m.ListSoftwareInventory()
	assert.Len(t, sw, 1) // 只有服务器有软件信息
}

func TestInvalidInput(t *testing.T) {
	m := NewManager()
	err := m.AddAsset(&Asset{})
	assert.ErrorIs(t, err, ErrInvalidInput)

	err = m.CreateGroup(&AssetGroup{})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// ========== Handler 测试 ==========

func setupRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)
	m := newTestManager()
	h := NewHandlers(m)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r, h
}

func TestHandlerListAssets(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/assets", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Web服务器-01")
}

func TestHandlerGetAsset(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/assets/asset-01", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Web服务器-01")
}

func TestHandlerSearchAssets(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/assets/search?q=Web", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Web服务器-01")
}

func TestHandlerAddAsset(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	body := `{"id":"asset-new","name":"新服务器","type":"server","ip_address":"192.168.1.200"}`
	req, _ := http.NewRequest("POST", "/api/v1/assetmgr/assets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "新服务器")
}

func TestHandlerDeleteAsset(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/assetmgr/assets/asset-01", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "资产已删除")
}

func TestHandlerHardwareInventory(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/inventory/hardware", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total")
}

func TestHandlerSoftwareInventory(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/inventory/software", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total")
}

func TestHandlerListGroups(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/groups", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlerCreateGroup(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	body := `{"id":"group-new","name":"测试分组","description":"测试"}`
	req, _ := http.NewRequest("POST", "/api/v1/assetmgr/groups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "测试分组")
}

func TestHandlerSummary(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/summary", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total")
}

func TestHandlerAgingAssets(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/lifecycle/aging?years=1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "assets")
}

func TestHandlerExpiredWarranty(t *testing.T) {
	r, _ := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assetmgr/lifecycle/warranty-expired", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "核心交换机-01")
}
