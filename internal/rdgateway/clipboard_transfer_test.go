// Package rdgateway 剪贴板和文件传输测试
package rdgateway

import (
	"testing"
)

func TestClipboardUpdateAndGet(t *testing.T) {
	cs := NewClipboardSync(100)

	cs.UpdateClipboard("session1", "hello world", "text", "local")

	state, err := cs.GetClipboard("session1")
	if err != nil {
		t.Fatalf("获取剪贴板失败: %v", err)
	}
	if state.Content != "hello world" {
		t.Errorf("内容不匹配: %s", state.Content)
	}
	if state.Format != "text" {
		t.Errorf("格式不匹配: %s", state.Format)
	}
	if state.Source != "local" {
		t.Errorf("来源不匹配: %s", state.Source)
	}
}

func TestClipboardNotFound(t *testing.T) {
	cs := NewClipboardSync(100)
	_, err := cs.GetClipboard("nonexistent")
	if err == nil {
		t.Error("不存在的剪贴板应返回错误")
	}
}

func TestClipboardHistory(t *testing.T) {
	cs := NewClipboardSync(100)

	cs.UpdateClipboard("s1", "content1", "text", "local")
	cs.UpdateClipboard("s1", "content2", "html", "remote")
	cs.UpdateClipboard("s2", "content3", "text", "local")

	history := cs.GetHistory("", 0)
	if len(history) != 3 {
		t.Errorf("历史应有3条: %d", len(history))
	}

	// 限制数量
	limited := cs.GetHistory("", 1)
	if len(limited) != 1 {
		t.Errorf("限制应返回1条: %d", len(limited))
	}

	// 按会话过滤
	s1History := cs.GetHistory("s1", 0)
	if len(s1History) != 2 {
		t.Errorf("s1应有2条历史: %d", len(s1History))
	}
}

func TestClipboardClear(t *testing.T) {
	cs := NewClipboardSync(100)
	cs.UpdateClipboard("s1", "content", "text", "local")

	cs.ClearClipboard("s1")

	_, err := cs.GetClipboard("s1")
	if err == nil {
		t.Error("清除后应返回错误")
	}
}

func TestClipboardDefaultMaxHistory(t *testing.T) {
	cs := NewClipboardSync(0)
	if cs.maxHistory != 100 {
		t.Errorf("默认最大历史应为100: %d", cs.maxHistory)
	}
}

func TestFileTransferStart(t *testing.T) {
	ft := NewFileTransfer()

	job, err := ft.StartTransfer("session1", "test.txt", 1024, "upload")
	if err != nil {
		t.Fatalf("开始传输失败: %v", err)
	}
	if job.Filename != "test.txt" {
		t.Errorf("文件名不匹配: %s", job.Filename)
	}
	if job.Size != 1024 {
		t.Errorf("文件大小不匹配: %d", job.Size)
	}
	if job.Direction != "upload" {
		t.Errorf("方向不匹配: %s", job.Direction)
	}
	if job.Status != TransferStatusPending {
		t.Errorf("状态应为pending: %s", job.Status)
	}
}

func TestFileTransferInvalidDirection(t *testing.T) {
	ft := NewFileTransfer()
	_, err := ft.StartTransfer("s1", "f", 100, "invalid")
	if err == nil {
		t.Error("无效方向应返回错误")
	}
}

func TestFileTransferProgress(t *testing.T) {
	ft := NewFileTransfer()
	job, _ := ft.StartTransfer("s1", "file.txt", 1000, "download")

	err := ft.UpdateProgress(job.ID, 500)
	if err != nil {
		t.Fatalf("更新进度失败: %v", err)
	}

	got, _ := ft.GetTransfer(job.ID)
	if got.Status != TransferStatusTransferring {
		t.Errorf("状态应为transferring: %s", got.Status)
	}
	if got.Transferred != 500 {
		t.Errorf("已传输不匹配: %d", got.Transferred)
	}

	// 完成
	ft.UpdateProgress(job.ID, 1000)
	got, _ = ft.GetTransfer(job.ID)
	if got.Status != TransferStatusCompleted {
		t.Errorf("状态应为completed: %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("完成时间应被设置")
	}
}

func TestFileTransferNotFound(t *testing.T) {
	ft := NewFileTransfer()
	_, err := ft.GetTransfer("nonexistent")
	if err == nil {
		t.Error("不存在的传输应返回错误")
	}

	err = ft.UpdateProgress("nonexistent", 100)
	if err == nil {
		t.Error("更新不存在的传输应返回错误")
	}
}

func TestFileTransferFail(t *testing.T) {
	ft := NewFileTransfer()
	job, _ := ft.StartTransfer("s1", "f", 100, "upload")

	ft.FailTransfer(job.ID, "network error")

	got, _ := ft.GetTransfer(job.ID)
	if got.Status != TransferStatusFailed {
		t.Errorf("状态应为failed: %s", got.Status)
	}
	if got.Error != "network error" {
		t.Errorf("错误信息不匹配: %s", got.Error)
	}
	if got.CompletedAt == nil {
		t.Error("完成时间应被设置")
	}
}

func TestFileTransferList(t *testing.T) {
	ft := NewFileTransfer()
	ft.StartTransfer("s1", "f1", 100, "upload")
	ft.StartTransfer("s1", "f2", 200, "download")
	ft.StartTransfer("s2", "f3", 300, "upload")

	all := ft.ListTransfers("")
	if len(all) != 3 {
		t.Errorf("应有3个传输: %d", len(all))
	}

	s1 := ft.ListTransfers("s1")
	if len(s1) != 2 {
		t.Errorf("s1应有2个传输: %d", len(s1))
	}
}

func TestFileTransferDelete(t *testing.T) {
	ft := NewFileTransfer()
	job, _ := ft.StartTransfer("s1", "f", 100, "upload")

	err := ft.DeleteTransfer(job.ID)
	if err != nil {
		t.Fatalf("删除传输失败: %v", err)
	}

	if ft.TransferCount() != 0 {
		t.Errorf("传输数应为0: %d", ft.TransferCount())
	}
}

func TestFileTransferDeleteNotFound(t *testing.T) {
	ft := NewFileTransfer()
	err := ft.DeleteTransfer("nonexistent")
	if err == nil {
		t.Error("删除不存在的传输应返回错误")
	}
}

func TestFileTransferCount(t *testing.T) {
	ft := NewFileTransfer()
	if ft.TransferCount() != 0 {
		t.Errorf("初始传输数应为0: %d", ft.TransferCount())
	}

	ft.StartTransfer("s1", "f1", 100, "upload")
	ft.StartTransfer("s2", "f2", 200, "download")
	if ft.TransferCount() != 2 {
		t.Errorf("传输数应为2: %d", ft.TransferCount())
	}
}
