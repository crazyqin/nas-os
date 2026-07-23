package web

import "testing"

func TestProductRegistry_PutGetRemove(t *testing.T) {
	r := newProductRegistry()
	stopped := false
	r.put("docker", "mgr-a", func() { stopped = true })
	if r.get("docker") != "mgr-a" {
		t.Fatal("get")
	}
	if !r.has("docker") {
		t.Fatal("has")
	}
	// replace invokes old stop
	r.put("docker", "mgr-b", nil)
	if !stopped {
		t.Fatal("old stop should run on replace")
	}
	if r.get("docker") != "mgr-b" {
		t.Fatal("replaced holder")
	}
	r.drop("docker")
	if r.has("docker") {
		t.Fatal("drop")
	}
	// remove with stop
	stopped2 := false
	r.put("photos", "p", func() { stopped2 = true })
	if !r.remove("photos") {
		t.Fatal("remove")
	}
	if !stopped2 {
		t.Fatal("remove should stop")
	}
}

func TestProductRegistry_ClearAll(t *testing.T) {
	r := newProductRegistry()
	n := 0
	r.put("a", 1, func() { n++ })
	r.put("b", 2, func() { n++ })
	r.clearAll()
	if n != 2 {
		t.Fatalf("stops=%d", n)
	}
	if len(r.ids()) != 0 {
		t.Fatal("ids empty")
	}
}
