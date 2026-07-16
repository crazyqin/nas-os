// Package knowledgebase 测试
package knowledgebase

import (
	"encoding/json"
	"testing"
)

func TestCreateDoc(t *testing.T) {
	e := NewEngine()
	doc, err := e.CreateDoc(CreateDocRequest{
		Title:   "测试文档",
		Content: "# Hello\n这是测试内容",
		Author:  "admin",
		Tags:    []string{"test", "demo"},
	})
	if err != nil {
		t.Fatalf("创建文档失败: %v", err)
	}
	if doc == nil {
		t.Fatal("文档不应为nil")
	}
	if doc.Title != "测试文档" {
		t.Errorf("标题不匹配: %s", doc.Title)
	}
	if doc.Author != "admin" {
		t.Errorf("作者不匹配: %s", doc.Author)
	}
	if len(doc.Tags) != 2 {
		t.Errorf("标签数量不匹配: %d", len(doc.Tags))
	}
}

func TestGetDoc(t *testing.T) {
	e := NewEngine()
	doc, _ := e.CreateDoc(CreateDocRequest{Title: "test", Content: "content", Author: "admin"})

	got, err := e.GetDoc(doc.ID)
	if err != nil {
		t.Fatalf("获取文档失败: %v", err)
	}
	if got.Title != "test" {
		t.Errorf("标题不匹配")
	}
}

func TestGetDocNotFound(t *testing.T) {
	e := NewEngine()
	_, err := e.GetDoc("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestUpdateDoc(t *testing.T) {
	e := NewEngine()
	doc, _ := e.CreateDoc(CreateDocRequest{Title: "old", Content: "old content", Author: "admin"})

	newTitle := "new"
	updated, err := e.UpdateDoc(doc.ID, UpdateDocRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Title != "new" {
		t.Errorf("标题未更新: %s", updated.Title)
	}
}

func TestDeleteDoc(t *testing.T) {
	e := NewEngine()
	doc, _ := e.CreateDoc(CreateDocRequest{Title: "to delete", Author: "admin"})

	err := e.DeleteDoc(doc.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = e.GetDoc(doc.ID)
	if err == nil {
		t.Error("已删除文档不应存在")
	}
}

func TestListDocs(t *testing.T) {
	e := NewEngine()
	e.CreateDoc(CreateDocRequest{Title: "doc1", Author: "admin", WorkspaceID: "ws1"})
	e.CreateDoc(CreateDocRequest{Title: "doc2", Author: "admin", WorkspaceID: "ws1"})
	e.CreateDoc(CreateDocRequest{Title: "doc3", Author: "admin", WorkspaceID: "ws2"})

	all := e.ListDocs("ws1")
	if len(all) != 2 {
		t.Errorf("期望2个文档，实际 %d", len(all))
	}

	allDocs := e.ListDocs("")
	if len(allDocs) != 3 {
		t.Errorf("期望3个文档，实际 %d", len(allDocs))
	}
}

func TestSearchDocs(t *testing.T) {
	e := NewEngine()
	e.CreateDoc(CreateDocRequest{Title: "Go语言入门", Content: "学习Go编程", Author: "admin"})
	e.CreateDoc(CreateDocRequest{Title: "Python教程", Content: "学习Python", Author: "admin"})

	results := e.SearchDocs(SearchQuery{Query: "Go"})
	if len(results) == 0 {
		t.Error("搜索Go应有结果")
	}

	results = e.SearchDocs(SearchQuery{Query: "Python"})
	if len(results) == 0 {
		t.Error("搜索Python应有结果")
	}
}

func TestSearchDocsWithTags(t *testing.T) {
	e := NewEngine()
	e.CreateDoc(CreateDocRequest{Title: "Go语言入门", Content: "学习Go编程", Author: "admin", Tags: []string{"go", "backend"}})
	e.CreateDoc(CreateDocRequest{Title: "Python教程", Content: "学习Python", Author: "admin", Tags: []string{"python", "ml"}})

	results := e.SearchDocs(SearchQuery{Query: "Go", Tags: []string{"go"}})
	if len(results) != 1 {
		t.Errorf("期望1个结果，实际 %d", len(results))
	}
}

func TestCreateWorkspace(t *testing.T) {
	e := NewEngine()
	ws, err := e.CreateWorkspace(CreateWorkspaceRequest{
		Name:        "工作空间",
		Description: "测试工作空间",
		Owner:       "admin",
	})
	if err != nil {
		t.Fatalf("创建工作空间失败: %v", err)
	}
	if ws.Name != "工作空间" {
		t.Errorf("名称不匹配: %s", ws.Name)
	}
}

func TestGetWorkspace(t *testing.T) {
	e := NewEngine()
	ws, _ := e.CreateWorkspace(CreateWorkspaceRequest{Name: "test", Owner: "admin"})

	got, err := e.GetWorkspace(ws.ID)
	if err != nil {
		t.Fatalf("获取工作空间失败: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("名称不匹配")
	}
}

func TestUpdateWorkspace(t *testing.T) {
	e := NewEngine()
	ws, _ := e.CreateWorkspace(CreateWorkspaceRequest{Name: "old", Owner: "admin"})

	newName := "new"
	updated, err := e.UpdateWorkspace(ws.ID, UpdateWorkspaceRequest{Name: &newName})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("名称未更新: %s", updated.Name)
	}
}

func TestDeleteWorkspace(t *testing.T) {
	e := NewEngine()
	ws, _ := e.CreateWorkspace(CreateWorkspaceRequest{Name: "temp", Owner: "admin"})

	err := e.DeleteWorkspace(ws.ID)
	if err != nil {
		t.Fatalf("删除工作空间失败: %v", err)
	}
}

func TestListWorkspaces(t *testing.T) {
	e := NewEngine()
	e.CreateWorkspace(CreateWorkspaceRequest{Name: "ws1", Owner: "admin"})
	e.CreateWorkspace(CreateWorkspaceRequest{Name: "ws2", Owner: "admin"})
	e.CreateWorkspace(CreateWorkspaceRequest{Name: "ws3", Owner: "other"})

	adminWS := e.ListWorkspaces("admin")
	if len(adminWS) != 2 {
		t.Errorf("期望2个工作空间，实际 %d", len(adminWS))
	}

	allWS := e.ListWorkspaces("")
	if len(allWS) != 3 {
		t.Errorf("期望3个工作空间，实际 %d", len(allWS))
	}
}

func TestCreateTemplate(t *testing.T) {
	e := NewEngine()
	tpl, err := e.CreateTemplate("会议记录", "会议记录模板", "# 会议记录\n\n## 参与者\n\n## 议题\n", "工作")
	if err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}
	if tpl.Name != "会议记录" {
		t.Errorf("名称不匹配: %s", tpl.Name)
	}
}

func TestGetTemplate(t *testing.T) {
	e := NewEngine()
	tpl, _ := e.CreateTemplate("test", "desc", "content", "cat")

	got, err := e.GetTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("获取模板失败: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("名称不匹配")
	}
}

func TestListTemplates(t *testing.T) {
	e := NewEngine()
	e.CreateTemplate("tpl1", "desc", "content", "工作")
	e.CreateTemplate("tpl2", "desc", "content", "学习")
	e.CreateTemplate("tpl3", "desc", "content", "工作")

	workTpls := e.ListTemplates("工作")
	if len(workTpls) != 2 {
		t.Errorf("期望2个模板，实际 %d", len(workTpls))
	}

	allTpls := e.ListTemplates("")
	if len(allTpls) != 3 {
		t.Errorf("期望3个模板，实际 %d", len(allTpls))
	}
}

func TestDeleteTemplate(t *testing.T) {
	e := NewEngine()
	tpl, _ := e.CreateTemplate("temp", "desc", "content", "cat")

	err := e.DeleteTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("删除模板失败: %v", err)
	}
}

func TestCreateDocFromTemplate(t *testing.T) {
	e := NewEngine()
	tpl, _ := e.CreateTemplate("meeting", "会议模板", "# 会议记录\n\n## 议题\n", "工作")

	doc, err := e.CreateDocFromTemplate(tpl.ID, CreateDocRequest{
		Title:  "周会",
		Author: "admin",
	})
	if err != nil {
		t.Fatalf("从模板创建文档失败: %v", err)
	}
	if doc.Content != "# 会议记录\n\n## 议题\n" {
		t.Error("内容应使用模板内容")
	}
}

func TestAddNote(t *testing.T) {
	e := NewEngine()
	doc, _ := e.CreateDoc(CreateDocRequest{Title: "test", Author: "admin"})

	note, err := e.AddNote(doc.ID, "这是笔记", "admin")
	if err != nil {
		t.Fatalf("添加笔记失败: %v", err)
	}
	if note.Content != "这是笔记" {
		t.Errorf("内容不匹配: %s", note.Content)
	}
}

func TestGetNotes(t *testing.T) {
	e := NewEngine()
	doc, _ := e.CreateDoc(CreateDocRequest{Title: "test", Author: "admin"})

	e.AddNote(doc.ID, "note1", "admin")
	e.AddNote(doc.ID, "note2", "admin")
	e.AddNote(doc.ID, "note3", "other")

	notes := e.GetNotes(doc.ID)
	if len(notes) != 3 {
		t.Errorf("期望3个笔记，实际 %d", len(notes))
	}
}

func TestGetTagStats(t *testing.T) {
	e := NewEngine()
	e.CreateDoc(CreateDocRequest{Title: "d1", Tags: []string{"go", "backend"}, Author: "a"})
	e.CreateDoc(CreateDocRequest{Title: "d2", Tags: []string{"go", "frontend"}, Author: "a"})
	e.CreateDoc(CreateDocRequest{Title: "d3", Tags: []string{"python"}, Author: "a"})

	stats := e.GetTagStats()
	goCount := 0
	for _, s := range stats {
		if s.Tag == "go" {
			goCount = s.Count
		}
	}
	if goCount != 2 {
		t.Errorf("go标签应出现2次，实际 %d", goCount)
	}
}

func TestGetFavorites(t *testing.T) {
	e := NewEngine()
	doc1, _ := e.CreateDoc(CreateDocRequest{Title: "fav1", Author: "admin"})
	e.CreateDoc(CreateDocRequest{Title: "fav2", Author: "admin"})

	isFav := true
	e.UpdateDoc(doc1.ID, UpdateDocRequest{IsFavorite: &isFav})

	favs := e.GetFavorites()
	if len(favs) != 1 {
		t.Errorf("期望1个收藏，实际 %d", len(favs))
	}
}

func TestGraphAddNode(t *testing.T) {
	g := NewGraph()
	err := g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	if err != nil {
		t.Fatalf("添加节点失败: %v", err)
	}

	node, err := g.GetNode("1")
	if err != nil {
		t.Fatalf("获取节点失败: %v", err)
	}
	if node.Title != "Node 1" {
		t.Errorf("标题不匹配: %s", node.Title)
	}
}

func TestGraphAddEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})

	err := g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	if err != nil {
		t.Fatalf("添加边失败: %v", err)
	}

	edges := g.GetEdges("1")
	if len(edges) != 1 {
		t.Errorf("期望1条边，实际 %d", len(edges))
	}
}

func TestGraphRemoveNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})

	err := g.RemoveNode("1")
	if err != nil {
		t.Fatalf("删除节点失败: %v", err)
	}

	_, err = g.GetNode("1")
	if err == nil {
		t.Error("已删除节点不应存在")
	}

	edges := g.GetEdges("2")
	if len(edges) != 0 {
		t.Error("相关边应被删除")
	}
}

func TestGraphRemoveEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})

	err := g.RemoveEdge("1", "2")
	if err != nil {
		t.Fatalf("删除边失败: %v", err)
	}

	edges := g.GetEdges("1")
	if len(edges) != 0 {
		t.Error("边应被删除")
	}
}

func TestGraphGetNeighbors(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	g.AddEdge(GraphEdge{Source: "1", Target: "3", Type: "link"})

	neighbors := g.GetNeighbors("1")
	if len(neighbors) != 2 {
		t.Errorf("期望2个邻居，实际 %d", len(neighbors))
	}
}

func TestGraphFindPath(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddNode(GraphNode{ID: "4", Title: "Node 4"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	g.AddEdge(GraphEdge{Source: "2", Target: "3", Type: "link"})
	g.AddEdge(GraphEdge{Source: "3", Target: "4", Type: "link"})

	path, err := g.FindPath("1", "4")
	if err != nil {
		t.Fatalf("查找路径失败: %v", err)
	}
	if len(path) != 4 {
		t.Errorf("路径长度应为4，实际 %d", len(path))
	}
}

func TestGraphGetBacklinks(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddEdge(GraphEdge{Source: "1", Target: "3", Type: "link"})
	g.AddEdge(GraphEdge{Source: "2", Target: "3", Type: "link"})

	backlinks := g.GetBacklinks("3")
	if len(backlinks) != 2 {
		t.Errorf("期望2个反向链接，实际 %d", len(backlinks))
	}
}

func TestGraphGetForwardLinks(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	g.AddEdge(GraphEdge{Source: "1", Target: "3", Type: "link"})

	forwardLinks := g.GetForwardLinks("1")
	if len(forwardLinks) != 2 {
		t.Errorf("期望2个正向链接，实际 %d", len(forwardLinks))
	}
}

func TestGraphGetGraphData(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})

	data := g.GetGraphData()
	if len(data.Nodes) != 2 {
		t.Errorf("期望2个节点，实际 %d", len(data.Nodes))
	}
	if len(data.Edges) != 1 {
		t.Errorf("期望1条边，实际 %d", len(data.Edges))
	}
}

func TestGraphGetConnectedComponents(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddNode(GraphNode{ID: "4", Title: "Node 4"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	g.AddEdge(GraphEdge{Source: "3", Target: "4", Type: "link"})

	components := g.GetConnectedComponents()
	if len(components) != 2 {
		t.Errorf("期望2个连通分量，实际 %d", len(components))
	}
}

func TestGraphGetNodeDegree(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	g.AddEdge(GraphEdge{Source: "1", Target: "3", Type: "link"})

	in, out := g.GetNodeDegree("1")
	if in != 0 {
		t.Errorf("入度应为0，实际 %d", in)
	}
	if out != 2 {
		t.Errorf("出度应为2，实际 %d", out)
	}
}

func TestGraphGetMostConnectedNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode(GraphNode{ID: "1", Title: "Node 1"})
	g.AddNode(GraphNode{ID: "2", Title: "Node 2"})
	g.AddNode(GraphNode{ID: "3", Title: "Node 3"})
	g.AddEdge(GraphEdge{Source: "1", Target: "2", Type: "link"})
	g.AddEdge(GraphEdge{Source: "1", Target: "3", Type: "link"})

	nodes := g.GetMostConnectedNodes(1)
	if len(nodes) != 1 {
		t.Errorf("期望1个节点，实际 %d", len(nodes))
	}
	if nodes[0].ID != "1" {
		t.Errorf("应返回节点1，实际 %s", nodes[0].ID)
	}
}

func TestBuildGraphFromDocs(t *testing.T) {
	g := NewGraph()
	docs := []*Document{
		{ID: "1", Title: "Doc 1", Links: []Link{{SourceID: "1", TargetID: "2"}}},
		{ID: "2", Title: "Doc 2"},
	}
	links := []Link{
		{SourceID: "1", TargetID: "2", Type: "reference"},
	}

	g.BuildGraphFromDocs(docs, links)

	data := g.GetGraphData()
	if len(data.Nodes) != 2 {
		t.Errorf("期望2个节点，实际 %d", len(data.Nodes))
	}
	if len(data.Edges) != 1 {
		t.Errorf("期望1条边，实际 %d", len(data.Edges))
	}
}

func TestEditorRenderMarkdown(t *testing.T) {
	e := NewEditor()
	result := e.RenderMarkdown("# Hello\n这是**粗体**和*斜体*")

	if result.HTML == "" {
		t.Error("HTML不应为空")
	}
	if result.Plain == "" {
		t.Error("Plain不应为空")
	}
}

func TestEditorHighlightCode(t *testing.T) {
	e := NewEditor()
	result := e.HighlightCode("func main() {}", "go")

	if result == "" {
		t.Error("结果不应为空")
	}
}

func TestEditorRenderMath(t *testing.T) {
	e := NewEditor()
	result := e.RenderMath("行内公式 $E=mc^2$ 和块级公式 $$\\int_0^1 x dx$$")

	if result == "" {
		t.Error("结果不应为空")
	}
}

func TestEditorExtractLinks(t *testing.T) {
	e := NewEditor()
	links := e.ExtractLinks("[link1](http://1.com) and [link2](http://2.com)")

	if len(links) != 2 {
		t.Errorf("期望2个链接，实际 %d", len(links))
	}
}

func TestEditorExtractWikiLinks(t *testing.T) {
	e := NewEditor()
	links := e.ExtractWikiLinks("参见 [[文档1]] 和 [[文档2]]")

	if len(links) != 2 {
		t.Errorf("期望2个链接，实际 %d", len(links))
	}
}

func TestEditorExtractTags(t *testing.T) {
	e := NewEditor()
	tags := e.ExtractTags("这是 #go 和 #python 标签")

	if len(tags) != 2 {
		t.Errorf("期望2个标签，实际 %d", len(tags))
	}
}

func TestEditorExtractMentions(t *testing.T) {
	e := NewEditor()
	mentions := e.ExtractMentions("@alice 和 @bob")

	if len(mentions) != 2 {
		t.Errorf("期望2个提及，实际 %d", len(mentions))
	}
}

func TestEditorGenerateTOC(t *testing.T) {
	e := NewEditor()
	toc := e.GenerateTOC("# 标题1\n## 标题2\n### 标题3")

	if len(toc) != 3 {
		t.Errorf("期望3个目录项，实际 %d", len(toc))
	}
	if toc[0].Level != 1 {
		t.Errorf("第一项级别应为1，实际 %d", toc[0].Level)
	}
}

func TestEditorWordCount(t *testing.T) {
	e := NewEditor()
	count := e.WordCount("Hello World Foo Bar")

	if count != 4 {
		t.Errorf("期望4个单词，实际 %d", count)
	}
}

func TestEditorCharCount(t *testing.T) {
	e := NewEditor()
	count := e.CharCount("Hello")

	if count != 5 {
		t.Errorf("期望5个字符，实际 %d", count)
	}
}

func TestEditorReadingTime(t *testing.T) {
	e := NewEditor()
	words := make([]string, 400)
	for i := range words {
		words[i] = "word"
	}
	content := ""
	for _, w := range words {
		content += w + " "
	}

	time := e.ReadingTime(content)
	if time != 2 {
		t.Errorf("期望2分钟，实际 %d", time)
	}
}

func TestEditorReplaceVariables(t *testing.T) {
	e := NewEditor()
	result := e.ReplaceVariables("Hello {{name}}, welcome to {{place}}!", map[string]string{
		"name":  "Alice",
		"place": "Wonderland",
	})

	if result != "Hello Alice, welcome to Wonderland!" {
		t.Errorf("变量替换失败: %s", result)
	}
}

func TestEditorSanitizeHTML(t *testing.T) {
	e := NewEditor()
	result := e.SanitizeHTML(`<p>Hello</p><script>alert('xss')</script>`)

	if result == `<p>Hello</p><script>alert('xss')</script>` {
		t.Error("脚本标签应被移除")
	}
}

func TestImporterFromNotion(t *testing.T) {
	imp := NewImporter()
	data := []byte(`{
		"pages": [
			{
				"id": "page1",
				"title": "Notion页面",
				"content": "内容",
				"tags": ["test"]
			}
		]
	}`)

	result, err := imp.ImportFromNotion(data, "ws1", "admin")
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if len(result.Docs) != 1 {
		t.Errorf("期望1个文档，实际 %d", len(result.Docs))
	}
}

func TestImporterFromObsidian(t *testing.T) {
	imp := NewImporter()
	data := []byte(`{
		"files": [
			{
				"path": "notes/test.md",
				"content": "# Test\n内容 #tag1 #tag2",
				"front_matter": {"tags": ["meta"]}
			}
		]
	}`)

	result, err := imp.ImportFromObsidian(data, "ws1", "admin")
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if len(result.Docs) != 1 {
		t.Errorf("期望1个文档，实际 %d", len(result.Docs))
	}
}

func TestImporterFromConfluence(t *testing.T) {
	imp := NewImporter()
	data := []byte(`{
		"pages": [
			{
				"id": "page1",
				"title": "Confluence页面",
				"content": "<h1>标题</h1><p>内容</p>",
				"space": "SPACE"
			}
		]
	}`)

	result, err := imp.ImportFromConfluence(data, "ws1", "admin")
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if len(result.Docs) != 1 {
		t.Errorf("期望1个文档，实际 %d", len(result.Docs))
	}
}

func TestImporterFromEvernote(t *testing.T) {
	imp := NewImporter()
	data := []byte(`{
		"notes": [
			{
				"guid": "note1",
				"title": "Evernote笔记",
				"content": "<en-note>内容</en-note>",
				"tags": ["test"],
				"notebook": "default",
				"created": "20240101T000000Z",
				"updated": "20240102T000000Z"
			}
		]
	}`)

	result, err := imp.ImportFromEvernote(data, "ws1", "admin")
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if len(result.Docs) != 1 {
		t.Errorf("期望1个文档，实际 %d", len(result.Docs))
	}
}

func TestImporterFromMarkdown(t *testing.T) {
	imp := NewImporter()
	doc, err := imp.ImportFromMarkdown("# 测试\n内容 #tag", "", "ws1", "admin")
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if doc.Title != "测试" {
		t.Errorf("标题不匹配: %s", doc.Title)
	}
}

func TestExporterToMarkdown(t *testing.T) {
	imp := NewImporter()
	doc := &Document{
		Title:   "测试",
		Content: "内容",
		Author:  "admin",
		Tags:    []string{"test"},
	}

	md := imp.ExportToMarkdown(doc)
	if md == "" {
		t.Error("导出结果不应为空")
	}
}

func TestExporterToJSON(t *testing.T) {
	imp := NewImporter()
	doc := &Document{
		ID:      "1",
		Title:   "测试",
		Content: "内容",
		Author:  "admin",
	}

	data, err := imp.ExportToJSON(doc)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("JSON解析失败: %v", err)
	}
	if result["title"] != "测试" {
		t.Errorf("标题不匹配: %v", result["title"])
	}
}
