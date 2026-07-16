package logcenter

import "testing"

func TestClassifyRebootReason(t *testing.T) {
	cases := map[string]RebootReason{
		"kernel panic watchdog reset": RebootReasonKernelPanic,
		"scheduled upgrade window":    RebootReasonScheduledUpdate,
		"UPS power loss":              RebootReasonPowerLoss,
		"admin reboot command":        RebootReasonUserInitiated,
		"":                            RebootReasonUnknown,
	}
	for input, want := range cases {
		if got := ClassifyRebootReason(input); got != want {
			t.Fatalf("ClassifyRebootReason(%q)=%s want %s", input, got, want)
		}
	}
}

func TestRebootHistoryNewestFirstAndLimit(t *testing.T) {
	h := NewRebootHistory(2)
	h.Add(RebootEvent{Node: "n1", Details: "admin reboot command"})
	h.Add(RebootEvent{Node: "n2", Details: "scheduled upgrade"})
	h.Add(RebootEvent{Node: "n3", Details: "kernel panic"})

	events := h.List(0)
	if len(events) != 2 {
		t.Fatalf("len=%d want 2", len(events))
	}
	if events[0].Node != "n3" || events[0].Reason != RebootReasonKernelPanic {
		t.Fatalf("unexpected newest event: %+v", events[0])
	}
}
