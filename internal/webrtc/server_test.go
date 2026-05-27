package webrtc

import (
	"testing"
)

func TestNewWebRTCServer(t *testing.T) {
	srv := NewWebRTCServer(nil)
	if srv == nil {
		t.Fatal("nil")
	}
	if srv.config.MaxSessions != 100 {
		t.Errorf("expected 100, got %d", srv.config.MaxSessions)
	}
}

func TestCreateAndCloseSession(t *testing.T) {
	srv := NewWebRTCServer(nil)
	pc, err := srv.CreateSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	if pc.State != StateNew {
		t.Errorf("expected new, got %s", pc.State)
	}
	srv.CloseSession("s1")
	stats := srv.GetStats()
	if stats.ActiveSessions != 0 {
		t.Errorf("expected 0, got %d", stats.ActiveSessions)
	}
}

func TestMaxSessions(t *testing.T) {
	config := &WebRTCConfig{MaxSessions: 1}
	srv := NewWebRTCServer(config)
	srv.CreateSession("s1")
	_, err := srv.CreateSession("s2")
	if err == nil {
		t.Error("expected error")
	}
}

func TestSignaling(t *testing.T) {
	srv := NewWebRTCServer(nil)
	srv.CreateSession("s1")

	offer := &SessionDescription{Type: "offer", SDP: "v=0\r\n..."}
	srv.SetRemoteSDP("s1", offer)

	answer := &SessionDescription{Type: "answer", SDP: "v=0\r\n..."}
	srv.SetLocalSDP("s1", answer)

	candidate := &ICECandidate{Candidate: "candidate:1", SDPMid: "0"}
	srv.AddICECandidate("s1", candidate)

	pc, _ := srv.GetSession("s1")
	if len(pc.ICECandidates) != 1 {
		t.Error("expected 1 candidate")
	}
}

func TestStream(t *testing.T) {
	srv := NewWebRTCServer(nil)
	tracks := []*MediaTrack{
		{ID: "t1", Kind: MediaVideo, Label: "camera1", Width: 1920, Height: 1080},
	}
	stream := srv.CreateStream("st1", "Camera 1", "camera", tracks)
	if stream.Name != "Camera 1" {
		t.Error("wrong name")
	}
	got, _ := srv.GetStream("st1")
	if got.ID != "st1" {
		t.Error("wrong id")
	}
}

func TestListStreams(t *testing.T) {
	srv := NewWebRTCServer(nil)
	srv.CreateStream("s1", "Cam1", "camera", nil)
	srv.CreateStream("s2", "Cam2", "camera", nil)
	streams := srv.ListStreams()
	if len(streams) != 2 {
		t.Errorf("expected 2, got %d", len(streams))
	}
}

func TestRecording(t *testing.T) {
	srv := NewWebRTCServer(nil)
	tracks := []*MediaTrack{{ID: "t1", Kind: MediaVideo}}
	srv.CreateStream("st1", "Cam", "camera", tracks)

	rec, err := srv.StartRecording("r1", "st1", "webm", "/recordings/r1.webm")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "recording" {
		t.Errorf("expected recording, got %s", rec.Status)
	}

	rec, _ = srv.StopRecording("r1")
	if rec.Status != "completed" {
		t.Errorf("expected completed, got %s", rec.Status)
	}
}

func TestRecordingStreamNotFound(t *testing.T) {
	srv := NewWebRTCServer(nil)
	_, err := srv.StartRecording("r1", "nonexistent", "webm", "/tmp/test")
	if err == nil {
		t.Error("expected error")
	}
}

func TestConnectSession(t *testing.T) {
	srv := NewWebRTCServer(nil)
	srv.CreateSession("s1")
	srv.ConnectSession("s1")
	sessions := srv.ListSessions()
	if sessions[0].State != StateConnected {
		t.Errorf("expected connected, got %s", sessions[0].State)
	}
}

func TestStats(t *testing.T) {
	srv := NewWebRTCServer(nil)
	srv.CreateSession("s1")
	srv.CreateStream("st1", "Cam", "camera", nil)
	srv.StartRecording("r1", "st1", "webm", "/tmp/test")

	stats := srv.GetStats()
	if stats.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", stats.TotalSessions)
	}
	if stats.TotalStreams != 1 {
		t.Errorf("expected 1 stream, got %d", stats.TotalStreams)
	}
	if stats.TotalRecordings != 1 {
		t.Errorf("expected 1 recording, got %d", stats.TotalRecordings)
	}
}

func TestSessionNotFound(t *testing.T) {
	srv := NewWebRTCServer(nil)
	err := srv.CloseSession("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}
