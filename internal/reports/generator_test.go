package reports

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========== ReportGenerator 测试 ==========

func TestNewReportGenerator(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, err := NewTemplateManager(dataDir)
	assert.NoError(t, err)

	rg := NewReportGenerator(tmplMgr, dataDir)

	assert.NotNil(t, rg)
	assert.NotNil(t, rg.DataSources)
	assert.NotNil(t, rg.customReports)
}

func TestReportGenerator_RegisterDataSource(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)

	mockDS := &MockDataSource{name: "test-source"}
	rg.RegisterDataSource(mockDS)

	assert.Contains(t, rg.DataSources, "test-source")
}

func TestReportGenerator_ListDataSources(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)

	// 空数据源
	assert.Empty(t, rg.ListDataSources())

	// 添加数据源
	rg.RegisterDataSource(&MockDataSource{name: "ds1"})
	rg.RegisterDataSource(&MockDataSource{name: "ds2"})

	sources := rg.ListDataSources()
	assert.Len(t, sources, 2)
	assert.Contains(t, sources, "ds1")
	assert.Contains(t, sources, "ds2")
}

func TestReportGenerator_CreateCustomReport(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)

	// 注册数据源
	rg.RegisterDataSource(&MockDataSource{name: "quota"})

	input := CustomReportInput{
		Name:        "测试报表",
		Description: "测试描述",
		DataSource:  "quota",
		Fields: []TemplateField{
			{Name: "user", Source: "username"},
			{Name: "usage", Source: "used_bytes"},
		},
	}

	report, err := rg.CreateCustomReport(input, "admin")

	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, "测试报表", report.Name)
	assert.Equal(t, "quota", report.DataSource)
	assert.Equal(t, "admin", report.CreatedBy)
}

func TestReportGenerator_CreateCustomReport_InvalidDataSource(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)

	input := CustomReportInput{
		Name:       "测试报表",
		DataSource: "nonexistent",
	}

	report, err := rg.CreateCustomReport(input, "admin")

	assert.Error(t, err)
	assert.Equal(t, ErrDataSourceNotFound, err)
	assert.Nil(t, report)
}

func TestReportGenerator_GetCustomReport(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)
	rg.RegisterDataSource(&MockDataSource{name: "quota"})

	// 创建报表
	input := CustomReportInput{
		Name:       "测试报表",
		DataSource: "quota",
	}
	created, _ := rg.CreateCustomReport(input, "admin")

	// 获取报表
	report, err := rg.GetCustomReport(created.ID)

	assert.NoError(t, err)
	assert.Equal(t, created.ID, report.ID)
}

func TestReportGenerator_GetCustomReport_NotFound(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)

	report, err := rg.GetCustomReport("nonexistent")

	assert.Error(t, err)
	assert.Equal(t, ErrReportNotFound, err)
	assert.Nil(t, report)
}

func TestReportGenerator_ListCustomReports(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)
	rg.RegisterDataSource(&MockDataSource{name: "quota"})
	rg.RegisterDataSource(&MockDataSource{name: "storage"})

	// 创建多个报表
	rg.CreateCustomReport(CustomReportInput{Name: "报表1", DataSource: "quota"}, "admin")
	rg.CreateCustomReport(CustomReportInput{Name: "报表2", DataSource: "quota"}, "admin")
	rg.CreateCustomReport(CustomReportInput{Name: "报表3", DataSource: "storage"}, "admin")

	// 列出所有报表
	all := rg.ListCustomReports("")
	assert.Len(t, all, 3)

	// 按数据源过滤
	quotaReports := rg.ListCustomReports("quota")
	assert.Len(t, quotaReports, 2)

	storageReports := rg.ListCustomReports("storage")
	assert.Len(t, storageReports, 1)
}

func TestReportGenerator_UpdateCustomReport(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)
	rg.RegisterDataSource(&MockDataSource{name: "quota"})

	// 创建报表
	created, _ := rg.CreateCustomReport(CustomReportInput{
		Name:       "原始名称",
		DataSource: "quota",
	}, "admin")

	// 更新报表
	updated, err := rg.UpdateCustomReport(created.ID, CustomReportInput{
		Name:        "新名称",
		Description: "新描述",
		DataSource:  "quota",
	})

	assert.NoError(t, err)
	assert.Equal(t, "新名称", updated.Name)
	assert.Equal(t, "新描述", updated.Description)
}

func TestReportGenerator_DeleteCustomReport(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)
	rg.RegisterDataSource(&MockDataSource{name: "quota"})

	// 创建报表
	created, _ := rg.CreateCustomReport(CustomReportInput{
		Name:       "测试报表",
		DataSource: "quota",
	}, "admin")

	// 删除报表
	err := rg.DeleteCustomReport(created.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = rg.GetCustomReport(created.ID)
	assert.Equal(t, ErrReportNotFound, err)
}

func TestReportGenerator_DeleteCustomReport_NotFound(t *testing.T) {
	dataDir := t.TempDir()
	tmplMgr, _ := NewTemplateManager(dataDir)
	rg := NewReportGenerator(tmplMgr, dataDir)

	err := rg.DeleteCustomReport("nonexistent")
	assert.Equal(t, ErrReportNotFound, err)
}

// ========== MockDataSource 测试用实现 ==========

type MockDataSource struct {
	name string
}

func (m *MockDataSource) Name() string {
	return m.name
}

func (m *MockDataSource) Query(
	query map[string]interface{},
	fields []TemplateField,
	filters []TemplateFilter,
	sorts []TemplateSort,
	aggregations []TemplateAggregation,
	groupBy []string,
	limit, offset int,
) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{"username": "user1", "used_bytes": 1024},
		{"username": "user2", "used_bytes": 2048},
	}, nil
}

func (m *MockDataSource) GetSummary(query map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"total_records": 2,
	}, nil
}

func (m *MockDataSource) GetAvailableFields() []TemplateField {
	return []TemplateField{
		{Name: "username", Source: "username", Type: FieldTypeString},
		{Name: "used_bytes", Source: "used_bytes", Type: FieldTypeBytes},
	}
}