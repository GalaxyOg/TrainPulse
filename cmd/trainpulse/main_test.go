package main

import (
	"strings"
	"testing"
)

func TestResolveTmuxSessionNameWithCheckerExplicitSession(t *testing.T) {
	name, err := resolveTmuxSessionNameWithChecker("exp1", "rid-1", func(_ string) bool { return false })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "exp1" {
		t.Fatalf("expected exp1, got %s", name)
	}
}

func TestResolveTmuxSessionNameWithCheckerAutoSession(t *testing.T) {
	name, err := resolveTmuxSessionNameWithChecker("", "rid-1", func(_ string) bool { return false })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "trainpulse-rid-1"; name != want {
		t.Fatalf("expected %s, got %s", want, name)
	}
}

func TestResolveTmuxSessionNameWithCheckerAutoSessionCollision(t *testing.T) {
	seen := map[string]bool{
		"trainpulse-rid-1":   true,
		"trainpulse-rid-1-1": true,
	}
	name, err := resolveTmuxSessionNameWithChecker("", "rid-1", func(s string) bool { return seen[s] })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "trainpulse-rid-1-2"; name != want {
		t.Fatalf("expected %s, got %s", want, name)
	}
}

func TestResolveTmuxSessionNameWithCheckerRejectsExistingExplicitSession(t *testing.T) {
	_, err := resolveTmuxSessionNameWithChecker("exp1", "rid-1", func(s string) bool { return s == "exp1" })
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWrapCommandKeepSessionOnFailure(t *testing.T) {
	wrapped := wrapCommandKeepSessionOnFailure("echo hi")
	required := []string{
		"echo hi",
		"tp_exit=$?",
		"keeping tmux session for debugging",
		`exec "${SHELL:-/bin/bash}"`,
		`exit "$tp_exit"`,
	}
	for _, s := range required {
		if !strings.Contains(wrapped, s) {
			t.Fatalf("wrapped command missing %q: %s", s, wrapped)
		}
	}
}
