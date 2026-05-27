package mqttbroker

import (
	"testing"
	"time"
)

func TestNewBroker(t *testing.T) {
	broker := NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker returned nil")
	}
	if broker.config.Port != 1883 {
		t.Errorf("expected port 1883, got %d", broker.config.Port)
	}
}

func TestConnectClient(t *testing.T) {
	broker := NewBroker(nil)
	client, err := broker.ConnectClient("client1", "user1", true, 60)
	if err != nil {
		t.Fatalf("ConnectClient failed: %v", err)
	}
	if client.ClientID != "client1" {
		t.Errorf("expected client1, got %s", client.ClientID)
	}
	if client.State != ClientConnected {
		t.Errorf("expected connected, got %s", client.State)
	}
}

func TestDisconnectClient(t *testing.T) {
	broker := NewBroker(nil)
	broker.ConnectClient("client1", "user1", true, 60)
	err := broker.DisconnectClient("client1")
	if err != nil {
		t.Fatalf("DisconnectClient failed: %v", err)
	}
	stats := broker.GetStats()
	if stats.ConnectedClients != 0 {
		t.Errorf("expected 0 connected, got %d", stats.ConnectedClients)
	}
}

func TestSubscribeAndPublish(t *testing.T) {
	broker := NewBroker(nil)
	broker.ConnectClient("client1", "user1", true, 60)

	err := broker.Subscribe("client1", "home/temperature", QoS1)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	broker.Publish("home/temperature", []byte("25.5"), QoS1, false)

	stats := broker.GetStats()
	if stats.TotalMessages != 1 {
		t.Errorf("expected 1 message, got %d", stats.TotalMessages)
	}
}

func TestRetainedMessage(t *testing.T) {
	broker := NewBroker(nil)
	broker.ConnectClient("client1", "user1", true, 60)

	broker.Publish("home/humidity", []byte("60"), QoS0, true)

	msgs := broker.GetRetainedMessages()
	if len(msgs) != 1 {
		t.Errorf("expected 1 retained, got %d", len(msgs))
	}
	if string(msgs[0].Payload) != "60" {
		t.Errorf("expected 60, got %s", string(msgs[0].Payload))
	}
}

func TestUnsubscribe(t *testing.T) {
	broker := NewBroker(nil)
	broker.ConnectClient("client1", "user1", true, 60)
	broker.Subscribe("client1", "test/topic", QoS0)
	broker.Unsubscribe("client1", "test/topic")

	client := broker.ListClients()[0]
	if len(client.Subscriptions) != 0 {
		t.Errorf("expected 0 subs, got %d", len(client.Subscriptions))
	}
}

func TestTopicMatching(t *testing.T) {
	tests := []struct {
		pattern string
		topic   string
		match   bool
	}{
		{"home/temp", "home/temp", true},
		{"home/+/temp", "home/living/temp", true},
		{"home/#", "home/living/temp", true},
		{"home/temp", "home/humidity", false},
		{"home/+/temp", "home/temp", false},
	}
	for _, tt := range tests {
		got := matchTopic(tt.pattern, tt.topic)
		if got != tt.match {
			t.Errorf("matchTopic(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.match)
		}
	}
}

func TestMaxClients(t *testing.T) {
	config := &BrokerConfig{MaxClients: 2}
	broker := NewBroker(config)
	broker.ConnectClient("c1", "u1", true, 60)
	broker.ConnectClient("c2", "u2", true, 60)
	_, err := broker.ConnectClient("c3", "u3", true, 60)
	if err == nil {
		t.Error("expected error for max clients")
	}
}

func TestListClients(t *testing.T) {
	broker := NewBroker(nil)
	broker.ConnectClient("c1", "u1", true, 60)
	broker.ConnectClient("c2", "u2", true, 60)
	clients := broker.ListClients()
	if len(clients) != 2 {
		t.Errorf("expected 2, got %d", len(clients))
	}
}

func TestWillMessage(t *testing.T) {
	broker := NewBroker(nil)
	client, _ := broker.ConnectClient("c1", "u1", true, 60)
	client.WillTopic = "status/c1"
	client.WillMessage = []byte("offline")
	client.WillQoS = QoS1

	broker.Subscribe("c2", "status/c1", QoS1)
	broker.ConnectClient("c2", "u2", true, 60)

	broker.DisconnectClient("c1")

	time.Sleep(10 * time.Millisecond)
}
