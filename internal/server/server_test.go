package server

import "testing"

func TestStateTransitions(t *testing.T) {
	t.Parallel()
	srv := NewServer(nil, "", Server{ID: "x", state: StateStopped})

	if err := srv.SetState(StateStarting); err != nil {
		t.Fatalf("expected valid transition: %v", err)
	}
	if err := srv.SetState(StateStopped); err == nil {
		t.Fatal("expected invalid transition error")
	}
}
