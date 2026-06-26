package clientthumb

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine_DefaultConfig(t *testing.T) {
	e := NewEngine(nil)
	if e.config.DefaultFormat != FormatWebP {
		t.Errorf("expected default format WebP, got %s", e.config.DefaultFormat)
	}
	if e.config.DefaultQuality != 80 {
		t.Errorf("expected default quality 80, got %d", e.config.DefaultQuality)
	}
	if e.config.DefaultSize != SizeMedium {
		t.Errorf("expected default size medium, got %s", e.config.DefaultSize)
	}
	if !e.config.EnablePerfTrack {
		t.Error("expected EnablePerfTrack=true")
	}
}

func TestNewEngine_CustomConfig(t *testing.T) {
	cfg := &EngineConfig{
		DefaultFormat:   FormatAVIF,
		DefaultQuality:  90,
		DefaultSize:     SizeLarge,
		MaxClientTasks:  8,
		ClientTimeout:   60 * time.Second,
		EnablePerfTrack: false,
	}
	e := NewEngine(cfg)
	if e.config.DefaultFormat != FormatAVIF {
		t.Errorf("expected AVIF, got %s", e.config.DefaultFormat)
	}
}

func TestEngine_RegisterClient(t *testing.T) {
	e := NewEngine(nil)
	caps := ClientCaps{
		Formats:   []Format{FormatWebP, FormatJPEG},
		MaxSize:   1200,
		Hardware:  "apple-m1",
		WebPCodec: true,
	}
	client := e.RegisterClient("client-1", caps)
	if client.ID != "client-1" {
		t.Errorf("expected client-1, got %s", client.ID)
	}
	if len(client.Capabilities.Formats) != 2 {
		t.Errorf("expected 2 formats, got %d", len(client.Capabilities.Formats))
	}

	clients := e.ListClients()
	if len(clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(clients))
	}
}

func TestEngine_UnregisterClient(t *testing.T) {
	e := NewEngine(nil)
	e.RegisterClient("client-1", ClientCaps{})
	e.UnregisterClient("client-1")

	clients := e.ListClients()
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
}

func TestEngine_SubmitTask_Assigned(t *testing.T) {
	e := NewEngine(nil)
	caps := ClientCaps{
		Formats:  []Format{FormatWebP, FormatJPEG},
		MaxSize:  1200,
		Hardware: "gpu",
	}
	e.RegisterClient("client-1", caps)

	task, err := e.SubmitTask(context.Background(), "file-001", "/photos/img.jpg", FormatWebP, SizeSmall)
	if err != nil {
		t.Fatalf("SubmitTask error: %v", err)
	}
	if task.Status != TaskAssigned {
		t.Errorf("expected TaskAssigned, got %s", task.Status)
	}
	if task.ClientID != "client-1" {
		t.Errorf("expected client-1, got %s", task.ClientID)
	}
	if task.Width != 200 || task.Height != 200 {
		t.Errorf("expected 200x200, got %dx%d", task.Width, task.Height)
	}
}

func TestEngine_SubmitTask_Pending(t *testing.T) {
	e := NewEngine(nil)
	// 没有注册客户端，任务应进入待分配队列
	task, err := e.SubmitTask(context.Background(), "file-001", "/photos/img.jpg", FormatWebP, SizeMedium)
	if err != nil {
		t.Fatalf("SubmitTask error: %v", err)
	}
	if task.Status != TaskPending {
		t.Errorf("expected TaskPending, got %s", task.Status)
	}
	if task.ClientID != "" {
		t.Errorf("expected empty ClientID, got %s", task.ClientID)
	}
}

func TestEngine_SubmitTask_DefaultFormatSize(t *testing.T) {
	e := NewEngine(nil)
	task, err := e.SubmitTask(context.Background(), "file-001", "/photos/img.jpg", "", "")
	if err != nil {
		t.Fatalf("SubmitTask error: %v", err)
	}
	if task.Format != FormatWebP {
		t.Errorf("expected default WebP, got %s", task.Format)
	}
	if task.Size != SizeMedium {
		t.Errorf("expected default Medium, got %s", task.Size)
	}
}

func TestEngine_ReportResult(t *testing.T) {
	e := NewEngine(nil)
	caps := ClientCaps{Formats: []Format{FormatWebP}}
	e.RegisterClient("client-1", caps)

	task, _ := e.SubmitTask(context.Background(), "file-001", "/photos/img.jpg", FormatWebP, SizeSmall)
	now := time.Now()
	task.StartedAt = &now

	result := &TaskResult{
		ThumbnailPath: "/thumbs/file-001_small.webp",
		FileSize:      15360,
		Width:         200,
		Height:        200,
		Format:        FormatWebP,
		Checksum:      "abc123",
	}

	err := e.ReportResult(task.ID, result)
	if err != nil {
		t.Fatalf("ReportResult error: %v", err)
	}

	gotTask, ok := e.GetTask(task.ID)
	if !ok {
		t.Fatal("task not found after ReportResult")
	}
	if gotTask.Status != TaskCompleted {
		t.Errorf("expected TaskCompleted, got %s", gotTask.Status)
	}
	if gotTask.Result.FileSize != 15360 {
		t.Errorf("expected fileSize=15360, got %d", gotTask.Result.FileSize)
	}

	stats := e.GetStats()
	if stats.CompletedTasks != 1 {
		t.Errorf("expected 1 completed, got %d", stats.CompletedTasks)
	}
}

func TestEngine_ReportFailure(t *testing.T) {
	e := NewEngine(nil)
	caps := ClientCaps{Formats: []Format{FormatWebP}}
	e.RegisterClient("client-1", caps)

	task, _ := e.SubmitTask(context.Background(), "file-001", "/photos/img.jpg", FormatWebP, SizeSmall)

	err := e.ReportFailure(task.ID, "out of memory")
	if err != nil {
		t.Fatalf("ReportFailure error: %v", err)
	}

	gotTask, ok := e.GetTask(task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if gotTask.Status != TaskFailed {
		t.Errorf("expected TaskFailed, got %s", gotTask.Status)
	}
}

func TestEngine_PruneStaleClients(t *testing.T) {
	cfg := &EngineConfig{
		ClientTimeout: 5 * time.Second,
	}
	e := NewEngine(cfg)

	caps := ClientCaps{Formats: []Format{FormatWebP}}
	c := e.RegisterClient("client-1", caps)
	// 模拟过期心跳
	c.LastHeartbeat = time.Now().Add(-10 * time.Second)

	pruned := e.PruneStaleClients()
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	clients := e.ListClients()
	if len(clients) != 0 {
		t.Errorf("expected 0 clients after prune, got %d", len(clients))
	}
}

func TestSizeDimensions(t *testing.T) {
	dims, ok := SizeDimensions[SizeSmall]
	if !ok || dims[0] != 200 || dims[1] != 200 {
		t.Errorf("SizeSmall expected 200x200, got %v", dims)
	}
	dims, ok = SizeDimensions[SizeXLarge]
	if !ok || dims[0] != 1200 || dims[1] != 1200 {
		t.Errorf("SizeXLarge expected 1200x1200, got %v", dims)
	}
}

func TestEngine_MultipleClients_LoadBalancing(t *testing.T) {
	e := NewEngine(nil)

	// 注册两个客户端，client-2 支持 WebP（更匹配）
	e.RegisterClient("client-1", ClientCaps{Formats: []Format{FormatJPEG}})
	e.RegisterClient("client-2", ClientCaps{Formats: []Format{FormatWebP, FormatJPEG}, WebPCodec: true})

	task, err := e.SubmitTask(context.Background(), "file-001", "/photos/img.jpg", FormatWebP, SizeSmall)
	if err != nil {
		t.Fatalf("SubmitTask error: %v", err)
	}
	// client-2 支持 WebP，应优先分配
	if task.ClientID != "client-2" {
		t.Errorf("expected client-2 (WebP support), got %s", task.ClientID)
	}
}
