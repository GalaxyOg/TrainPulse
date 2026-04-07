package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeartbeatDefault30(t *testing.T) {
	t.Setenv("TRAINPULSE_HEARTBEAT_MINUTES", "")
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("[trainpulse]\nmessage_type='text'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := ResolveRuntime(RuntimeInput{ConfigPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	if rt.HeartbeatMinutes != 30 {
		t.Fatalf("heartbeat_minutes=%d want 30", rt.HeartbeatMinutes)
	}
}

func TestPriorityCLIOverEnvOverFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	content := []byte("[trainpulse]\nwebhook_url='https://file.example/hook'\nmessage_type='post'\n")
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAINPULSE_WEBHOOK_URL", "https://env.example/hook")
	t.Setenv("TRAINPULSE_MESSAGE_TYPE", "text")
	rt, err := ResolveRuntime(RuntimeInput{ConfigPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	if rt.WebhookURL != "https://env.example/hook" {
		t.Fatalf("webhook=%s", rt.WebhookURL)
	}
	if rt.MessageType != "text" {
		t.Fatalf("message_type=%s", rt.MessageType)
	}
	cliURL := "https://cli.example/hook"
	rt2, err := ResolveRuntime(RuntimeInput{ConfigPath: cfgPath, WebhookURL: &cliURL})
	if err != nil {
		t.Fatal(err)
	}
	if rt2.WebhookURL != cliURL {
		t.Fatalf("webhook=%s want %s", rt2.WebhookURL, cliURL)
	}
}
