// Package websocket 提供消息广播功能测试
package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TestBroadcaster_CreateRoom 测试创建房间.
func TestBroadcaster_CreateRoom(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)
	b.Start()
	defer b.Stop()

	room, err := b.CreateRoom("room-1", "Test Room")
	if err != nil {
		t.Fatalf("创建房间失败: %v", err)
	}

	if room.ID != "room-1" {
		t.Errorf("房间 ID 错误: got %s, want room-1", room.ID)
	}

	// 重复创建应该失败
	_, err = b.CreateRoom("room-1", "Another Room")
	if err == nil {
		t.Error("重复创建房间应该失败")
	}
}

// TestBroadcaster_DeleteRoom 测试删除房间.
func TestBroadcaster_DeleteRoom(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)

	b.CreateRoom("room-1", "Test Room")
	err := b.DeleteRoom("room-1")
	if err != nil {
		t.Fatalf("删除房间失败: %v", err)
	}

	// 删除不存在的房间
	err = b.DeleteRoom("room-1")
	if err == nil {
		t.Error("删除不存在的房间应该失败")
	}
}

// TestBroadcaster_JoinLeaveRoom 测试加入和离开房间.
func TestBroadcaster_JoinLeaveRoom(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)

	b.CreateRoom("room-1", "Test Room")

	client := &Client{
		ID:        "client-1",
		Name:      "Test Client",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}

	// 加入房间
	err := b.JoinRoom("room-1", "client-1", client)
	if err != nil {
		t.Fatalf("加入房间失败: %v", err)
	}

	// 离开房间
	err = b.LeaveRoom("room-1", "client-1")
	if err != nil {
		t.Fatalf("离开房间失败: %v", err)
	}

	// 离开不存在的房间
	err = b.LeaveRoom("nonexistent", "client-1")
	if err == nil {
		t.Error("离开不存在的房间应该失败")
	}
}

// TestBroadcaster_TopicSubscribe 测试主题订阅.
func TestBroadcaster_TopicSubscribe(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)

	// 订阅主题
	err := b.SubscribeTopic("client-1", "topic-1")
	if err != nil {
		t.Fatalf("订阅主题失败: %v", err)
	}

	// 检查订阅
	topics := b.GetClientTopics("client-1")
	if len(topics) != 1 || topics[0] != "topic-1" {
		t.Errorf("主题订阅错误: got %v, want [topic-1]", topics)
	}

	// 取消订阅
	err = b.UnsubscribeTopic("client-1", "topic-1")
	if err != nil {
		t.Fatalf("取消订阅失败: %v", err)
	}

	topics = b.GetClientTopics("client-1")
	if len(topics) != 0 {
		t.Errorf("取消订阅后不应有主题: got %v", topics)
	}
}

// TestBroadcaster_BroadcastToRoom 测试房间广播.
func TestBroadcaster_BroadcastToRoom(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)
	b.Start()
	defer b.Stop()

	b.CreateRoom("room-1", "Test Room")

	// 创建多个客户端
	var wg sync.WaitGroup
	receivedCount := 0
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		client := &Client{
			ID:        string(rune('a' + i)),
			Name:      "Client " + string(rune('a'+i)),
			Connected: true,
			JoinedAt:  time.Now(),
			sendChan:  make(chan *BroadcastMessage, 10),
		}

		b.JoinRoom("room-1", client.ID, client)

		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			select {
			case msg := <-c.sendChan:
				mu.Lock()
				receivedCount++
				mu.Unlock()
				_ = msg
			case <-time.After(time.Second):
			}
		}(client)
	}

	// 广播消息
	data, _ := json.Marshal(map[string]string{"message": "hello"})
	msg := &BroadcastMessage{
		Type: "test",
		Data: data,
	}

	err := b.BroadcastToRoom("room-1", msg)
	if err != nil {
		t.Fatalf("广播失败: %v", err)
	}

	wg.Wait()

	if receivedCount != 3 {
		t.Errorf("广播接收数量错误: got %d, want 3", receivedCount)
	}
}

// TestBroadcaster_BroadcastToTopic 测试主题广播.
func TestBroadcaster_BroadcastToTopic(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)
	b.Start()
	defer b.Stop()

	// 创建房间和客户端
	b.CreateRoom("room-1", "Test Room")

	client1 := &Client{
		ID:        "client-1",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}
	client2 := &Client{
		ID:        "client-2",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}

	b.JoinRoom("room-1", "client-1", client1)
	b.JoinRoom("room-1", "client-2", client2)

	// 只有 client-1 订阅 topic-1
	b.SubscribeTopic("client-1", "topic-1")
	b.SubscribeTopic("client-2", "topic-2")

	// 广播到 topic-1
	data, _ := json.Marshal(map[string]string{"message": "hello topic"})
	msg := &BroadcastMessage{
		Type: "test",
		Data: data,
	}

	err := b.BroadcastToTopic("topic-1", msg)
	if err != nil {
		t.Fatalf("广播到主题失败: %v", err)
	}

	// 只有 client-1 应该收到消息
	select {
	case <-client1.sendChan:
		// 正确
	case <-time.After(time.Second):
		t.Error("client-1 应该收到消息")
	}

	select {
	case <-client2.sendChan:
		t.Error("client-2 不应该收到 topic-1 的消息")
	case <-time.After(100 * time.Millisecond):
		// 正确
	}
}

// TestBroadcaster_RoomTopicBinding 测试房间主题绑定.
func TestBroadcaster_RoomTopicBinding(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)

	b.CreateRoom("room-1", "Test Room")

	err := b.BindRoomTopic("room-1", "topic-1")
	if err != nil {
		t.Fatalf("绑定房间主题失败: %v", err)
	}

	topics := b.GetRoomTopics("room-1")
	if len(topics) != 1 || topics[0] != "topic-1" {
		t.Errorf("房间主题绑定错误: got %v, want [topic-1]", topics)
	}

	err = b.UnbindRoomTopic("room-1", "topic-1")
	if err != nil {
		t.Fatalf("解绑房间主题失败: %v", err)
	}

	topics = b.GetRoomTopics("room-1")
	if len(topics) != 0 {
		t.Errorf("解绑后不应有主题: got %v", topics)
	}
}

// TestBroadcaster_Stats 测试统计功能.
func TestBroadcaster_Stats(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)
	b.Start()
	defer b.Stop()

	// 创建房间和客户端
	b.CreateRoom("room-1", "Test Room")

	client := &Client{
		ID:        "client-1",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}
	b.JoinRoom("room-1", "client-1", client)
	b.SubscribeTopic("client-1", "topic-1")

	stats := b.Stats()
	if stats.RoomCount != 1 {
		t.Errorf("房间数量错误: got %d, want 1", stats.RoomCount)
	}
	if stats.TopicCount != 1 {
		t.Errorf("主题数量错误: got %d, want 1", stats.TopicCount)
	}
}

// TestBroadcaster_History 测试历史记录.
func TestBroadcaster_HroadcastHistory(t *testing.T) {
	config := DefaultBroadcasterConfig
	config.EnableHistory = true
	b := NewBroadcaster(config)
	b.Start()
	defer b.Stop()

	b.CreateRoom("room-1", "Test Room")

	client := &Client{
		ID:        "client-1",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 100),
	}
	b.JoinRoom("room-1", "client-1", client)

	// 发送多条消息
	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(map[string]int{"index": i})
		msg := &BroadcastMessage{
			Type: "test",
			Data: data,
		}
		b.BroadcastToRoom("room-1", msg)
	}

	// 等待消息处理
	time.Sleep(100 * time.Millisecond)

	// 获取历史
	history, err := b.GetRoomHistory("room-1", 3)
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("历史记录数量错误: got %d, want 3", len(history))
	}
}

// TestBroadcaster_LeaveAllRooms 测试离开所有房间.
func TestBroadcaster_LeaveAllRooms(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)

	b.CreateRoom("room-1", "Room 1")
	b.CreateRoom("room-2", "Room 2")

	client := &Client{
		ID:        "client-1",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}

	b.JoinRoom("room-1", "client-1", client)
	b.JoinRoom("room-2", "client-1", client)
	b.SubscribeTopic("client-1", "topic-1")

	// 离开所有房间
	b.LeaveAllRooms("client-1")

	// 检查是否已从所有房间移除
	clients1, _ := b.GetRoomClients("room-1")
	clients2, _ := b.GetRoomClients("room-2")

	for _, c := range clients1 {
		if c.ID == "client-1" {
			t.Error("client-1 应该已从 room-1 移除")
		}
	}
	for _, c := range clients2 {
		if c.ID == "client-1" {
			t.Error("client-1 应该已从 room-2 移除")
		}
	}
}

// TestBroadcaster_ExcludeClients 测试排除客户端.
func TestBroadcaster_ExcludeClients(t *testing.T) {
	b := NewBroadcaster(DefaultBroadcasterConfig)
	b.Start()
	defer b.Stop()

	b.CreateRoom("room-1", "Test Room")

	client1 := &Client{
		ID:        "client-1",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}
	client2 := &Client{
		ID:        "client-2",
		Connected: true,
		JoinedAt:  time.Now(),
		sendChan:  make(chan *BroadcastMessage, 10),
	}

	b.JoinRoom("room-1", "client-1", client1)
	b.JoinRoom("room-1", "client-2", client2)

	// 广播并排除 client-1
	data, _ := json.Marshal(map[string]string{"message": "hello"})
	msg := &BroadcastMessage{
		Type:    "test",
		Data:    data,
		Exclude: []string{"client-1"},
	}

	b.BroadcastToRoom("room-1", msg)

	// 等待消息处理
	time.Sleep(50 * time.Millisecond)

	// client-1 不应该收到消息
	select {
	case <-client1.sendChan:
		t.Error("client-1 不应该收到消息")
	default:
		// 正确
	}

	// client-2 应该收到消息
	select {
	case <-client2.sendChan:
		// 正确
	default:
		t.Error("client-2 应该收到消息")
	}
}
