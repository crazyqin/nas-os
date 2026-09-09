// aggregation_test.go - 空间分析聚合统计测试.
package fileindex

import "testing"

func TestSizeByExtension(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()
	sizes := idx.SizeByExtension()
	if sizes[".js"] <= 0 {
		t.Error(".js 扩展名占用应大于 0")
	}
	var total int64
	for _, s := range sizes {
		total += s
	}
	if st := idx.Stats(); total != st.TotalSize {
		t.Errorf("扩展名占用合计 %d 与索引总大小 %d 不符", total, st.TotalSize)
	}
}

func TestTopDirs(t *testing.T) {
	idx := newIndexer(t)
	idx.Build()
	dirs := idx.TopDirs(10)
	// 夹具: 根下散文件 + docs/ + src/（.git 被排除）
	if len(dirs) != 3 {
		t.Fatalf("应聚合出 3 个一级目录, got %d: %+v", len(dirs), dirs)
	}
	for i := 1; i < len(dirs); i++ {
		if dirs[i-1].Size < dirs[i].Size {
			t.Error("应按占用大小降序排列")
		}
	}
	names := map[string]bool{}
	var files int
	for _, d := range dirs {
		names[d.Name] = true
		files += d.Files
	}
	if !names["src"] || !names["docs"] || !names["(根目录)"] {
		t.Errorf("目录名不符: %v", names)
	}
	if files != idx.Stats().TotalFiles {
		t.Errorf("目录文件数合计 %d 与总文件数 %d 不符", files, idx.Stats().TotalFiles)
	}
	if len(idx.TopDirs(2)) != 2 {
		t.Error("limit=2 应只返回前 2 个")
	}
}
