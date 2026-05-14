package notifycenter

import (
	"testing"
	"time"
)

func TestSendAndList(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "Test", Body: "Hello", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.Send(&Notification{ID: "n2", Title: "Test2", Body: "World", Channel: ChannelWebhook, Priority: PriorityHigh})

	all := mgr.List(ListOptions{})
	if len(all) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(all))
	}
}

func TestListUnreadOnly(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "A", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.Send(&Notification{ID: "n2", Title: "B", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.MarkRead("n1")

	unread := mgr.List(ListOptions{UnreadOnly: true})
	if len(unread) != 1 {
		t.Errorf("expected 1 unread, got %d", len(unread))
	}
	if unread[0].ID != "n2" {
		t.Errorf("expected n2, got %s", unread[0].ID)
	}
}

func TestListByChannel(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "A", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.Send(&Notification{ID: "n2", Title: "B", Channel: ChannelWebhook, Priority: PriorityNormal})

	emailOnly := mgr.List(ListOptions{Channel: ChannelEmail})
	if len(emailOnly) != 1 {
		t.Errorf("expected 1 email, got %d", len(emailOnly))
	}
}

func TestListWithLimit(t *testing.T) {
	mgr := NewManager()
	for i := 0; i < 10; i++ {
		mgr.Send(&Notification{ID: "n" + string(rune('0'+i)), Title: "T", Channel: ChannelEmail, Priority: PriorityNormal})
	}
	limited := mgr.List(ListOptions{Limit: 3})
	if len(limited) != 3 {
		t.Errorf("expected 3, got %d", len(limited))
	}
}

func TestMarkRead(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "T", Channel: ChannelEmail, Priority: PriorityNormal})
	if !mgr.MarkRead("n1") {
		t.Error("expected MarkRead to return true")
	}
	if mgr.UnreadCount() != 0 {
		t.Errorf("expected 0 unread, got %d", mgr.UnreadCount())
	}
}

func TestMarkReadNotFound(t *testing.T) {
	mgr := NewManager()
	if mgr.MarkRead("nonexistent") {
		t.Error("expected false for nonexistent")
	}
}

func TestMarkAllRead(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "A", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.Send(&Notification{ID: "n2", Title: "B", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.Send(&Notification{ID: "n3", Title: "C", Channel: ChannelEmail, Priority: PriorityNormal})

	count := mgr.MarkAllRead()
	if count != 3 {
		t.Errorf("expected 3 marked, got %d", count)
	}
	if mgr.UnreadCount() != 0 {
		t.Errorf("expected 0 unread, got %d", mgr.UnreadCount())
	}
}

func TestDeleteNotification(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "T", Channel: ChannelEmail, Priority: PriorityNormal})
	if !mgr.DeleteNotification("n1") {
		t.Error("expected delete to return true")
	}
	if len(mgr.List(ListOptions{})) != 0 {
		t.Error("expected empty list after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	mgr := NewManager()
	if mgr.DeleteNotification("nonexistent") {
		t.Error("expected false")
	}
}

func TestChannels(t *testing.T) {
	mgr := NewManager()
	ch := &Channel{ID: "email-1", Name: "Gmail", Type: ChannelEmail, Enabled: true}
	mgr.AddChannel(ch)

	got, ok := mgr.GetChannel("email-1")
	if !ok {
		t.Fatal("expected channel to exist")
	}
	if got.Name != "Gmail" {
		t.Errorf("expected Gmail, got %s", got.Name)
	}

	chs := mgr.ListChannels()
	if len(chs) != 1 {
		t.Errorf("expected 1 channel, got %d", len(chs))
	}
}

func TestTemplates(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{ID: "t1", Name: "Alert", Channel: ChannelEmail, Subject: "Alert: {{.Title}}", Body: "{{.Body}}"}
	mgr.AddTemplate(tmpl)

	got, ok := mgr.GetTemplate("t1")
	if !ok {
		t.Fatal("expected template to exist")
	}
	if got.Name != "Alert" {
		t.Errorf("expected Alert, got %s", got.Name)
	}
}

func TestPreferences(t *testing.T) {
	mgr := NewManager()
	pref := &Preference{
		UserID:          "user1",
		EnabledChannels: []ChannelType{ChannelEmail, ChannelWeChat},
		MinPriority:     PriorityHigh,
		QuietHoursStart: "22:00",
		QuietHoursEnd:   "08:00",
	}
	mgr.SetPreference(pref)

	got, ok := mgr.GetPreference("user1")
	if !ok {
		t.Fatal("expected preference to exist")
	}
	if got.MinPriority != PriorityHigh {
		t.Errorf("expected high, got %s", got.MinPriority)
	}
}

func TestStats(t *testing.T) {
	mgr := NewManager()
	mgr.Send(&Notification{ID: "n1", Title: "A", Channel: ChannelEmail, Priority: PriorityNormal})
	mgr.Send(&Notification{ID: "n2", Title: "B", Channel: ChannelWebhook, Priority: PriorityNormal})
	mgr.MarkRead("n1")

	stats := mgr.GetStats()
	if stats["total"] != 2 {
		t.Errorf("expected total=2, got %v", stats["total"])
	}
	if stats["unread"] != 1 {
		t.Errorf("expected unread=1, got %v", stats["unread"])
	}
}

func TestNotificationTimestamps(t *testing.T) {
	mgr := NewManager()
	before := time.Now()
	mgr.Send(&Notification{ID: "n1", Title: "T", Channel: ChannelEmail, Priority: PriorityNormal})
	notif := mgr.List(ListOptions{})[0]
	if notif.SentAt.Before(before) {
		t.Error("SentAt should be set")
	}
	if notif.Read {
		t.Error("should not be read initially")
	}
}
