package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSearchInput(t *testing.T) {
	p, j := parseSearchInput("p:proj-a j:job-b")
	if p != "proj-a" || j != "job-b" {
		t.Fatalf("unexpected parse result p=%q j=%q", p, j)
	}

	p2, j2 := parseSearchInput("myproj myjob")
	if p2 != "myproj" || j2 != "myjob" {
		t.Fatalf("unexpected fallback parse p=%q j=%q", p2, j2)
	}
}

func TestValidateSetup(t *testing.T) {
	ok := []setupField{
		{key: "message_type", value: "post"},
		{key: "heartbeat_minutes", value: "30"},
		{key: "dry_run", value: "false"},
		{key: "store_path", value: "~/.local/state/trainpulse/runs.db"},
		{key: "error_log_path", value: "~/.local/state/trainpulse/notifier_errors.log"},
	}
	if err := validateSetup(ok); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	bad := []setupField{
		{key: "message_type", value: "invalid"},
		{key: "heartbeat_minutes", value: "-1"},
		{key: "dry_run", value: "xx"},
		{key: "store_path", value: ""},
		{key: "error_log_path", value: ""},
	}
	if err := validateSetup(bad); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestReadTailLinesAndSummary(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "train.log")
	content := "line1\nline2\nERROR: bad things happened\ntraceback here\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := readTailLines(logPath, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	summary := extractErrorSummary(lines)
	if summary == "" || summary == "-" {
		t.Fatalf("expected non-empty summary")
	}
}
