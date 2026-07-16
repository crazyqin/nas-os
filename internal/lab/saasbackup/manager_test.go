package saasbackup

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 租户管理测试 ==========

func TestConnectTenant(t *testing.T) {
	mgr := NewManager()

	req := ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	}

	tenant, err := mgr.ConnectTenant(req)
	require.NoError(t, err)
	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, ProviderMicrosoft365, tenant.Provider)
	assert.Equal(t, "example.com", tenant.Domain)
	assert.Equal(t, "admin@example.com", tenant.AdminEmail)
	assert.Equal(t, TenantStatusConnected, tenant.Status)
}

func TestConnectTenantGoogle(t *testing.T) {
	mgr := NewManager()

	req := ConnectTenantRequest{
		Provider:   ProviderGoogleWorkspace,
		Domain:     "google.com",
		AdminEmail: "admin@google.com",
	}

	tenant, err := mgr.ConnectTenant(req)
	require.NoError(t, err)
	assert.Equal(t, ProviderGoogleWorkspace, tenant.Provider)
	assert.Equal(t, TenantStatusConnected, tenant.Status)
}

func TestListTenants(t *testing.T) {
	mgr := NewManager()

	mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example1.com",
		AdminEmail: "admin1@example1.com",
	})
	mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderGoogleWorkspace,
		Domain:     "example2.com",
		AdminEmail: "admin2@example2.com",
	})

	tenants := mgr.ListTenants()
	assert.Len(t, tenants, 2)
}

func TestListTenantsEmpty(t *testing.T) {
	mgr := NewManager()

	tenants := mgr.ListTenants()
	assert.Len(t, tenants, 0)
}

func TestDisconnectTenant(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	disconnectedID := tenant.ID

	err := mgr.DisconnectTenant(tenant.ID)
	assert.NoError(t, err)

	// 验证状态已更新
	tenants := mgr.ListTenants()
	for _, tenant := range tenants {
		if tenant.ID == disconnectedID {
			assert.Equal(t, TenantStatusDisconnected, tenant.Status)
		}
	}
}

func TestDisconnectTenantNotFound(t *testing.T) {
	mgr := NewManager()

	err := mgr.DisconnectTenant("nonexistent")
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

// ========== 备份任务测试 ==========

func TestCreateJob(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	req := CreateJobRequest{
		TenantID:      tenant.ID,
		UserID:        "user1",
		ResourceType:  ResourceMail,
		Schedule:      "0 2 * * *",
		RetentionDays: 30,
	}

	job, err := mgr.CreateJob(req)
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, tenant.ID, job.TenantID)
	assert.Equal(t, "user1", job.UserID)
	assert.Equal(t, ResourceMail, job.ResourceType)
	assert.Equal(t, JobStatusIdle, job.Status)
	assert.Equal(t, 30, job.RetentionDays)
}

func TestCreateJobDefaultRetention(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderGoogleWorkspace,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	req := CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceDrive,
	}

	job, err := mgr.CreateJob(req)
	require.NoError(t, err)
	assert.Equal(t, 30, job.RetentionDays) // 默认值
}

func TestCreateJobTenantNotFound(t *testing.T) {
	mgr := NewManager()

	req := CreateJobRequest{
		TenantID:     "nonexistent",
		UserID:       "user1",
		ResourceType: ResourceMail,
	}

	_, err := mgr.CreateJob(req)
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestCreateJobTenantDisconnected(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	// 断开租户
	mgr.DisconnectTenant(tenant.ID)

	req := CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	}

	_, err := mgr.CreateJob(req)
	assert.ErrorIs(t, err, ErrTenantNotConnected)
}

func TestListJobs(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})
	mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user2",
		ResourceType: ResourceDrive,
	})

	jobs := mgr.ListJobs()
	assert.Len(t, jobs, 2)
}

// ========== 执行备份测试 ==========

func TestExecuteBackup(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	job, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})

	result, err := mgr.ExecuteBackup(job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, result.Status)
	assert.NotNil(t, result.LastRun)
	assert.Greater(t, result.ItemCount, 0)
	assert.Greater(t, result.SizeBytes, int64(0))
}

func TestExecuteBackupJobNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.ExecuteBackup("nonexistent")
	assert.ErrorIs(t, err, ErrJobNotFound)
}

func TestExecuteBackupAlreadyRunning(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	job, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})

	// 手动设置为运行中
	mgr.mu.Lock()
	mgr.jobs[job.ID].Status = JobStatusRunning
	mgr.mu.Unlock()

	_, err := mgr.ExecuteBackup(job.ID)
	assert.ErrorIs(t, err, ErrJobAlreadyRunning)
}

// ========== 恢复数据测试 ==========

func TestRestoreDataOriginal(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	job, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})

	// 先执行备份生成数据
	result, _ := mgr.ExecuteBackup(job.ID)

	// 获取备份项
	items, _ := mgr.ListItems(result.ID)
	require.Greater(t, len(items), 0)

	// 恢复数据
	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}

	req := RestoreRequest{
		JobID:       result.ID,
		ItemIDs:     itemIDs,
		RestoreMode: RestoreModeOriginal,
	}

	restored, err := mgr.RestoreData(req)
	require.NoError(t, err)
	assert.Greater(t, len(restored), 0)
}

func TestRestoreDataCrossUser(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	job, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})

	result, _ := mgr.ExecuteBackup(job.ID)
	items, _ := mgr.ListItems(result.ID)
	require.Greater(t, len(items), 0)

	itemIDs := []string{items[0].ID}
	req := RestoreRequest{
		JobID:        result.ID,
		ItemIDs:      itemIDs,
		TargetUserID: "user2",
		RestoreMode:  RestoreModeCrossUser,
	}

	restored, err := mgr.RestoreData(req)
	require.NoError(t, err)
	assert.Len(t, restored, 1)
}

func TestRestoreDataCrossUserMissingTarget(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	job, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})

	result, _ := mgr.ExecuteBackup(job.ID)
	items, _ := mgr.ListItems(result.ID)
	require.Greater(t, len(items), 0)

	req := RestoreRequest{
		JobID:       result.ID,
		ItemIDs:     []string{items[0].ID},
		RestoreMode: RestoreModeCrossUser,
	}

	_, err := mgr.RestoreData(req)
	assert.ErrorIs(t, err, ErrCrossUserRequiresTarget)
}

func TestRestoreDataJobNotFound(t *testing.T) {
	mgr := NewManager()

	req := RestoreRequest{
		JobID:       "nonexistent",
		ItemIDs:     []string{"item1"},
		RestoreMode: RestoreModeOriginal,
	}

	_, err := mgr.RestoreData(req)
	assert.ErrorIs(t, err, ErrJobNotFound)
}

// ========== 统计和查询测试 ==========

func TestGetStats(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	// 创建多个任务并执行
	job1, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})
	job2, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user2",
		ResourceType: ResourceDrive,
	})

	mgr.ExecuteBackup(job1.ID)
	mgr.ExecuteBackup(job2.ID)

	stats := mgr.GetStats()
	assert.Equal(t, 2, stats.TotalJobs)
	assert.Greater(t, stats.TotalItems, 0)
	assert.Greater(t, stats.TotalSize, int64(0))
	assert.Greater(t, stats.SuccessRate, 0.0)
	assert.NotNil(t, stats.LastBackupTime)
}

func TestGetStatsEmpty(t *testing.T) {
	mgr := NewManager()

	stats := mgr.GetStats()
	assert.Equal(t, 0, stats.TotalJobs)
	assert.Equal(t, 0, stats.TotalItems)
	assert.Equal(t, int64(0), stats.TotalSize)
	assert.Equal(t, 0.0, stats.SuccessRate)
	assert.Nil(t, stats.LastBackupTime)
}

func TestListItems(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	job, _ := mgr.CreateJob(CreateJobRequest{
		TenantID:     tenant.ID,
		UserID:       "user1",
		ResourceType: ResourceMail,
	})

	result, _ := mgr.ExecuteBackup(job.ID)
	items, err := mgr.ListItems(result.ID)
	require.NoError(t, err)
	assert.Greater(t, len(items), 0)

	// 验证所有项都属于该任务
	for _, item := range items {
		assert.Equal(t, result.ID, item.JobID)
	}
}

func TestListItemsJobNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.ListItems("nonexistent")
	assert.ErrorIs(t, err, ErrJobNotFound)
}

// ========== 并发测试 ==========

func TestConcurrentConnectTenant(t *testing.T) {
	mgr := NewManager()

	var wg sync.WaitGroup
	providers := []SaaSProvider{ProviderMicrosoft365, ProviderGoogleWorkspace}
	domains := []string{"example1.com", "example2.com", "example3.com", "example4.com", "example5.com"}

	for i, domain := range domains {
		wg.Add(1)
		go func(d string, p SaaSProvider) {
			defer wg.Done()
			_, err := mgr.ConnectTenant(ConnectTenantRequest{
				Provider:   p,
				Domain:     d,
				AdminEmail: "admin@" + d,
			})
			assert.NoError(t, err)
		}(domain, providers[i%len(providers)])
	}

	wg.Wait()

	tenants := mgr.ListTenants()
	assert.Len(t, tenants, len(domains))
}

func TestConcurrentCreateJob(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	var wg sync.WaitGroup
	resourceTypes := []ResourceType{ResourceMail, ResourceDrive, ResourceContacts, ResourceCalendar}

	for _, rt := range resourceTypes {
		wg.Add(1)
		go func(r ResourceType) {
			defer wg.Done()
			_, err := mgr.CreateJob(CreateJobRequest{
				TenantID:     tenant.ID,
				UserID:       "user1",
				ResourceType: r,
			})
			assert.NoError(t, err)
		}(rt)
	}

	wg.Wait()

	jobs := mgr.ListJobs()
	assert.Len(t, jobs, len(resourceTypes))
}

func TestConcurrentExecuteBackup(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	// 创建多个任务
	var jobs []*BackupJob
	for i := 0; i < 5; i++ {
		job, _ := mgr.CreateJob(CreateJobRequest{
			TenantID:     tenant.ID,
			UserID:       "user1",
			ResourceType: ResourceMail,
		})
		jobs = append(jobs, job)
	}

	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j *BackupJob) {
			defer wg.Done()
			_, err := mgr.ExecuteBackup(j.ID)
			assert.NoError(t, err)
		}(job)
	}

	wg.Wait()

	// 验证所有任务都已完成
	for _, job := range jobs {
		updatedJob, _ := mgr.ListItems(job.ID)
		assert.NotNil(t, updatedJob)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	mgr := NewManager()

	tenant, _ := mgr.ConnectTenant(ConnectTenantRequest{
		Provider:   ProviderMicrosoft365,
		Domain:     "example.com",
		AdminEmail: "admin@example.com",
	})

	var wg sync.WaitGroup

	// 并发读写
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			mgr.ListTenants()
			mgr.ListJobs()
			mgr.GetStats()
		}()
		go func() {
			defer wg.Done()
			mgr.CreateJob(CreateJobRequest{
				TenantID:     tenant.ID,
				UserID:       "user1",
				ResourceType: ResourceMail,
			})
		}()
	}

	wg.Wait()

	// 验证没有 panic 或数据损坏
	jobs := mgr.ListJobs()
	assert.Len(t, jobs, 10)
}
