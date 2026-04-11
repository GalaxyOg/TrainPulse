package notifier

import (
	"strings"
	"testing"
)

func TestBuildTextEmojiStyle(t *testing.T) {
	n := New("https://example.com/hook", "text", false, "")
	payload := map[string]any{
		"event":          "SUCCEEDED",
		"project":        "ReinFlow",
		"job_name":       "ffsm_ft_hlgauss_ab2",
		"run_id":         "20260406-115140-66fa64d2",
		"host":           "DRLServer",
		"cwd":            "/home/yh/Algo_test/ReinFlow",
		"git_branch":     "feat_ffsm_e2e_pipeline",
		"git_commit":     "4295d75",
		"start_time":     "2026-04-06T11:51:40+00:00",
		"end_time":       "2026-04-06T22:18:44+00:00",
		"duration":       37624.071,
		"exit_code":      0,
		"log_path":       "/tmp/train.log",
		"cmd":            "python train.py",
		"last_heartbeat": "2026-04-06T22:18:00+00:00",
	}

	text := n.buildText(payload)
	required := []string{
		"✅ Task Succeeded · ReinFlow",
		"✅ event: SUCCEEDED",
		"📦 project: ReinFlow",
		"🧩 job: ffsm_ft_hlgauss_ab2",
		"🆔 run_id: 20260406-115140-66fa64d2",
		"🖥️ host: DRLServer",
		"📂 cwd: /home/yh/Algo_test/ReinFlow",
		"🌿 git: feat_ffsm_e2e_pipeline@4295d75",
		"🕒 start: 2026-04-06T11:51:40+00:00",
		"🕓 end: 2026-04-06T22:18:44+00:00",
		"⏱️ duration: 37624.071s",
		"📉 exit_code: 0",
		"📝 log: /tmp/train.log",
		"💻 cmd: python train.py",
	}
	for _, s := range required {
		if !strings.Contains(text, s) {
			t.Fatalf("text missing line: %q\nfull:\n%s", s, text)
		}
	}
}

func TestBuildMessagePostEmojiStyle(t *testing.T) {
	n := New("https://example.com/hook", "post", false, "")
	payload := map[string]any{
		"event":    "STARTED",
		"project":  "FFSM_Env",
		"job_name": "ppo_tuned_ffsm_joint7",
		"run_id":   "20260411-155342-e9a8072d",
		"host":     "DRLServer",
		"cwd":      "/home/yh/Algo_test/FFSM_Env/rl-zoo3",
		"cmd":      "python -m rl_zoo3.train",
	}
	msg := n.BuildMessage(payload)

	if got := msg["msg_type"]; got != "post" {
		t.Fatalf("msg_type=%v want post", got)
	}

	content, ok := msg["content"].(map[string]any)
	if !ok {
		t.Fatalf("content type mismatch: %T", msg["content"])
	}
	post, ok := content["post"].(map[string]any)
	if !ok {
		t.Fatalf("post type mismatch: %T", content["post"])
	}
	zh, ok := post["zh_cn"].(map[string]any)
	if !ok {
		t.Fatalf("zh_cn type mismatch: %T", post["zh_cn"])
	}

	title, _ := zh["title"].(string)
	if !strings.Contains(title, "🚀 Task Started · FFSM_Env") {
		t.Fatalf("unexpected title: %q", title)
	}

	rows, ok := zh["content"].([]any)
	if !ok {
		t.Fatalf("content rows type mismatch: %T", zh["content"])
	}
	if len(rows) < 6 {
		t.Fatalf("unexpected rows len: %d", len(rows))
	}

	firstRow, ok := rows[0].([]any)
	if !ok || len(firstRow) == 0 {
		t.Fatalf("first row type mismatch: %T", rows[0])
	}
	firstCell, ok := firstRow[0].(map[string]any)
	if !ok {
		t.Fatalf("first cell type mismatch: %T", firstRow[0])
	}
	firstText, _ := firstCell["text"].(string)
	if !strings.Contains(firstText, "🚀 Task Started · FFSM_Env") {
		t.Fatalf("unexpected first row text: %q", firstText)
	}
}
