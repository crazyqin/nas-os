package iptvservice

import (
	"testing"
)

func TestAddChannel(t *testing.T) {
	svc := NewIPTVService()

	ch := &Channel{
		Name:  "CCTV-1",
		Group: "央视",
		Type:  ChannelTypeLive,
		URL:   "http://example.com/live/cctv1.m3u8",
	}

	err := svc.AddChannel(ch)
	if err != nil {
		t.Fatalf("添加频道失败: %v", err)
	}

	if ch.ID == "" {
		t.Error("频道 ID 不应为空")
	}
}

func TestSearchChannels(t *testing.T) {
	svc := NewIPTVService()

	svc.AddChannel(&Channel{Name: "CCTV-1", Group: "央视", URL: "http://1"})
	svc.AddChannel(&Channel{Name: "CCTV-2", Group: "央视", URL: "http://2"})
	svc.AddChannel(&Channel{Name: "湖南卫视", Group: "卫视", URL: "http://3"})

	results := svc.SearchChannels("CCTV")
	if len(results) != 2 {
		t.Errorf("应找到 2 个频道, got %d", len(results))
	}
}

func TestGetStats(t *testing.T) {
	svc := NewIPTVService()

	svc.AddChannel(&Channel{Name: "CH1", URL: "http://1"})
	svc.AddChannel(&Channel{Name: "CH2", URL: "http://2"})

	stats := svc.GetStats()
	total := stats["totalChannels"].(int)
	if total != 2 {
		t.Errorf("总频道数应为 2, got %d", total)
	}
}

func TestImportM3U(t *testing.T) {
	svc := NewIPTVService()

	playlist, err := svc.ImportM3U("test", "http://example.com/playlist.m3u")
	if err != nil {
		t.Fatalf("导入 M3U 失败: %v", err)
	}

	if playlist.Name != "test" {
		t.Errorf("播放列表名称应为 test, got %s", playlist.Name)
	}
}
