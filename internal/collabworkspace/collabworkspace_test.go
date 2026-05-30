package collabworkspace

import (
	"testing"
	"time"
)

// ==================== 工作空间管理测试 ====================

func TestCreateWorkspace(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, err := cw.CreateWorkspace("测试工作空间", "这是一个测试", "user1")
	if err != nil {
		t.Fatalf("创建工作空间失败: %v", err)
	}

	if ws.Name != "测试工作空间" {
		t.Errorf("期望工作空间名称为 '测试工作空间'，实际为 '%s'", ws.Name)
	}

	if ws.OwnerID != "user1" {
		t.Errorf("期望所有者为 'user1'，实际为 '%s'", ws.OwnerID)
	}

	if len(ws.Members) != 1 {
		t.Errorf("期望成员数为 1，实际为 %d", len(ws.Members))
	}

	if ws.Members[0].Role != "owner" {
		t.Errorf("期望创建者角色为 'owner'，实际为 '%s'", ws.Members[0].Role)
	}
}

func TestCreateWorkspaceValidation(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	_, err := cw.CreateWorkspace("", "desc", "user1")
	if err == nil {
		t.Error("期望空名称返回错误")
	}

	_, err = cw.CreateWorkspace("name", "desc", "")
	if err == nil {
		t.Error("期望空所有者返回错误")
	}
}

func TestDeleteWorkspace(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("待删除", "desc", "user1")

	// 创建文档和任务
	cw.CreateDocument(ws.ID, "doc1", "content", "user1")
	cw.CreateTask(ws.ID, "task1", "desc", "user1", 3, nil)

	err := cw.DeleteWorkspace(ws.ID, "user1")
	if err != nil {
		t.Fatalf("删除工作空间失败: %v", err)
	}

	_, err = cw.GetWorkspace(ws.ID)
	if err == nil {
		t.Error("期望工作空间已被删除")
	}

	// 验证文档也被删除
	docs, _ := cw.ListDocuments(ws.ID, "user1")
	if len(docs) != 0 {
		t.Errorf("期望文档数为 0，实际为 %d", len(docs))
	}
}

func TestDeleteWorkspacePermission(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	err := cw.DeleteWorkspace(ws.ID, "user2")
	if err == nil {
		t.Error("期望非管理员无法删除工作空间")
	}
}

func TestListWorkspaces(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	cw.CreateWorkspace("ws1", "desc", "user1")
	cw.CreateWorkspace("ws2", "desc", "user1")
	cw.CreateWorkspace("ws3", "desc", "user2")

	wsList := cw.ListWorkspaces("user1")
	if len(wsList) != 2 {
		t.Errorf("期望 user1 看到 2 个工作空间，实际为 %d", len(wsList))
	}

	wsList = cw.ListWorkspaces("user2")
	if len(wsList) != 1 {
		t.Errorf("期望 user2 看到 1 个工作空间，实际为 %d", len(wsList))
	}
}

func TestListWorkspacesExcludesArchived(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("archived", "desc", "user1")
	cw.ArchiveWorkspace(ws.ID, "user1")

	wsList := cw.ListWorkspaces("user1")
	if len(wsList) != 0 {
		t.Errorf("期望归档工作空间不出现在列表中，实际为 %d", len(wsList))
	}
}

func TestArchiveWorkspace(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	err := cw.ArchiveWorkspace(ws.ID, "user1")
	if err != nil {
		t.Fatalf("归档工作空间失败: %v", err)
	}

	got, _ := cw.GetWorkspace(ws.ID)
	if !got.IsArchived {
		t.Error("期望工作空间已归档")
	}
}

// ==================== 成员管理测试 ====================

func TestAddMember(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	err := cw.AddMember(ws.ID, "user2", "张三", "member", "user1")
	if err != nil {
		t.Fatalf("添加成员失败: %v", err)
	}

	got, _ := cw.GetWorkspace(ws.ID)
	if len(got.Members) != 2 {
		t.Errorf("期望成员数为 2，实际为 %d", len(got.Members))
	}
}

func TestAddMemberDuplicate(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "张三", "member", "user1")

	err := cw.AddMember(ws.ID, "user2", "张三", "member", "user1")
	if err == nil {
		t.Error("期望重复添加成员返回错误")
	}
}

func TestRemoveMember(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "张三", "member", "user1")

	err := cw.RemoveMember(ws.ID, "user2", "user1")
	if err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}

	got, _ := cw.GetWorkspace(ws.ID)
	if len(got.Members) != 1 {
		t.Errorf("期望成员数为 1，实际为 %d", len(got.Members))
	}
}

func TestRemoveOwner(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	err := cw.RemoveMember(ws.ID, "user1", "user1")
	if err == nil {
		t.Error("期望无法移除所有者")
	}
}

// ==================== 权限控制测试 ====================

func TestPermission(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	// 所有者应该有管理权限
	if !cw.CheckPermission(ws.ID, "user1", PermissionManage) {
		t.Error("期望所有者有管理权限")
	}

	// 新成员应该有写权限
	if !cw.CheckPermission(ws.ID, "user2", PermissionWrite) {
		t.Error("期望新成员有写权限")
	}

	// 设置只读权限
	err := cw.SetPermission(ws.ID, "user2", PermissionRead, "user1")
	if err != nil {
		t.Fatalf("设置权限失败: %v", err)
	}

	if cw.CheckPermission(ws.ID, "user2", PermissionWrite) {
		t.Error("期望 user2 没有写权限")
	}

	if !cw.CheckPermission(ws.ID, "user2", PermissionRead) {
		t.Error("期望 user2 有读权限")
	}
}

func TestGetPermission(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	level := cw.GetPermission(ws.ID, "user1")
	if level != PermissionManage {
		t.Errorf("期望权限级别为 %d，实际为 %d", PermissionManage, level)
	}

	level = cw.GetPermission(ws.ID, "unknown")
	if level != PermissionNone {
		t.Errorf("期望权限级别为 %d，实际为 %d", PermissionNone, level)
	}
}

// ==================== 文档协作测试 ====================

func TestCreateDocument(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	doc, err := cw.CreateDocument(ws.ID, "测试文档", "初始内容", "user1")
	if err != nil {
		t.Fatalf("创建文档失败: %v", err)
	}

	if doc.Title != "测试文档" {
		t.Errorf("期望文档标题为 '测试文档'，实际为 '%s'", doc.Title)
	}

	if doc.Version != 1 {
		t.Errorf("期望版本为 1，实际为 %d", doc.Version)
	}

	if len(doc.Versions) != 1 {
		t.Errorf("期望版本历史长度为 1，实际为 %d", len(doc.Versions))
	}
}

func TestEditDocument(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "v1", "user1")

	err := cw.EditDocument(doc.ID, "v2", "user1", "第二次编辑")
	if err != nil {
		t.Fatalf("编辑文档失败: %v", err)
	}

	got, _ := cw.GetDocument(doc.ID, "user1")
	if got.Content != "v2" {
		t.Errorf("期望内容为 'v2'，实际为 '%s'", got.Content)
	}

	if got.Version != 2 {
		t.Errorf("期望版本为 2，实际为 %d", got.Version)
	}

	if len(got.Versions) != 2 {
		t.Errorf("期望版本历史长度为 2，实际为 %d", len(got.Versions))
	}
}

func TestDocumentPermission(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	// 设置 user2 为只读
	cw.SetPermission(ws.ID, "user2", PermissionRead, "user1")

	_, err := cw.GetDocument(doc.ID, "user2")
	if err != nil {
		t.Error("期望只读用户能获取文档")
	}

	err = cw.EditDocument(doc.ID, "new", "user2", "edit")
	if err == nil {
		t.Error("期望只读用户无法编辑文档")
	}
}

func TestDocumentLock(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	err := cw.LockDocument(doc.ID, "user1")
	if err != nil {
		t.Fatalf("锁定文档失败: %v", err)
	}

	// user2 无法编辑被锁定的文档
	err = cw.EditDocument(doc.ID, "new", "user2", "edit")
	if err == nil {
		t.Error("期望被锁定的文档无法被其他人编辑")
	}

	// 解锁
	err = cw.UnlockDocument(doc.ID, "user1")
	if err != nil {
		t.Fatalf("解锁文档失败: %v", err)
	}

	// 现在可以编辑
	err = cw.EditDocument(doc.ID, "new", "user2", "edit")
	if err != nil {
		t.Errorf("解锁后期望可以编辑: %v", err)
	}
}

func TestDocumentEditing(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	err := cw.JoinEditing(doc.ID, "user1")
	if err != nil {
		t.Fatalf("加入编辑失败: %v", err)
	}

	err = cw.JoinEditing(doc.ID, "user2")
	if err != nil {
		t.Fatalf("加入编辑失败: %v", err)
	}

	got, _ := cw.GetDocument(doc.ID, "user1")
	if len(got.Editors) != 2 {
		t.Errorf("期望编辑者数为 2，实际为 %d", len(got.Editors))
	}

	cw.LeaveEditing(doc.ID, "user1")
	got, _ = cw.GetDocument(doc.ID, "user1")
	if len(got.Editors) != 1 {
		t.Errorf("期望编辑者数为 1，实际为 %d", len(got.Editors))
	}
}

func TestDocumentVersionRestore(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "version1", "user1")
	cw.EditDocument(doc.ID, "version2", "user1", "edit2")
	cw.EditDocument(doc.ID, "version3", "user1", "edit3")

	err := cw.RestoreDocumentVersion(doc.ID, 1, "user1")
	if err != nil {
		t.Fatalf("恢复版本失败: %v", err)
	}

	got, _ := cw.GetDocument(doc.ID, "user1")
	if got.Content != "version1" {
		t.Errorf("期望内容为 'version1'，实际为 '%s'", got.Content)
	}

	// 版本号应该增加
	if got.Version != 4 {
		t.Errorf("期望版本为 4，实际为 %d", got.Version)
	}
}

func TestDeleteDocument(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	err := cw.DeleteDocument(doc.ID, "user1")
	if err != nil {
		t.Fatalf("删除文档失败: %v", err)
	}

	_, err = cw.GetDocument(doc.ID, "user1")
	if err == nil {
		t.Error("期望文档已被删除")
	}
}

func TestListDocuments(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.CreateDocument(ws.ID, "doc1", "c1", "user1")
	cw.CreateDocument(ws.ID, "doc2", "c2", "user1")
	cw.CreateDocument(ws.ID, "doc3", "c3", "user1")

	docs, err := cw.ListDocuments(ws.ID, "user1")
	if err != nil {
		t.Fatalf("列出文档失败: %v", err)
	}

	if len(docs) != 3 {
		t.Errorf("期望文档数为 3，实际为 %d", len(docs))
	}
}

// ==================== 评论系统测试 ====================

func TestAddComment(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	comment, err := cw.AddComment(doc.ID, "user1", "张三", "这是一条评论", "")
	if err != nil {
		t.Fatalf("添加评论失败: %v", err)
	}

	if comment.Content != "这是一条评论" {
		t.Errorf("期望评论内容为 '这是一条评论'，实际为 '%s'", comment.Content)
	}
}

func TestReplyComment(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	parent, _ := cw.AddComment(doc.ID, "user1", "张三", "原始评论", "")
	reply, err := cw.AddComment(doc.ID, "user2", "李四", "回复评论", parent.ID)
	if err != nil {
		t.Fatalf("回复评论失败: %v", err)
	}

	if reply.ParentID != parent.ID {
		t.Errorf("期望父评论ID为 '%s'，实际为 '%s'", parent.ID, reply.ParentID)
	}
}

func TestEditComment(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	comment, _ := cw.AddComment(doc.ID, "user1", "张三", "原始内容", "")

	err := cw.EditComment(comment.ID, "user1", "修改后的内容")
	if err != nil {
		t.Fatalf("编辑评论失败: %v", err)
	}

	comments, _ := cw.GetComments(doc.ID, "user1")
	if comments[0].Content != "修改后的内容" {
		t.Errorf("期望评论内容为 '修改后的内容'，实际为 '%s'", comments[0].Content)
	}

	if !comments[0].IsEdited {
		t.Error("期望 IsEdited 为 true")
	}
}

func TestEditCommentPermission(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	comment, _ := cw.AddComment(doc.ID, "user1", "张三", "内容", "")

	err := cw.EditComment(comment.ID, "user2", "恶意修改")
	if err == nil {
		t.Error("期望非作者无法编辑评论")
	}
}

func TestDeleteComment(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	comment, _ := cw.AddComment(doc.ID, "user1", "张三", "待删除", "")

	err := cw.DeleteComment(comment.ID, "user1")
	if err != nil {
		t.Fatalf("删除评论失败: %v", err)
	}

	comments, _ := cw.GetComments(doc.ID, "user1")
	if len(comments) != 0 {
		t.Errorf("期望评论数为 0，实际为 %d", len(comments))
	}
}

func TestGetComments(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	doc, _ := cw.CreateDocument(ws.ID, "doc", "content", "user1")

	cw.AddComment(doc.ID, "user1", "user1", "评论1", "")
	cw.AddComment(doc.ID, "user1", "user1", "评论2", "")

	comments, err := cw.GetComments(doc.ID, "user1")
	if err != nil {
		t.Fatalf("获取评论失败: %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("期望评论数为 2，实际为 %d", len(comments))
	}
}

// ==================== 任务看板测试 ====================

func TestCreateTask(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	dueDate := time.Now().Add(24 * time.Hour)
	task, err := cw.CreateTask(ws.ID, "完成开发", "实现所有功能", "user1", 4, &dueDate)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	if task.Title != "完成开发" {
		t.Errorf("期望任务标题为 '完成开发'，实际为 '%s'", task.Title)
	}

	if task.Status != TaskStatusTodo {
		t.Errorf("期望状态为 '%s'，实际为 '%s'", TaskStatusTodo, task.Status)
	}

	if task.Priority != 4 {
		t.Errorf("期望优先级为 4，实际为 %d", task.Priority)
	}
}

func TestCreateTaskDefaultPriority(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 0, nil)
	if task.Priority != 3 {
		t.Errorf("期望默认优先级为 3，实际为 %d", task.Priority)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	err := cw.UpdateTaskStatus(task.ID, TaskStatusInProgress, "user1")
	if err != nil {
		t.Fatalf("更新任务状态失败: %v", err)
	}

	got, _ := cw.GetTask(task.ID, "user1")
	if got.Status != TaskStatusInProgress {
		t.Errorf("期望状态为 '%s'，实际为 '%s'", TaskStatusInProgress, got.Status)
	}

	// 完成任务
	cw.UpdateTaskStatus(task.ID, TaskStatusDone, "user1")
	got, _ = cw.GetTask(task.ID, "user1")
	if got.CompletedAt == nil {
		t.Error("期望完成时间已设置")
	}
}

func TestAssignTask(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	err := cw.AssignTask(task.ID, "user2", "user1")
	if err != nil {
		t.Fatalf("分配任务失败: %v", err)
	}

	got, _ := cw.GetTask(task.ID, "user1")
	if got.AssigneeID != "user2" {
		t.Errorf("期望分配给 'user2'，实际为 '%s'", got.AssigneeID)
	}
}

func TestListTasks(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	cw.CreateTask(ws.ID, "task1", "desc", "user1", 3, nil)
	cw.CreateTask(ws.ID, "task2", "desc", "user1", 5, nil)
	cw.CreateTask(ws.ID, "task3", "desc", "user1", 1, nil)

	tasks, err := cw.ListTasks(ws.ID, "user1", "")
	if err != nil {
		t.Fatalf("列出任务失败: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("期望任务数为 3，实际为 %d", len(tasks))
	}

	// 应按优先级降序排列
	if tasks[0].Priority != 5 {
		t.Errorf("期望第一个任务优先级为 5，实际为 %d", tasks[0].Priority)
	}
}

func TestListTasksByStatus(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")

	task1, _ := cw.CreateTask(ws.ID, "task1", "desc", "user1", 3, nil)
	cw.CreateTask(ws.ID, "task2", "desc", "user1", 3, nil)

	cw.UpdateTaskStatus(task1.ID, TaskStatusInProgress, "user1")

	tasks, _ := cw.ListTasks(ws.ID, "user1", TaskStatusTodo)
	if len(tasks) != 1 {
		t.Errorf("期望 todo 任务数为 1，实际为 %d", len(tasks))
	}

	tasks, _ = cw.ListTasks(ws.ID, "user1", TaskStatusInProgress)
	if len(tasks) != 1 {
		t.Errorf("期望 in_progress 任务数为 1，实际为 %d", len(tasks))
	}
}

func TestUpdateTask(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	task, _ := cw.CreateTask(ws.ID, "原始标题", "原始描述", "user1", 3, nil)

	newDue := time.Now().Add(48 * time.Hour)
	err := cw.UpdateTask(task.ID, "user1", "新标题", "新描述", 5, &newDue)
	if err != nil {
		t.Fatalf("更新任务失败: %v", err)
	}

	got, _ := cw.GetTask(task.ID, "user1")
	if got.Title != "新标题" {
		t.Errorf("期望标题为 '新标题'，实际为 '%s'", got.Title)
	}
	if got.Priority != 5 {
		t.Errorf("期望优先级为 5，实际为 %d", got.Priority)
	}
}

func TestDeleteTask(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	err := cw.DeleteTask(task.ID, "user1")
	if err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}

	_, err = cw.GetTask(task.ID, "user1")
	if err == nil {
		t.Error("期望任务已被删除")
	}
}

func TestTaskTags(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	tag := Tag{ID: "tag1", Name: "紧急", Color: "red"}
	err := cw.AddTaskTag(task.ID, tag, "user1")
	if err != nil {
		t.Fatalf("添加标签失败: %v", err)
	}

	got, _ := cw.GetTask(task.ID, "user1")
	if len(got.Tags) != 1 {
		t.Errorf("期望标签数为 1，实际为 %d", len(got.Tags))
	}

	// 移除标签
	err = cw.RemoveTaskTag(task.ID, "tag1", "user1")
	if err != nil {
		t.Fatalf("移除标签失败: %v", err)
	}

	got, _ = cw.GetTask(task.ID, "user1")
	if len(got.Tags) != 0 {
		t.Errorf("期望标签数为 0，实际为 %d", len(got.Tags))
	}
}

func TestGetTasksByAssignee(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.AddMember(ws.ID, "user2", "user2", "member", "user1")

	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)
	cw.AssignTask(task.ID, "user2", "user1")

	tasks, err := cw.GetTasksByAssignee(ws.ID, "user2", "user1")
	if err != nil {
		t.Fatalf("获取分配任务失败: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("期望任务数为 1，实际为 %d", len(tasks))
	}
}

func TestGetTasksByTag(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	task, _ := cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	tag := Tag{ID: "bug", Name: "Bug", Color: "red"}
	cw.AddTaskTag(task.ID, tag, "user1")

	tasks, err := cw.GetTasksByTag(ws.ID, "bug", "user1")
	if err != nil {
		t.Fatalf("按标签获取任务失败: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("期望任务数为 1，实际为 %d", len(tasks))
	}
}

// ==================== 活动记录测试 ====================

func TestActivities(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.CreateDocument(ws.ID, "doc", "content", "user1")
	cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	activities, err := cw.GetActivities(ws.ID, "user1", 0)
	if err != nil {
		t.Fatalf("获取活动记录失败: %v", err)
	}

	// 至少应该有 workspace_create, document_create, task_create
	if len(activities) < 3 {
		t.Errorf("期望活动数至少为 3，实际为 %d", len(activities))
	}
}

func TestActivitiesLimit(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.CreateDocument(ws.ID, "doc1", "content", "user1")
	cw.CreateDocument(ws.ID, "doc2", "content", "user1")
	cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	activities, _ := cw.GetActivities(ws.ID, "user1", 2)
	if len(activities) != 2 {
		t.Errorf("期望活动数为 2，实际为 %d", len(activities))
	}
}

func TestActivitiesByType(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	ws, _ := cw.CreateWorkspace("ws", "desc", "user1")
	cw.CreateDocument(ws.ID, "doc", "content", "user1")
	cw.CreateTask(ws.ID, "task", "desc", "user1", 3, nil)

	activities, _ := cw.GetActivitiesByType(ws.ID, "user1", ActivityDocumentCreate)
	if len(activities) != 1 {
		t.Errorf("期望文档创建活动数为 1，实际为 %d", len(activities))
	}

	activities, _ = cw.GetActivitiesByType(ws.ID, "user1", ActivityTaskCreate)
	if len(activities) != 1 {
		t.Errorf("期望任务创建活动数为 1，实际为 %d", len(activities))
	}
}

// ==================== 综合场景测试 ====================

func TestFullWorkflow(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	// 1. 创建工作空间
	ws, err := cw.CreateWorkspace("产品研发", "产品研发工作空间", "admin")
	if err != nil {
		t.Fatalf("创建工作空间失败: %v", err)
	}

	// 2. 添加成员
	cw.AddMember(ws.ID, "dev1", "开发者1", "developer", "admin")
	cw.AddMember(ws.ID, "dev2", "开发者2", "developer", "admin")

	// 3. 创建文档
	doc, _ := cw.CreateDocument(ws.ID, "需求文档", "需求内容", "admin")

	// 4. 多人编辑
	cw.JoinEditing(doc.ID, "dev1")
	cw.JoinEditing(doc.ID, "dev2")
	cw.EditDocument(doc.ID, "更新后的需求", "dev1", "补充需求")

	// 5. 添加评论
	comment, _ := cw.AddComment(doc.ID, "dev2", "开发者2", "这个需求有疑问", "")
	cw.AddComment(doc.ID, "dev1", "开发者1", "已解答", comment.ID)

	// 6. 创建任务
	task, _ := cw.CreateTask(ws.ID, "实现登录功能", "实现用户登录", "admin", 4, nil)
	cw.AssignTask(task.ID, "dev1", "admin")
	cw.AddTaskTag(task.ID, Tag{ID: "feature", Name: "功能", Color: "blue"}, "admin")
	cw.UpdateTaskStatus(task.ID, TaskStatusInProgress, "dev1")

	// 7. 验证
	docs, _ := cw.ListDocuments(ws.ID, "admin")
	if len(docs) != 1 {
		t.Errorf("期望文档数为 1，实际为 %d", len(docs))
	}

	tasks, _ := cw.ListTasks(ws.ID, "admin", "")
	if len(tasks) != 1 {
		t.Errorf("期望任务数为 1，实际为 %d", len(tasks))
	}

	comments, _ := cw.GetComments(doc.ID, "admin")
	if len(comments) != 2 {
		t.Errorf("期望评论数为 2，实际为 %d", len(comments))
	}

	activities, _ := cw.GetActivities(ws.ID, "admin", 0)
	if len(activities) < 5 {
		t.Errorf("期望活动数至少为 5，实际为 %d", len(activities))
	}
}

// ==================== 边界情况测试 ====================

func TestNonexistentWorkspace(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	_, err := cw.GetWorkspace("nonexistent")
	if err == nil {
		t.Error("期望获取不存在的工作空间返回错误")
	}

	err = cw.DeleteWorkspace("nonexistent", "user1")
	if err == nil {
		t.Error("期望删除不存在的工作空间返回错误")
	}
}

func TestNonexistentDocument(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	_, err := cw.GetDocument("nonexistent", "user1")
	if err == nil {
		t.Error("期望获取不存在的文档返回错误")
	}

	err = cw.EditDocument("nonexistent", "content", "user1", "msg")
	if err == nil {
		t.Error("期望编辑不存在的文档返回错误")
	}
}

func TestNonexistentTask(t *testing.T) {
	cw := NewCollaborativeWorkspace()

	_, err := cw.GetTask("nonexistent", "user1")
	if err == nil {
		t.Error("期望获取不存在的任务返回错误")
	}
}

func TestPermissionLevel(t *testing.T) {
	tests := []struct {
		level    PermissionLevel
		expected string
	}{
		{PermissionNone, "none"},
		{PermissionRead, "read"},
		{PermissionWrite, "write"},
		{PermissionManage, "manage"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("PermissionLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}
