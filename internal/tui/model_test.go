package tui

import "testing"

func TestChipToStatus(t *testing.T) {
	if chipToStatus("running") != "RUNNING" {
		t.Fatalf("running mapping failed")
	}
	if chipToStatus("all") != "" {
		t.Fatalf("all should map to empty")
	}
}

func TestMoreRecent(t *testing.T) {
	a := "2026-04-07T10:00:00+08:00"
	b := "2026-04-07T09:00:00+08:00"
	if !moreRecent(a, b) {
		t.Fatalf("expected a newer than b")
	}
	if moreRecent("", b) {
		t.Fatalf("empty should not be newer")
	}
}
