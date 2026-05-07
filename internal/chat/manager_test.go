// Package chat 单元测试
package chat

import (
	"testing"
)

func TestCreateChannel(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{
		Name:      "general",
		Type:      ChannelTypeGroup,
		CreatorID: "user1",
	})
	if ch == nil {
		t.Fatal("频道不应为nil")
	}
	if ch.Name != "general" {
		t.Errorf("名称不匹配: %s", ch.Name)
	}
	if ch.Type != ChannelTypeGroup {
		t.Errorf("类型不匹配: %s", ch.Type)
	}
	if ch.CreatorID != "user1" {
		t.Errorf("创建者不匹配: %s", ch.CreatorID)
	}
}

func TestGetChannel(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	got, err := m.GetChannel(ch.ID)
	if err != nil {
		t.Fatalf("获取频道失败: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("名称不匹配")
	}
}

func TestGetChannelNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetChannel("nonexistent")
	if err == nil {
		t.Error("不存在的频道应返回错误")
	}
}

func TestListChannels(t *testing.T) {
	m := NewManager()
	m.CreateChannel(CreateChannelRequest{Name: "ch1", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.CreateChannel(CreateChannelRequest{Name: "ch2", Type: ChannelTypeChannel, CreatorID: "user2"})

	chs := m.ListChannels()
	if len(chs) != 2 {
		t.Errorf("期望2个频道，实际 %d", len(chs))
	}
}

func TestListChannelsByUser(t *testing.T) {
	m := NewManager()
	ch1 := m.CreateChannel(CreateChannelRequest{Name: "ch1", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.CreateChannel(CreateChannelRequest{Name: "ch2", Type: ChannelTypeGroup, CreatorID: "user2"})

	// user1 只在 ch1 中（作为创建者自动加入）
	chs := m.ListChannelsByUser("user1")
	if len(chs) != 1 {
		t.Errorf("期望1个频道，实际 %d", len(chs))
	}
	if chs[0].ID != ch1.ID {
		t.Error("频道ID不匹配")
	}
}

func TestUpdateChannel(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "old", Type: ChannelTypeGroup, CreatorID: "user1"})

	newName := "new"
	updated, err := m.UpdateChannel(ch.ID, UpdateChannelRequest{Name: &newName})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("名称未更新: %s", updated.Name)
	}
}

func TestDeleteChannel(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "to-delete", Type: ChannelTypeGroup, CreatorID: "user1"})

	err := m.DeleteChannel(ch.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = m.GetChannel(ch.ID)
	if err == nil {
		t.Error("已删除频道不应存在")
	}
}

func TestSendMessage(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	msg, err := m.SendMessage(ch.ID, SendMessageRequest{
		SenderID: "user1",
		Content:  "Hello, world!",
	})
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("消息内容不匹配: %s", msg.Content)
	}
	if msg.Type != MessageTypeText {
		t.Errorf("消息类型应为text: %s", msg.Type)
	}
}

func TestSendMessageToNonexistentChannel(t *testing.T) {
	m := NewManager()
	_, err := m.SendMessage("nonexistent", SendMessageRequest{
		SenderID: "user1",
		Content:  "test",
	})
	if err == nil {
		t.Error("向不存在的频道发送消息应返回错误")
	}
}

func TestReplyMessage(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	original, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "original"})

	reply, err := m.SendMessage(ch.ID, SendMessageRequest{
		SenderID: "user2",
		Content:  "reply",
		ReplyTo:  original.ID,
	})
	if err != nil {
		t.Fatalf("回复消息失败: %v", err)
	}
	if reply.ReplyTo != original.ID {
		t.Errorf("ReplyTo不匹配")
	}
}

func TestReplyToNonexistentMessage(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	_, err := m.SendMessage(ch.ID, SendMessageRequest{
		SenderID: "user1",
		Content:  "reply",
		ReplyTo:  "nonexistent",
	})
	if err == nil {
		t.Error("回复不存在的消息应返回错误")
	}
}

func TestGetMessages(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "msg1"})
	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user2", Content: "msg2"})
	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "msg3"})

	msgs, total, err := m.GetMessages(ch.ID, 50, 0)
	if err != nil {
		t.Fatalf("获取消息失败: %v", err)
	}
	if total != 3 {
		t.Errorf("期望3条消息，实际 %d", total)
	}
	if len(msgs) != 3 {
		t.Errorf("期望3条消息，实际 %d", len(msgs))
	}
}

func TestGetMessagesPagination(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	for i := 0; i < 10; i++ {
		m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "msg"})
	}

	msgs, total, _ := m.GetMessages(ch.ID, 3, 0)
	if total != 10 {
		t.Errorf("总数应为10，实际 %d", total)
	}
	if len(msgs) != 3 {
		t.Errorf("limit=3应返回3条，实际 %d", len(msgs))
	}

	msgs2, _, _ := m.GetMessages(ch.ID, 3, 3)
	if len(msgs2) != 3 {
		t.Errorf("offset=3, limit=3应返回3条，实际 %d", len(msgs2))
	}
}

func TestEditMessage(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "original"})

	edited, err := m.EditMessage(msg.ID, "edited content")
	if err != nil {
		t.Fatalf("编辑消息失败: %v", err)
	}
	if edited.Content != "edited content" {
		t.Errorf("内容未更新: %s", edited.Content)
	}
	if edited.EditedAt == nil {
		t.Error("EditedAt应不为nil")
	}
}

func TestDeleteMessage(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "to delete"})

	err := m.DeleteMessage(msg.ID)
	if err != nil {
		t.Fatalf("删除消息失败: %v", err)
	}

	// 已删除的消息不应出现在 GetMessages 结果中
	msgs, _, _ := m.GetMessages(ch.ID, 50, 0)
	for _, m := range msgs {
		if m.ID == msg.ID {
			t.Error("已删除消息不应出现在列表中")
		}
	}
}

func TestAddMember(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	member, err := m.AddMember(ch.ID, AddMemberRequest{UserID: "user2", Role: MemberRoleMember})
	if err != nil {
		t.Fatalf("添加成员失败: %v", err)
	}
	if member.UserID != "user2" {
		t.Errorf("用户ID不匹配: %s", member.UserID)
	}
	if member.Role != MemberRoleMember {
		t.Errorf("角色不匹配: %s", member.Role)
	}
}

func TestAddDuplicateMember(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	_, err := m.AddMember(ch.ID, AddMemberRequest{UserID: "user1"})
	if err == nil {
		t.Error("添加重复成员应返回错误")
	}
}

func TestRemoveMember(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.AddMember(ch.ID, AddMemberRequest{UserID: "user2"})

	err := m.RemoveMember(ch.ID, "user2")
	if err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}

	members, _ := m.ListMembers(ch.ID)
	for _, mem := range members {
		if mem.UserID == "user2" {
			t.Error("已移除成员不应存在")
		}
	}
}

func TestUpdateMemberRole(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.AddMember(ch.ID, AddMemberRequest{UserID: "user2", Role: MemberRoleMember})

	updated, err := m.UpdateMemberRole(ch.ID, "user2", MemberRoleAdmin)
	if err != nil {
		t.Fatalf("更新角色失败: %v", err)
	}
	if updated.Role != MemberRoleAdmin {
		t.Errorf("角色未更新: %s", updated.Role)
	}
}

func TestListMembers(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.AddMember(ch.ID, AddMemberRequest{UserID: "user2"})
	m.AddMember(ch.ID, AddMemberRequest{UserID: "user3"})

	members, err := m.ListMembers(ch.ID)
	if err != nil {
		t.Fatalf("列出成员失败: %v", err)
	}
	// user1 (创建者) + user2 + user3 = 3
	if len(members) != 3 {
		t.Errorf("期望3个成员，实际 %d", len(members))
	}
}

func TestAddReaction(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "hello"})

	err := m.AddReaction(msg.ID, AddReactionRequest{Emoji: "👍", UserID: "user2"})
	if err != nil {
		t.Fatalf("添加反应失败: %v", err)
	}
}

func TestAddDuplicateReaction(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "hello"})

	m.AddReaction(msg.ID, AddReactionRequest{Emoji: "👍", UserID: "user2"})
	err := m.AddReaction(msg.ID, AddReactionRequest{Emoji: "👍", UserID: "user2"})
	if err == nil {
		t.Error("重复反应应返回错误")
	}
}

func TestRemoveReaction(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "hello"})

	m.AddReaction(msg.ID, AddReactionRequest{Emoji: "👍", UserID: "user2"})
	err := m.RemoveReaction(msg.ID, RemoveReactionRequest{Emoji: "👍", UserID: "user2"})
	if err != nil {
		t.Fatalf("移除反应失败: %v", err)
	}
}

func TestRemoveNonexistentReaction(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "hello"})

	err := m.RemoveReaction(msg.ID, RemoveReactionRequest{Emoji: "👍", UserID: "user2"})
	if err == nil {
		t.Error("移除不存在的反应应返回错误")
	}
}

func TestMarkAsRead(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "hello"})

	err := m.MarkAsRead(ch.ID, "user1")
	if err != nil {
		t.Fatalf("标记已读失败: %v", err)
	}
}

func TestGetUnreadCount(t *testing.T) {
	m := NewManager()
	// 先创建频道（user1 LastRead = 创建时间），再添加 user2
	ch2 := m.CreateChannel(CreateChannelRequest{Name: "test2", Type: ChannelTypeGroup, CreatorID: "user1"})
	m.AddMember(ch2.ID, AddMemberRequest{UserID: "user2"})

	// 发送消息
	m.SendMessage(ch2.ID, SendMessageRequest{SenderID: "user1", Content: "msg1"})
	m.SendMessage(ch2.ID, SendMessageRequest{SenderID: "user1", Content: "msg2"})

	// user2 应有2条未读
	counts := m.GetUnreadCount("user2")
	found := false
	for _, c := range counts {
		if c.ChannelID == ch2.ID {
			found = true
			if c.Count != 2 {
				t.Errorf("期望2条未读，实际 %d", c.Count)
			}
		}
	}
	if !found {
		t.Error("应有 test2 频道的未读计数")
	}

	// 标记已读后应为0
	m.MarkAsRead(ch2.ID, "user2")
	counts = m.GetUnreadCount("user2")
	for _, c := range counts {
		if c.ChannelID == ch2.ID {
			t.Error("标记已读后不应有未读")
		}
	}
}

func TestSearchMessages(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "Hello world"})
	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "Go programming"})
	m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "Hello Go"})

	results := m.SearchMessages("Hello", "")
	if len(results) != 2 {
		t.Errorf("搜索Hello应有2条结果，实际 %d", len(results))
	}
}

func TestSearchMessagesByChannel(t *testing.T) {
	m := NewManager()
	ch1 := m.CreateChannel(CreateChannelRequest{Name: "ch1", Type: ChannelTypeGroup, CreatorID: "user1"})
	ch2 := m.CreateChannel(CreateChannelRequest{Name: "ch2", Type: ChannelTypeGroup, CreatorID: "user1"})

	m.SendMessage(ch1.ID, SendMessageRequest{SenderID: "user1", Content: "test message"})
	m.SendMessage(ch2.ID, SendMessageRequest{SenderID: "user1", Content: "test message"})

	results := m.SearchMessages("test", ch1.ID)
	if len(results) != 1 {
		t.Errorf("限定频道搜索应有1条结果，实际 %d", len(results))
	}
}

func TestSearchDeletedMessages(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})
	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{SenderID: "user1", Content: "to be deleted"})
	m.DeleteMessage(msg.ID)

	results := m.SearchMessages("deleted", "")
	if len(results) != 0 {
		t.Error("已删除消息不应出现在搜索结果中")
	}
}

func TestCreateDirectChannel(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{
		Name:      "DM",
		Type:      ChannelTypeDirect,
		CreatorID: "user1",
	})
	if ch.Type != ChannelTypeDirect {
		t.Errorf("类型应为direct: %s", ch.Type)
	}
}

func TestCreateChannelType(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{
		Name:      "announce",
		Type:      ChannelTypeChannel,
		CreatorID: "user1",
	})
	if ch.Type != ChannelTypeChannel {
		t.Errorf("类型应为channel: %s", ch.Type)
	}
}

func TestSendMessageWithCustomType(t *testing.T) {
	m := NewManager()
	ch := m.CreateChannel(CreateChannelRequest{Name: "test", Type: ChannelTypeGroup, CreatorID: "user1"})

	msg, _ := m.SendMessage(ch.ID, SendMessageRequest{
		SenderID: "user1",
		Content:  "photo.jpg",
		Type:     MessageTypeImage,
	})
	if msg.Type != MessageTypeImage {
		t.Errorf("类型应为image: %s", msg.Type)
	}
}
