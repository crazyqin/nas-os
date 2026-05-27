package containerregistry

import (
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry(nil)
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if reg.config == nil {
		t.Fatal("default config not set")
	}
	if reg.config.MaxBlobSize != 1<<30 {
		t.Errorf("expected MaxBlobSize 1<<30, got %d", reg.config.MaxBlobSize)
	}
}

func TestPushAndPullImage(t *testing.T) {
	reg := NewRegistry(nil)

	layer := &Blob{
		Digest:    "sha256:abc123",
		Size:      1024,
		Path:      "/blobs/abc123",
		RefCount:  1,
		CreatedAt: time.Now(),
	}

	manifest := &Manifest{
		Digest:        "sha256:manifest123",
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: &Descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    "sha256:config123",
			Size:      256,
		},
		Layers: []*Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Digest:    layer.Digest,
				Size:      layer.Size,
			},
		},
		Size:      layer.Size + 256,
		CreatedAt: time.Now(),
	}

	// Push
	err := reg.PushImage("library/nginx", "latest", manifest, []*Blob{layer})
	if err != nil {
		t.Fatalf("PushImage failed: %v", err)
	}

	// Pull
	pulled, err := reg.PullImage("library/nginx", "latest")
	if err != nil {
		t.Fatalf("PullImage failed: %v", err)
	}
	if pulled.Digest != manifest.Digest {
		t.Errorf("expected digest %s, got %s", manifest.Digest, pulled.Digest)
	}

	// Stats
	stats := reg.GetStats()
	if stats.TotalPushes != 1 {
		t.Errorf("expected 1 push, got %d", stats.TotalPushes)
	}
	if stats.TotalPulls != 1 {
		t.Errorf("expected 1 pull, got %d", stats.TotalPulls)
	}
}

func TestListImages(t *testing.T) {
	reg := NewRegistry(nil)

	manifest := &Manifest{
		Digest:        "sha256:m1",
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Layers:        []*Descriptor{},
		Size:          0,
		CreatedAt:     time.Now(),
	}

	reg.PushImage("library/nginx", "latest", manifest, nil)
	reg.PushImage("library/redis", "latest", manifest, nil)

	images := reg.ListImages()
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

func TestDeleteTag(t *testing.T) {
	reg := NewRegistry(nil)

	manifest := &Manifest{
		Digest:        "sha256:m1",
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Layers:        []*Descriptor{},
		Size:          0,
		CreatedAt:     time.Now(),
	}

	reg.PushImage("library/nginx", "v1", manifest, nil)
	reg.PushImage("library/nginx", "v2", manifest, nil)

	err := reg.DeleteTag("library/nginx", "v1")
	if err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	tags, _ := reg.GetImageTags("library/nginx")
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
	if _, exists := tags["v2"]; !exists {
		t.Error("v2 tag should still exist")
	}
}

func TestGarbageCollection(t *testing.T) {
	reg := NewRegistry(nil)

	layer := &Blob{
		Digest: "sha256:orphan",
		Size:   2048,
		Path:   "/blobs/orphan",
	}
	reg.blobs[layer.Digest] = layer

	result := reg.GarbageCollection()
	if result.BlobsDeleted != 1 {
		t.Errorf("expected 1 blob deleted, got %d", result.BlobsDeleted)
	}
	if result.SpaceFreed != 2048 {
		t.Errorf("expected 2048 space freed, got %d", result.SpaceFreed)
	}
}

func TestSearchImages(t *testing.T) {
	reg := NewRegistry(nil)

	manifest := &Manifest{
		Digest:        "sha256:m1",
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Layers:        []*Descriptor{},
		Size:          0,
		CreatedAt:     time.Now(),
	}

	reg.PushImage("library/nginx", "latest", manifest, nil)
	reg.PushImage("library/redis", "latest", manifest, nil)

	results := reg.SearchImages("nginx")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "library/nginx" {
		t.Errorf("expected library/nginx, got %s", results[0].Name)
	}
}

func TestImageSizeLimit(t *testing.T) {
	config := &RegistryConfig{
		MaxImageSize: 100,
	}
	reg := NewRegistry(config)

	layer := &Blob{
		Digest: "sha256:big",
		Size:   200,
	}

	manifest := &Manifest{
		Digest:        "sha256:m1",
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Layers:        []*Descriptor{{Digest: layer.Digest, Size: layer.Size}},
		Size:          layer.Size,
		CreatedAt:     time.Now(),
	}

	err := reg.PushImage("library/big", "latest", manifest, []*Blob{layer})
	if err == nil {
		t.Error("expected error for oversized image")
	}
}

func TestPullNonExistent(t *testing.T) {
	reg := NewRegistry(nil)

	_, err := reg.PullImage("nonexistent", "latest")
	if err == nil {
		t.Error("expected error for non-existent image")
	}
}

func TestGenerateDigest(t *testing.T) {
	data := []byte("hello world")
	digest := GenerateDigest(data)
	if len(digest) != 71 { // "sha256:" + 64 hex chars
		t.Errorf("expected digest length 71, got %d", len(digest))
	}
}
