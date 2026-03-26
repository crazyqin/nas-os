// Package s3 provides S3 object storage tests
package s3

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "my-bucket", false},
		{"valid with numbers", "bucket123", false},
		{"valid with dots", "my.bucket.name", false},
		{"too short", "ab", true},
		{"too long", "a1234567890123456789012345678901234567890123456789012345678901234", true},
		{"uppercase", "MyBucket", true},
		{"underscore", "my_bucket", true},
		{"spaces", "my bucket", true},
		{"start with xn--", "xn--bucket", true},
		{"end with -s3alias", "bucket-s3alias", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBucketName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBucketName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestManager_CreateBucket(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tests := []struct {
		name    string
		input   BucketInput
		wantErr bool
	}{
		{"valid bucket", BucketInput{Name: "test-bucket"}, false},
		{"duplicate bucket", BucketInput{Name: "test-bucket"}, true},
		{"invalid name", BucketInput{Name: "AB"}, true},
		{"with tags", BucketInput{Name: "tagged-bucket", Tags: map[string]string{"env": "test"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, err := m.CreateBucket(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBucket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && bucket == nil {
				t.Error("CreateBucket() returned nil bucket")
			}
		})
	}
}

func TestManager_GetBucket(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a bucket
	_, err = m.CreateBucket(BucketInput{Name: "test-bucket"})
	if err != nil {
		t.Fatalf("CreateBucket() error = %v", err)
	}

	tests := []struct {
		name       string
		bucketName string
		wantErr    bool
	}{
		{"existing bucket", "test-bucket", false},
		{"non-existing bucket", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, err := m.GetBucket(tt.bucketName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBucket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && bucket.Name != tt.bucketName {
				t.Errorf("GetBucket() name = %v, want %v", bucket.Name, tt.bucketName)
			}
		})
	}
}

func TestManager_ListBuckets(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Initially no buckets
	buckets := m.ListBuckets()
	if len(buckets) != 0 {
		t.Errorf("ListBuckets() returned %d buckets, want 0", len(buckets))
	}

	// Create buckets
	_, _ = m.CreateBucket(BucketInput{Name: "bucket-a"})
	_, _ = m.CreateBucket(BucketInput{Name: "bucket-b"})
	_, _ = m.CreateBucket(BucketInput{Name: "bucket-c"})

	buckets = m.ListBuckets()
	if len(buckets) != 3 {
		t.Errorf("ListBuckets() returned %d buckets, want 3", len(buckets))
	}

	// Verify sorted order
	if buckets[0].Name != "bucket-a" || buckets[1].Name != "bucket-b" || buckets[2].Name != "bucket-c" {
		t.Error("ListBuckets() not sorted correctly")
	}
}

func TestManager_DeleteBucket(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})

	// Delete existing bucket
	err = m.DeleteBucket("test-bucket")
	if err != nil {
		t.Errorf("DeleteBucket() error = %v", err)
	}

	// Verify deleted
	buckets := m.ListBuckets()
	if len(buckets) != 0 {
		t.Error("Bucket not deleted")
	}

	// Delete non-existing bucket
	err = m.DeleteBucket("nonexistent")
	if err != ErrBucketNotFound {
		t.Errorf("DeleteBucket() error = %v, want ErrBucketNotFound", err)
	}
}

func TestManager_PutObject(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})

	content := []byte("Hello, World!")
	reader := bytes.NewReader(content)

	obj, err := m.PutObject(context.Background(), "test-bucket", "test.txt", reader, int64(len(content)), "text/plain", nil)
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}

	if obj.Key != "test.txt" {
		t.Errorf("PutObject() key = %v, want test.txt", obj.Key)
	}
	if obj.Size != int64(len(content)) {
		t.Errorf("PutObject() size = %v, want %v", obj.Size, len(content))
	}
	if obj.ETag == "" {
		t.Error("PutObject() ETag is empty")
	}

	// Verify file exists
	objPath := filepath.Join(dataDir, "test-bucket", "test.txt")
	if _, err := os.Stat(objPath); os.IsNotExist(err) {
		t.Error("Object file not created")
	}
}

func TestManager_GetObject(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket and object
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})
	content := []byte("Hello, World!")
	_, _ = m.PutObject(context.Background(), "test-bucket", "test.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)

	// Get object
	reader, obj, err := m.GetObject(context.Background(), "test-bucket", "test.txt")
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	defer reader.Close()

	if obj.Key != "test.txt" {
		t.Errorf("GetObject() key = %v, want test.txt", obj.Key)
	}

	// Read content
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("GetObject() content mismatch")
	}

	// Get non-existing object
	_, _, err = m.GetObject(context.Background(), "test-bucket", "nonexistent.txt")
	if err != ErrObjectNotFound {
		t.Errorf("GetObject() error = %v, want ErrObjectNotFound", err)
	}
}

func TestManager_DeleteObject(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket and object
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})
	content := []byte("Hello, World!")
	_, _ = m.PutObject(context.Background(), "test-bucket", "test.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)

	// Delete object
	err = m.DeleteObject(context.Background(), "test-bucket", "test.txt")
	if err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}

	// Verify deleted
	_, _, err = m.GetObject(context.Background(), "test-bucket", "test.txt")
	if err != ErrObjectNotFound {
		t.Error("Object not deleted")
	}
}

func TestManager_ListObjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket and objects
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})
	_, _ = m.PutObject(context.Background(), "test-bucket", "file1.txt", bytes.NewReader([]byte("1")), 1, "text/plain", nil)
	_, _ = m.PutObject(context.Background(), "test-bucket", "file2.txt", bytes.NewReader([]byte("2")), 1, "text/plain", nil)
	_, _ = m.PutObject(context.Background(), "test-bucket", "folder/file3.txt", bytes.NewReader([]byte("3")), 1, "text/plain", nil)

	// List all objects
	result, err := m.ListObjects(context.Background(), "test-bucket", "", "", "", 100)
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}

	if len(result.Objects) != 3 {
		t.Errorf("ListObjects() returned %d objects, want 3", len(result.Objects))
	}

	// List with prefix
	result, err = m.ListObjects(context.Background(), "test-bucket", "folder/", "", "", 100)
	if err != nil {
		t.Fatalf("ListObjects() with prefix error = %v", err)
	}

	if len(result.Objects) != 1 {
		t.Errorf("ListObjects() with prefix returned %d objects, want 1", len(result.Objects))
	}

	// List with delimiter
	result, err = m.ListObjects(context.Background(), "test-bucket", "", "/", "", 100)
	if err != nil {
		t.Fatalf("ListObjects() with delimiter error = %v", err)
	}

	if len(result.CommonPrefixes) != 1 || result.CommonPrefixes[0] != "folder/" {
		t.Errorf("ListObjects() with delimiter common prefixes = %v, want [folder/]", result.CommonPrefixes)
	}
	if len(result.Objects) != 2 {
		t.Errorf("ListObjects() with delimiter returned %d objects, want 2", len(result.Objects))
	}
}

func TestManager_GeneratePresignedURL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})

	// Generate presigned URL
	url, err := m.GeneratePresignedURL(PresignedURLRequest{
		Bucket:   "test-bucket",
		Key:      "test.txt",
		Method:   "GET",
		Duration: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("GeneratePresignedURL() error = %v", err)
	}

	if url.URL == "" {
		t.Error("GeneratePresignedURL() URL is empty")
	}
	if url.Method != "GET" {
		t.Errorf("GeneratePresignedURL() method = %v, want GET", url.Method)
	}
}

func TestManager_MultipartUpload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})

	// Initiate multipart upload
	upload, err := m.InitiateMultipartUpload(context.Background(), "test-bucket", "large-file.bin", UploadConfig{Bucket: "test-bucket", Key: "large-file.bin"})
	if err != nil {
		t.Fatalf("InitiateMultipartUpload() error = %v", err)
	}

	if upload.UploadID == "" {
		t.Error("InitiateMultipartUpload() uploadID is empty")
	}

	// Upload parts
	part1 := []byte("Part 1 content")
	part2 := []byte("Part 2 content")

	_, err = m.UploadPart(context.Background(), upload.UploadID, 1, bytes.NewReader(part1), int64(len(part1)))
	if err != nil {
		t.Fatalf("UploadPart(1) error = %v", err)
	}

	_, err = m.UploadPart(context.Background(), upload.UploadID, 2, bytes.NewReader(part2), int64(len(part2)))
	if err != nil {
		t.Fatalf("UploadPart(2) error = %v", err)
	}

	// Complete multipart upload
	parts := []*CompletedPart{
		{PartNumber: 1, ETag: "etag1"},
		{PartNumber: 2, ETag: "etag2"},
	}

	obj, err := m.CompleteMultipartUpload(context.Background(), upload.UploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload() error = %v", err)
	}

	if obj.Key != "large-file.bin" {
		t.Errorf("CompleteMultipartUpload() key = %v, want large-file.bin", obj.Key)
	}
	if obj.Size != int64(len(part1)+len(part2)) {
		t.Errorf("CompleteMultipartUpload() size = %v, want %v", obj.Size, len(part1)+len(part2))
	}
}

func TestManager_CopyObject(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create buckets
	_, _ = m.CreateBucket(BucketInput{Name: "source-bucket"})
	_, _ = m.CreateBucket(BucketInput{Name: "dest-bucket"})

	// Create source object
	content := []byte("Hello, World!")
	_, _ = m.PutObject(context.Background(), "source-bucket", "source.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)

	// Copy object
	obj, err := m.CopyObject(context.Background(), "source-bucket", "source.txt", "dest-bucket", "dest.txt", nil)
	if err != nil {
		t.Fatalf("CopyObject() error = %v", err)
	}

	if obj.Bucket != "dest-bucket" {
		t.Errorf("CopyObject() bucket = %v, want dest-bucket", obj.Bucket)
	}
	if obj.Key != "dest.txt" {
		t.Errorf("CopyObject() key = %v, want dest.txt", obj.Key)
	}

	// Verify content
	reader, _, err := m.GetObject(context.Background(), "dest-bucket", "dest.txt")
	if err != nil {
		t.Fatalf("GetObject() after copy error = %v", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("CopyObject() content mismatch")
	}
}

func TestManager_BucketStats(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket and objects
	_, _ = m.CreateBucket(BucketInput{Name: "test-bucket"})
	_, _ = m.PutObject(context.Background(), "test-bucket", "file1.txt", bytes.NewReader([]byte("12345")), 5, "text/plain", nil)
	_, _ = m.PutObject(context.Background(), "test-bucket", "file2.txt", bytes.NewReader([]byte("1234567890")), 10, "text/plain", nil)

	stats, err := m.GetBucketStats("test-bucket")
	if err != nil {
		t.Fatalf("GetBucketStats() error = %v", err)
	}

	if stats.ObjectCount != 2 {
		t.Errorf("GetBucketStats() objectCount = %v, want 2", stats.ObjectCount)
	}
	if stats.Size != 15 {
		t.Errorf("GetBucketStats() size = %v, want 15", stats.Size)
	}
}

func TestManager_PersistConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	// Create manager and bucket
	m1, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, _ = m1.CreateBucket(BucketInput{Name: "persist-test"})
	content := []byte("test content")
	_, _ = m1.PutObject(context.Background(), "persist-test", "test.txt", bytes.NewReader(content), int64(len(content)), "text/plain", nil)

	// Create new manager to verify persistence
	m2, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() second instance error = %v", err)
	}

	// Verify bucket exists
	bucket, err := m2.GetBucket("persist-test")
	if err != nil {
		t.Fatalf("GetBucket() after reload error = %v", err)
	}

	if bucket.Name != "persist-test" {
		t.Errorf("Bucket name = %v, want persist-test", bucket.Name)
	}

	// Verify object exists
	reader, obj, err := m2.GetObject(context.Background(), "persist-test", "test.txt")
	if err != nil {
		t.Fatalf("GetObject() after reload error = %v", err)
	}
	defer reader.Close()

	if obj.Size != int64(len(content)) {
		t.Errorf("Object size = %v, want %v", obj.Size, len(content))
	}
}

func TestManager_Quota(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	dataDir := filepath.Join(tmpDir, "data")

	m, err := NewManager(configPath, dataDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create bucket with quota
	quotaSize := int64(100)
	_, _ = m.CreateBucket(BucketInput{
		Name: "quota-bucket",
		Quota: &QuotaConfig{
			Enabled: true,
			Size:    quotaSize,
		},
	})

	// Upload within quota
	content := []byte("1234567890") // 10 bytes
	_, err = m.PutObject(context.Background(), "quota-bucket", "file1.txt", bytes.NewReader(content), 10, "text/plain", nil)
	if err != nil {
		t.Fatalf("PutObject() within quota error = %v", err)
	}

	// Upload exceeding quota
	largeContent := make([]byte, 100)
	_, err = m.PutObject(context.Background(), "quota-bucket", "file2.txt", bytes.NewReader(largeContent), 100, "text/plain", nil)
	if err != ErrQuotaExceeded {
		t.Errorf("PutObject() exceeding quota error = %v, want ErrQuotaExceeded", err)
	}
}

func TestS3Error(t *testing.T) {
	err := ErrBucketNotFound
	if err.Code != 404 {
		t.Errorf("ErrBucketNotFound.Code = %v, want 404", err.Code)
	}

	xml := err.ToXML()
	if !bytes.Contains([]byte(xml), []byte("NoSuchBucket")) {
		t.Error("ToXML() missing error code")
	}
}