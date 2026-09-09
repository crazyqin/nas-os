// checker_test.go - 更新检查纯逻辑测试（不发网络请求）.
package sysupdate

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		cur, tag string
		want     int
	}{
		{"3.24.6", "v3.24.6", 0},
		{"3.24.6", "v3.24.7", -1},
		{"3.25.0", "v3.24.9", 1},
		{"3.24.6", "v3.25.0-rc1", -1},
		{"3.24.6", "nightly-build", -1}, // 非 semver 标签且不同，视为有更新
		{"weird", "weird", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.cur, c.tag); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.cur, c.tag, got, c.want)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Errorf("短文本不应截断: %q", got)
	}
	if got := truncateRunes("hello world", 5); got != "hello\n…" {
		t.Errorf("长文本截断结果: %q", got)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	store := NewSettingsStore(t.TempDir())
	if store.Load().AutoCheck {
		t.Fatal("默认 AutoCheck 应为 false")
	}
	if err := store.Save(Settings{AutoCheck: true}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if !store.Load().AutoCheck {
		t.Fatal("保存后应能读回 true")
	}
}
