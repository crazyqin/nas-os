package web

import "testing"

func TestHolderBag_SetGetClear(t *testing.T) {
	s := &Server{h: newHolderBag()}
	s.setHolder("dockerMgr", "mgr")
	if !s.hasHolder("dockerMgr") {
		t.Fatal("has")
	}
	if holderAs[string](s, "dockerMgr") != "mgr" {
		t.Fatal("get")
	}
	s.setHolder("dockerMgr", nil)
	if s.hasHolder("dockerMgr") {
		t.Fatal("cleared")
	}
	if holderAs[string](s, "dockerMgr") != "" {
		t.Fatal("zero")
	}
}
