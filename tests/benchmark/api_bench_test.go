// Package benchmark 提供 NAS-OS API 性能基准测试
// v2.16.0 API 基准测试覆盖
package benchmark

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nas-os/internal/api"
	"nas-os/internal/cache"
	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
)

// ========== 测试路由设置 ==========

func setupAPIBenchmarkRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 模拟缓存
	c := cache.NewLRUCache(1000, time.Minute)

	apiGroup := r.Group("/api/v1")
	{
		// 健康检查
		apiGroup.GET("/health", func(ctx *gin.Context) {
			api.OK(ctx, gin.H{
				"status":  "healthy",
				"version": "2.16.0",
				"uptime":  time.Since(time.Now()).Seconds(),
			})
		})

		// 系统信息
		apiGroup.GET("/system/info", func(ctx *gin.Context) {
			api.OK(ctx, gin.H{
				"hostname":  "nas-server",
				"version":   "2.16.0",
				"cpu_usage": 25.5,
				"mem_usage": 45.2,
				"uptime":    86400,
			})
		})

		// 存储卷列表
		apiGroup.GET("/storage/volumes", func(ctx *gin.Context) {
			volumes := []gin.H{
				{"name": "data", "size": 1000000000000, "used": 500000000000, "free": 500000000000},
				{"name": "backup", "size": 2000000000000, "used": 1000000000000, "free": 1000000000000},
				{"name": "media", "size": 4000000000000, "used": 3500000000000, "free": 500000000000},
			}
			api.OK(ctx, gin.H{"volumes": volumes, "total": len(volumes)})
		})

		// 存储卷详情
		apiGroup.GET("/storage/volumes/:name", func(ctx *gin.Context) {
			name := ctx.Param("name")
			// 检查缓存
			cacheKey := "volume_" + name
			if cached, ok := c.Get(cacheKey); ok {
				api.OK(ctx, cached)
				return
			}
			api.OK(ctx, gin.H{
				"name":         name,
				"uuid":         "test-uuid-" + name,
				"size":         1000000000000,
				"used":         500000000000,
				"free":         500000000000,
				"data_profile": "raid1",
				"status":       "healthy",
			})
		})

		// 文件列表
		apiGroup.GET("/files/list", func(ctx *gin.Context) {
			path := ctx.DefaultQuery("path", "/")
			// 检查缓存
			cacheKey := "filelist_" + path
			if cached, ok := c.Get(cacheKey); ok {
				api.OK(ctx, cached)
				return
			}

			files := make([]gin.H, 50)
			for i := 0; i < 50; i++ {
				files[i] = gin.H{
					"name":     "file-" + string(rune(i)),
					"path":     path + "/file-" + string(rune(i)),
					"size":     1024 * (i + 1),
					"is_dir":   i%5 == 0,
					"mod_time": time.Now().Add(-time.Duration(i) * time.Hour),
				}
			}
			api.OK(ctx, gin.H{"path": path, "files": files, "total": len(files)})
		})

		// 用户列表
		apiGroup.GET("/users", func(ctx *gin.Context) {
			users := []gin.H{
				{"id": 1, "username": "admin", "role": "admin"},
				{"id": 2, "username": "user1", "role": "user"},
				{"id": 3, "username": "user2", "role": "user"},
			}
			api.OK(ctx, gin.H{"users": users, "total": len(users)})
		})

		// 共享列表
		apiGroup.GET("/shares", func(ctx *gin.Context) {
			shares := []gin.H{
				{"id": "share1", "name": "Documents", "path": "/mnt/data/docs", "protocol": "smb"},
				{"id": "share2", "name": "Media", "path": "/mnt/data/media", "protocol": "nfs"},
			}
			api.OK(ctx, gin.H{"shares": shares, "total": len(shares)})
		})

		// 性能指标
		apiGroup.GET("/performance/metrics", func(ctx *gin.Context) {
			api.OK(ctx, gin.H{
				"iops":       gin.H{"read": 1000, "write": 500},
				"throughput": gin.H{"read": 104857600, "write": 52428800},
				"latency":    gin.H{"read": 5.2, "write": 8.5},
				"cache": gin.H{
					"hit_rate":  0.85,
					"hits":      8500,
					"misses":    1500,
					"evictions": 200,
				},
			})
		})

		// 搜索
		apiGroup.POST("/search", func(ctx *gin.Context) {
			var req struct {
				Query string `json:"query"`
				Type  string `json:"type"`
			}
			if err := ctx.ShouldBindJSON(&req); err != nil {
				api.BadRequest(ctx, "Invalid request")
				return
			}

			results := make([]gin.H, 20)
			for i := 0; i < 20; i++ {
				results[i] = gin.H{
					"name":    "result-" + string(rune(i)),
					"path":    "/path/to/result-" + string(rune(i)),
					"size":    1024 * (i + 1),
					"matched": req.Query,
				}
			}
			api.OK(ctx, gin.H{"query": req.Query, "results": results, "total": len(results)})
		})

		// 上传
		apiGroup.POST("/files/upload", func(ctx *gin.Context) {
			file, _, err := ctx.Request.FormFile("file")
			if err != nil {
				api.BadRequest(ctx, "No file uploaded")
				return
			}
			defer file.Close()

			// 读取文件内容（模拟处理）
			_, _ = io.Copy(io.Discard, file)

			api.OK(ctx, gin.H{
				"message": "File uploaded successfully",
				"path":    ctx.DefaultQuery("path", "/tmp"),
			})
		})

		// 创建卷
		apiGroup.POST("/storage/volumes", func(ctx *gin.Context) {
			var req struct {
				Name    string   `json:"name"`
				Devices []string `json:"devices"`
				Profile string   `json:"profile"`
			}
			if err := ctx.ShouldBindJSON(&req); err != nil {
				api.BadRequest(ctx, "Invalid request")
				return
			}

			api.OK(ctx, gin.H{
				"message": "Volume created",
				"volume": gin.H{
					"name":    req.Name,
					"devices": req.Devices,
					"profile": req.Profile,
				},
			})
		})
	}

	return r
}

// ========== API 响应时间基准测试 ==========

// BenchmarkAPI_Health 健康检查端点
func BenchmarkAPI_Health(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_SystemInfo 系统信息端点
func BenchmarkAPI_SystemInfo(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/system/info", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_StorageVolumes 存储卷列表端点
func BenchmarkAPI_StorageVolumes(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/storage/volumes", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_StorageVolumeDetail 存储卷详情端点
func BenchmarkAPI_StorageVolumeDetail(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/storage/volumes/data", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_FilesList 文件列表端点
func BenchmarkAPI_FilesList(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/files/list?path=/home", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_Users 用户列表端点
func BenchmarkAPI_Users(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/users", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_Shares 共享列表端点
func BenchmarkAPI_Shares(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/shares", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_PerformanceMetrics 性能指标端点
func BenchmarkAPI_PerformanceMetrics(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/performance/metrics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_Search 搜索端点
func BenchmarkAPI_Search(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := bytes.NewBufferString(`{"query":"test","type":"files"}`)
		req, _ := http.NewRequest("POST", "/api/v1/search", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_CreateVolume 创建卷端点
func BenchmarkAPI_CreateVolume(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := bytes.NewBufferString(`{"name":"test-vol","devices":["/dev/sda1","/dev/sdb1"],"profile":"raid1"}`)
		req, _ := http.NewRequest("POST", "/api/v1/storage/volumes", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// ========== 存储操作基准测试 ==========

// BenchmarkStorage_VolumeJSONEncode 卷 JSON 编码
func BenchmarkStorage_VolumeJSONEncode(b *testing.B) {
	vol := &storage.Volume{
		Name:        "data",
		UUID:        "test-uuid",
		Devices:     []string{"/dev/sda1", "/dev/sdb1"},
		Size:        2000000000000,
		Used:        1000000000000,
		Free:        1000000000000,
		DataProfile: "raid1",
		MetaProfile: "raid1",
		MountPoint:  "/mnt/data",
		Status:      storage.VolumeStatus{Healthy: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(vol)
	}
}

// BenchmarkStorage_VolumeJSONDecode 卷 JSON 解码
func BenchmarkStorage_VolumeJSONDecode(b *testing.B) {
	data := []byte(`{"name":"data","uuid":"test-uuid","devices":["/dev/sda1","/dev/sdb1"],"size":2000000000000,"used":1000000000000,"free":1000000000000,"data_profile":"raid1","meta_profile":"raid1","mount_point":"/mnt/data","status":{"healthy":true}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vol storage.Volume
		_ = json.Unmarshal(data, &vol)
	}
}

// BenchmarkStorage_VolumeListEncode 卷列表 JSON 编码
func BenchmarkStorage_VolumeListEncode(b *testing.B) {
	volumes := make([]*storage.Volume, 100)
	for i := 0; i < 100; i++ {
		volumes[i] = &storage.Volume{
			Name:        "vol-" + string(rune(i)),
			UUID:        "uuid-" + string(rune(i)),
			Size:        uint64(i+1) * 1000000000000,
			DataProfile: "raid1",
			Status:      storage.VolumeStatus{Healthy: true},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(volumes)
	}
}

// BenchmarkStorage_SubVolumeCreate 子卷创建
func BenchmarkStorage_SubVolumeCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.SubVolume{
			ID:       uint64(i % 10000),
			Name:     "subvol-" + string(rune(i%100)),
			Path:     "/mnt/data/subvol",
			ParentID: 5,
			ReadOnly: false,
			UUID:     "test-uuid",
		}
	}
}

// BenchmarkStorage_SnapshotCreate 快照创建
func BenchmarkStorage_SnapshotCreate(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Snapshot{
			Name:      "snap-" + string(rune(i%100)),
			Path:      "/mnt/data/.snapshots/snap",
			Source:    "subvol",
			ReadOnly:  true,
			CreatedAt: now,
		}
	}
}

// ========== 并发 API 基准测试 ==========

// BenchmarkAPI_ConcurrentHealth 并发健康检查
func BenchmarkAPI_ConcurrentHealth(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", "/api/v1/health", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkAPI_ConcurrentVolumes 并发卷列表
func BenchmarkAPI_ConcurrentVolumes(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", "/api/v1/storage/volumes", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkAPI_ConcurrentFilesList 并发文件列表
func BenchmarkAPI_ConcurrentFilesList(b *testing.B) {
	router := setupAPIBenchmarkRouter()
	paths := []string{"/", "/home", "/var", "/tmp", "/data"}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := paths[i%len(paths)]
			req, _ := http.NewRequest("GET", "/api/v1/files/list?path="+path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			i++
		}
	})
}

// BenchmarkAPI_ConcurrentSearch 并发搜索
func BenchmarkAPI_ConcurrentSearch(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			body := bytes.NewBufferString(`{"query":"test` + string(rune(i%100)) + `","type":"files"}`)
			req, _ := http.NewRequest("POST", "/api/v1/search", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			i++
		}
	})
}

// BenchmarkAPI_MixedWorkload 混合工作负载
func BenchmarkAPI_MixedWorkload(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			var req *http.Request
			switch i % 5 {
			case 0:
				req, _ = http.NewRequest("GET", "/api/v1/health", nil)
			case 1:
				req, _ = http.NewRequest("GET", "/api/v1/storage/volumes", nil)
			case 2:
				req, _ = http.NewRequest("GET", "/api/v1/files/list", nil)
			case 3:
				req, _ = http.NewRequest("GET", "/api/v1/performance/metrics", nil)
			case 4:
				body := bytes.NewBufferString(`{"query":"test","type":"files"}`)
				req, _ = http.NewRequest("POST", "/api/v1/search", body)
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			i++
		}
	})
}

// ========== 文件上传基准测试 ==========

// BenchmarkAPI_FileUpload_1KB 1KB 文件上传
func BenchmarkAPI_FileUpload_1KB(b *testing.B) {
	benchmarkFileUpload(b, 1024)
}

// BenchmarkAPI_FileUpload_10KB 10KB 文件上传
func BenchmarkAPI_FileUpload_10KB(b *testing.B) {
	benchmarkFileUpload(b, 10*1024)
}

// BenchmarkAPI_FileUpload_100KB 100KB 文件上传
func BenchmarkAPI_FileUpload_100KB(b *testing.B) {
	benchmarkFileUpload(b, 100*1024)
}

// BenchmarkAPI_FileUpload_1MB 1MB 文件上传
func BenchmarkAPI_FileUpload_1MB(b *testing.B) {
	benchmarkFileUpload(b, 1024*1024)
}

func benchmarkFileUpload(b *testing.B, size int) {
	router := setupAPIBenchmarkRouter()

	// 创建测试文件
	tmpFile, err := ioutil.TempFile("", "upload-*.dat")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	data := make([]byte, size)
	tmpFile.Write(data)
	tmpFile.Close()

	b.ResetTimer()
	b.ReportMetric(float64(size), "bytes")

	for i := 0; i < b.N; i++ {
		// 创建 multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		file, err := os.Open(tmpFile.Name())
		if err != nil {
			b.Fatal(err)
		}

		part, err := writer.CreateFormFile("file", filepath.Base(tmpFile.Name()))
		if err != nil {
			b.Fatal(err)
		}

		io.Copy(part, file)
		file.Close()
		writer.Close()

		req, _ := http.NewRequest("POST", "/api/v1/files/upload?path=/tmp", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// ========== 响应处理基准测试 ==========

// BenchmarkAPI_ResponseSuccess 成功响应构建
func BenchmarkAPI_ResponseSuccess(b *testing.B) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := gin.H{
		"status": "ok",
		"count":  100,
		"items":  []string{"a", "b", "c"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		api.OK(c, data)
	}
}

// BenchmarkAPI_ResponseError 错误响应构建
func BenchmarkAPI_ResponseError(b *testing.B) {
	response := api.Error(400, "Bad request")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = response
	}
}

// BenchmarkAPI_ResponsePageData 分页数据响应
func BenchmarkAPI_ResponsePageData(b *testing.B) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	items := make([]gin.H, 100)
	for i := 0; i < 100; i++ {
		items[i] = gin.H{"id": i, "name": "item-" + string(rune(i))}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		api.OK(c, gin.H{
			"items":       items,
			"total":       1000,
			"page":        1,
			"page_size":   100,
			"total_pages": 10,
		})
	}
}

// ========== 内存分配基准测试 ==========

// BenchmarkAPI_MemoryAllocation_HTTPRequest HTTP 请求内存分配
func BenchmarkAPI_MemoryAllocation_HTTPRequest(b *testing.B) {
	router := setupAPIBenchmarkRouter()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/storage/volumes", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkAPI_MemoryAllocation_JSONMarshal JSON 序列化内存分配
func BenchmarkAPI_MemoryAllocation_JSONMarshal(b *testing.B) {
	data := gin.H{
		"volumes": []gin.H{
			{"name": "data", "size": 1000000000000},
			{"name": "backup", "size": 2000000000000},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

// BenchmarkAPI_MemoryAllocation_Response 响应对象内存分配
func BenchmarkAPI_MemoryAllocation_Response(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = api.Success(gin.H{
			"message": "ok",
			"data":    []int{1, 2, 3, 4, 5},
		})
	}
}
