package localchat

import (
	"testing"
	"time"
)

func TestNewChatService(t *testing.T) {
	config := ChatConfig{
		MaxFileSize:         50 * 1024 * 1024,
		MaxMessageLength:    4096,
		HistoryRetention:    90 * 24 * time.Hour,
		EnableEncryption:    true,
		EnableAI:            true,
		AIModel:             "local-llm",
		WebRTCEnabled:       true,
		MaxMeetingDuration:  4 * time.Hour,
	}
	svc := NewChatService(config)
	if svc == nil {
		t.Fatal("NewChatService returned nil")
	}
	status := svc.GetServiceStatus()
	if status["users"] != 0 {
		t.Errorf("expected 0 users, got %v", status["users"])
	}
	if status["channels"] != 0 {
		t.Errorf("expected 0 channels, got %v", status["channels"])
	}
	if status["messages"] != 0 {
		t.Errorf("expected 0 messages, got %v", status["messages"])
	}
	if status["meetings"] != 0 {
		t.Errorf("expected 0 meetings, got %v", status["meetings"])
	}
	if status["ai_enabled"] != true {
		t.Errorf("expected ai_enabled true, got %v", status["ai_enabled"])
	}
}

func TestNewChatServiceWithoutAI(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})
	status := svc.GetServiceStatus()
	if status["ai_enabled"] != false {
		t.Errorf("expected ai_enabled false, got %v", status["ai_enabled"])
	}
}

func TestServiceStartStop(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})
	err := svc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	err = svc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRegisterUser(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: true,
		AIModel:  "local-llm",
	})

	user, err := svc.RegisterUser("alice", "Alice Smith", "alice@example.com")
	if err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got %q", user.Username)
	}
	if user.FullName != "Alice Smith" {
		t.Errorf("expected fullName 'Alice Smith', got %q", user.FullName)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email 'alice@example.com', got %q", user.Email)
	}
	if user.Status != "online" {
		t.Errorf("expected status 'online', got %q", user.Status)
	}
	if !user.AIEnabled {
		t.Error("expected AIEnabled true when AI is enabled")
	}
	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}

	status := svc.GetServiceStatus()
	if status["users"] != 1 {
		t.Errorf("expected 1 user, got %v", status["users"])
	}
}

func TestRegisterDuplicateUser(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	_, err := svc.RegisterUser("bob", "Bob", "bob@example.com")
	if err != nil {
		t.Fatalf("first RegisterUser failed: %v", err)
	}
	_, err = svc.RegisterUser("bob", "Bob Again", "bob2@example.com")
	if err == nil {
		t.Error("expected error for duplicate username, got nil")
	}
}

func TestCreateChannel(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")

	channel, err := svc.CreateChannel("general", ChannelGroup, user.ID, false)
	if err != nil {
		t.Fatalf("CreateChannel failed: %v", err)
	}
	if channel.Name != "general" {
		t.Errorf("expected name 'general', got %q", channel.Name)
	}
	if channel.Type != ChannelGroup {
		t.Errorf("expected type %q, got %q", ChannelGroup, channel.Type)
	}
	if channel.OwnerID != user.ID {
		t.Errorf("expected ownerID %q, got %q", user.ID, channel.OwnerID)
	}
	if len(channel.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(channel.Members))
	}
	if channel.Members[0] != user.ID {
		t.Errorf("expected member to be %q, got %q", user.ID, channel.Members[0])
	}
	if !channel.IsPrivate == false { // just checking the value
		// this is fine
	}
}

func TestCreateChannelUserNotFound(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	_, err := svc.CreateChannel("general", ChannelGroup, "nonexistent-user", false)
	if err == nil {
		t.Error("expected error for nonexistent user, got nil")
	}
}

func TestSendMessage(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})
	svc.Start()
	defer svc.Stop()

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	channel, _ := svc.CreateChannel("general", ChannelGroup, user.ID, false)

	msg, err := svc.SendMessage(channel.ID, user.ID, MessageText, "Hello, world!")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %q", msg.Content)
	}
	if msg.ChannelID != channel.ID {
		t.Errorf("expected channelID %q, got %q", channel.ID, msg.ChannelID)
	}
	if msg.SenderID != user.ID {
		t.Errorf("expected senderID %q, got %q", user.ID, msg.SenderID)
	}
	if msg.Type != MessageText {
		t.Errorf("expected type %q, got %q", MessageText, msg.Type)
	}
	if msg.ID == "" {
		t.Error("expected non-empty message ID")
	}
}

func TestSendMessageChannelNotFound(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")

	_, err := svc.SendMessage("nonexistent-channel", user.ID, MessageText, "Hello")
	if err == nil {
		t.Error("expected error for nonexistent channel, got nil")
	}
}

func TestSendMessageNotMember(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})
	svc.Start()
	defer svc.Stop()

	owner, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	nonMember, _ := svc.RegisterUser("bob", "Bob", "bob@example.com")
	channel, _ := svc.CreateChannel("general", ChannelGroup, owner.ID, false)

	_, err := svc.SendMessage(channel.ID, nonMember.ID, MessageText, "Sneaky message")
	if err == nil {
		t.Error("expected error for non-member sending message, got nil")
	}
}

func TestGetMessages(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})
	svc.Start()
	defer svc.Stop()

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	channel, _ := svc.CreateChannel("general", ChannelGroup, user.ID, false)

	// Send multiple messages
	for i := 0; i < 5; i++ {
		svc.SendMessage(channel.ID, user.ID, MessageText, "Message")
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Wait for messages to be processed
	time.Sleep(200 * time.Millisecond)

	// Get all messages
	msgs, err := svc.GetMessages(channel.ID, 10, nil)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}

	// Get with limit
	msgs, err = svc.GetMessages(channel.ID, 2, nil)
	if err != nil {
		t.Fatalf("GetMessages with limit failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages with limit, got %d", len(msgs))
	}
}

func TestGetMessagesChannelNotFound(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	_, err := svc.GetMessages("nonexistent", 10, nil)
	if err == nil {
		t.Error("expected error for nonexistent channel, got nil")
	}
}

func TestScheduleMeeting(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI:          true,
		AIModel:           "local-llm",
		MaxMeetingDuration: 2 * time.Hour,
	})

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")

	meeting, err := svc.ScheduleMeeting("Weekly Standup", user.ID, time.Now().Add(1*time.Hour), 10)
	if err != nil {
		t.Fatalf("ScheduleMeeting failed: %v", err)
	}
	if meeting.Title != "Weekly Standup" {
		t.Errorf("expected title 'Weekly Standup', got %q", meeting.Title)
	}
	if meeting.HostID != user.ID {
		t.Errorf("expected hostID %q, got %q", user.ID, meeting.HostID)
	}
	if meeting.Status != MeetingScheduled {
		t.Errorf("expected status %q, got %q", MeetingScheduled, meeting.Status)
	}
	if meeting.MaxParticipants != 10 {
		t.Errorf("expected maxParticipants 10, got %d", meeting.MaxParticipants)
	}
	if meeting.AITranscript != true {
		t.Error("expected AITranscript true when AI is enabled")
	}
	if meeting.ID == "" {
		t.Error("expected non-empty meeting ID")
	}

	status := svc.GetServiceStatus()
	if status["meetings"] != 1 {
		t.Errorf("expected 1 meeting, got %v", status["meetings"])
	}
}

func TestStartMeeting(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	meeting, _ := svc.ScheduleMeeting("Test Meeting", user.ID, time.Now(), 10)

	err := svc.StartMeeting(meeting.ID)
	if err != nil {
		t.Fatalf("StartMeeting failed: %v", err)
	}
}

func TestStartMeetingNotFound(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	err := svc.StartMeeting("nonexistent-meeting")
	if err == nil {
		t.Error("expected error for nonexistent meeting, got nil")
	}
}

func TestStartMeetingInvalidStatus(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	meeting, _ := svc.ScheduleMeeting("Test Meeting", user.ID, time.Now(), 10)

	// Start once
	svc.StartMeeting(meeting.ID)

	// Try to start again
	err := svc.StartMeeting(meeting.ID)
	if err == nil {
		t.Error("expected error when starting already active meeting, got nil")
	}
}

func TestJoinMeeting(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	host, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	participant, _ := svc.RegisterUser("bob", "Bob", "bob@example.com")
	meeting, _ := svc.ScheduleMeeting("Test Meeting", host.ID, time.Now(), 10)

	svc.StartMeeting(meeting.ID)

	err := svc.JoinMeeting(meeting.ID, participant.ID)
	if err != nil {
		t.Fatalf("JoinMeeting failed: %v", err)
	}
}

func TestJoinMeetingNotActive(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	host, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	participant, _ := svc.RegisterUser("bob", "Bob", "bob@example.com")
	meeting, _ := svc.ScheduleMeeting("Test Meeting", host.ID, time.Now(), 10)

	// Don't start the meeting, try to join
	err := svc.JoinMeeting(meeting.ID, participant.ID)
	if err == nil {
		t.Error("expected error when joining non-active meeting, got nil")
	}
}

func TestJoinMeetingFull(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	host, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	meeting, _ := svc.ScheduleMeeting("Tiny Meeting", host.ID, time.Now(), 1)

	svc.StartMeeting(meeting.ID)
	svc.JoinMeeting(meeting.ID, host.ID) // host joins as first participant

	user2, _ := svc.RegisterUser("bob", "Bob", "bob@example.com")
	err := svc.JoinMeeting(meeting.ID, user2.ID)
	if err == nil {
		t.Error("expected error when meeting is full, got nil")
	}
}

func TestJoinMeetingNotFound(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	err := svc.JoinMeeting("nonexistent", user.ID)
	if err == nil {
		t.Error("expected error for nonexistent meeting, got nil")
	}
}

func TestAISummarizeMessages(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: true,
		AIModel:  "local-llm",
	})
	svc.Start()
	defer svc.Stop()

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	channel, _ := svc.CreateChannel("general", ChannelGroup, user.ID, false)

	svc.SendMessage(channel.ID, user.ID, MessageText, "Hello everyone")
	svc.SendMessage(channel.ID, user.ID, MessageText, "How is the project going?")
	time.Sleep(200 * time.Millisecond)

	summary, err := svc.AISummarizeMessages(channel.ID, 1*time.Hour)
	if err != nil {
		t.Fatalf("AISummarizeMessages failed: %v", err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestAISummarizeNoAI(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	_, err := svc.AISummarizeMessages("any-channel", 1*time.Hour)
	if err == nil {
		t.Error("expected error when AI not enabled, got nil")
	}
}

func TestAISmartReply(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: true,
		AIModel:  "local-llm",
	})
	svc.Start()
	defer svc.Stop()

	user, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	channel, _ := svc.CreateChannel("general", ChannelGroup, user.ID, false)

	svc.SendMessage(channel.ID, user.ID, MessageText, "What do you think?")
	time.Sleep(200 * time.Millisecond)

	replies, err := svc.AISmartReply(channel.ID)
	if err != nil {
		t.Fatalf("AISmartReply failed: %v", err)
	}
	if len(replies) == 0 {
		t.Error("expected non-empty reply suggestions")
	}
}

func TestAISmartReplyNoAI(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: false,
	})

	_, err := svc.AISmartReply("any-channel")
	if err == nil {
		t.Error("expected error when AI not enabled, got nil")
	}
}

func TestGetServiceStatus(t *testing.T) {
	svc := NewChatService(ChatConfig{
		EnableAI: true,
		AIModel:  "local-llm",
	})
	svc.Start()
	defer svc.Stop()

	user1, _ := svc.RegisterUser("alice", "Alice", "alice@example.com")
	svc.RegisterUser("bob", "Bob", "bob@example.com")

	ch1, _ := svc.CreateChannel("general", ChannelGroup, user1.ID, false)
	ch2, _ := svc.CreateChannel("direct", ChannelDirect, user1.ID, true)

	svc.SendMessage(ch1.ID, user1.ID, MessageText, "Hello")
	svc.SendMessage(ch2.ID, user1.ID, MessageText, "Secret message")
	svc.SendMessage(ch1.ID, user1.ID, MessageText, "Hi there")
	time.Sleep(2 * time.Second)

	svc.ScheduleMeeting("Meeting 1", user1.ID, time.Now().Add(1*time.Hour), 5)

	status := svc.GetServiceStatus()
	if status["users"] != 2 {
		t.Errorf("expected 2 users, got %v", status["users"])
	}
	if status["channels"] != 2 {
		t.Errorf("expected 2 channels, got %v", status["channels"])
	}
	if status["messages"] != 3 {
		t.Errorf("expected 3 messages, got %v", status["messages"])
	}
	if status["meetings"] != 1 {
		t.Errorf("expected 1 meeting, got %v", status["meetings"])
	}
	if status["ai_enabled"] != true {
		t.Errorf("expected ai_enabled true, got %v", status["ai_enabled"])
	}
}

func TestGenerateIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if id == "" {
			t.Fatal("generateID returned empty string")
		}
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}
