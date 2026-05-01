package s3gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestRouter 创建测试用的gin引擎和handler
func setupTestRouter() (*gin.Engine, *Gateway) {
	gw := NewGateway(GatewayConfig{
		StorageRoot:   "/tmp/s3-test",
		DefaultPolicy: PolicyPrivate,
		MaxObjectSize: 10 * 1024 * 1024, // 10MB
		EnableLogging: true,
		Region:        "us-east-1",
	})

	handler := NewHandler(gw)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return router, gw
}

// TestCreateAndListBuckets 测试创建和列出存储桶
func TestCreateAndListBuckets(t *testing.T) {
	router, _ := setupTestRouter()

	// 创建存储桶
	body := CreateBucketRequest{
		Name:   "my-test-bucket",
		Policy: PolicyPublic,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/s3/buckets?userId=user1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var bucket Bucket
	err := json.Unmarshal(w.Body.Bytes(), &bucket)
	require.NoError(t, err)
	assert.Equal(t, "my-test-bucket", bucket.Name)
	assert.Equal(t, "user1", bucket.OwnerID)
	assert.Equal(t, PolicyPublic, bucket.Policy)

	// 列出存储桶
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/s3/buckets?userId=user1", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["count"])
}

// TestPutAndGetObject 测试上传和下载对象
func TestPutAndGetObject(t *testing.T) {
	router, gw := setupTestRouter()

	// 先创建桶
	_, err := gw.CreateBucket("test-objects", "user1", PolicyPublic, BucketQuota{})
	require.NoError(t, err)

	// 上传对象
	objectData := []byte("hello, S3 gateway!")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/s3/buckets/test-objects/objects/hello.txt?userId=user1", bytes.NewReader(objectData))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var putResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &putResp)
	require.NoError(t, err)
	assert.Equal(t, "hello.txt", putResp["key"])

	// 下载对象
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/s3/buckets/test-objects/objects/hello.txt?userId=user1", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "hello, S3 gateway!", w2.Body.String())
	assert.Equal(t, "text/plain", w2.Header().Get("Content-Type"))
}

// TestDeleteObject 测试删除对象
func TestDeleteObject(t *testing.T) {
	router, gw := setupTestRouter()

	// 创建桶并上传对象
	_, err := gw.CreateBucket("del-test", "user1", PolicyPrivate, BucketQuota{})
	require.NoError(t, err)
	_, err = gw.PutObject("del-test", "file.dat", "user1", []byte("data"), "application/octet-stream", nil, nil)
	require.NoError(t, err)

	// 删除对象
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/s3/buckets/del-test/objects/file.dat?userId=user1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 再次获取应404
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/s3/buckets/del-test/objects/file.dat?userId=user1", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
}

// TestMultiTenantAccess 测试多租户隔离
func TestMultiTenantAccess(t *testing.T) {
	router, gw := setupTestRouter()

	// user1 创建桶并上传
	_, err := gw.CreateBucket("private-bucket", "user1", PolicyPrivate, BucketQuota{})
	require.NoError(t, err)
	_, err = gw.PutObject("private-bucket", "secret.txt", "user1", []byte("secret"), "text/plain", nil, nil)
	require.NoError(t, err)

	// user2 尝试读取 user1 的私有桶
	req := httptest.NewRequest(http.MethodGet, "/api/v1/s3/buckets/private-bucket/objects/secret.txt?userId=user2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code) // 访问被拒绝

	// user2 尝试列出 user1 的桶（应该看不到）
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/s3/buckets?userId=user2", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var resp map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["count"]) // user2 没有桶
}

// TestBucketQuota 测试桶配额限制
func TestBucketQuota(t *testing.T) {
	gw := NewGateway(GatewayConfig{
		EnableLogging: true,
		Region:        "us-east-1",
	})

	// 创建配额很小的桶（最大100字节，最多2个对象）
	_, err := gw.CreateBucket("quota-bucket", "user1", PolicyPrivate, BucketQuota{
		MaxSize:    100,
		MaxObjects: 2,
	})
	require.NoError(t, err)

	// 上传第一个对象
	_, err = gw.PutObject("quota-bucket", "obj1", "user1", []byte("data1"), "text/plain", nil, nil)
	require.NoError(t, err)

	// 上传第二个对象
	_, err = gw.PutObject("quota-bucket", "obj2", "user1", []byte("data2"), "text/plain", nil, nil)
	require.NoError(t, err)

	// 第三个应该失败（超过对象数量限制）
	_, err = gw.PutObject("quota-bucket", "obj3", "user1", []byte("data3"), "text/plain", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")
}

// TestListObjects 测试列出对象
func TestListObjects(t *testing.T) {
	router, gw := setupTestRouter()

	_, err := gw.CreateBucket("list-test", "user1", PolicyPublic, BucketQuota{})
	require.NoError(t, err)

	_, err = gw.PutObject("list-test", "dir/file1.txt", "user1", []byte("f1"), "text/plain", nil, nil)
	require.NoError(t, err)
	_, err = gw.PutObject("list-test", "dir/file2.txt", "user1", []byte("f2"), "text/plain", nil, nil)
	require.NoError(t, err)
	_, err = gw.PutObject("list-test", "other.txt", "user1", []byte("f3"), "text/plain", nil, nil)
	require.NoError(t, err)

	// 列出带前缀的对象
	req := httptest.NewRequest(http.MethodGet, "/api/v1/s3/buckets/list-test/objects?userId=user1&prefix=dir/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(2), resp["count"]) // 只有dir/前缀的2个
}

// TestStatsAndConfig 测试统计和配置接口
func TestStatsAndConfig(t *testing.T) {
	router, gw := setupTestRouter()

	_, err := gw.CreateBucket("stat-bucket", "user1", PolicyPrivate, BucketQuota{})
	require.NoError(t, err)
	_, err = gw.PutObject("stat-bucket", "obj1", "user1", []byte("hello"), "text/plain", nil, nil)
	require.NoError(t, err)

	// 获取统计
	req := httptest.NewRequest(http.MethodGet, "/api/v1/s3/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats TrafficStats
	err = json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalBuckets)
	assert.Equal(t, int64(1), stats.TotalObjects)
	assert.Equal(t, int64(5), stats.TotalSize)

	// 获取配置
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/s3/config", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "us-east-1")
}
