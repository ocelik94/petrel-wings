package server

import "testing"

func TestStateTransitions(t *testing.T) {
	t.Parallel()

	for from, allowed := range validTransitions {
		for to := range allowed {
			srv := NewServer(nil, "", Server{ID: "x", state: from})
			if err := srv.SetState(to); err != nil {
				t.Fatalf("expected valid transition %q -> %q: %v", from, to, err)
			}
		}
	}

	srv := NewServer(nil, "", Server{ID: "x", state: StateStarting})
	if err := srv.SetState(StateStopped); err == nil {
		t.Fatal("expected invalid transition error from starting to stopped")
	}

	same := NewServer(nil, "", Server{ID: "x", state: StateRunning})
	if err := same.SetState(StateRunning); err != nil {
		t.Fatalf("expected same-state transition to be no-op: %v", err)
	}
}
