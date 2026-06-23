package cmd

import (
	"encoding/json"
	"testing"
)

func TestCheckInboxResponse_WithUnread(t *testing.T) {
	got := checkInboxResponse(3)
	if got == "" {
		t.Fatal("expected non-empty response for count=3")
	}

	var resp hookResponse
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Decision != "block" {
		t.Errorf("decision = %q, want %q", resp.Decision, "block")
	}
	if resp.Reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestCheckInboxResponse_NoUnread(t *testing.T) {
	got := checkInboxResponse(0)
	if got != "" {
		t.Errorf("expected empty response for count=0, got %q", got)
	}
}

func TestCheckInboxResponse_NegativeCount(t *testing.T) {
	got := checkInboxResponse(-1)
	if got != "" {
		t.Errorf("expected empty response for count=-1, got %q", got)
	}
}
