package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/trainpulse/trainpulse/internal/runtime"
)

type FeishuNotifier struct {
	WebhookURL     string
	MessageType    string
	DryRun         bool
	Retries        int
	TimeoutSeconds int
	ErrorLogPath   string
	Client         *http.Client
}

func New(webhookURL, messageType string, dryRun bool, errorLogPath string) *FeishuNotifier {
	if messageType != "post" && messageType != "text" {
		messageType = "text"
	}
	return &FeishuNotifier{
		WebhookURL:     webhookURL,
		MessageType:    messageType,
		DryRun:         dryRun,
		Retries:        3,
		TimeoutSeconds: 8,
		ErrorLogPath:   errorLogPath,
		Client:         &http.Client{Timeout: 8 * time.Second},
	}
}

func eventStyle(event string) (string, string) {
	switch event {
	case "STARTED":
		return "[START]", "Task Started"
	case "SUCCEEDED":
		return "[OK]", "Task Succeeded"
	case "FAILED":
		return "[FAIL]", "Task Failed"
	case "INTERRUPTED":
		return "[INT]", "Task Interrupted"
	case "STOPPED":
		return "[STOP]", "Task Stopped"
	case "HEARTBEAT":
		return "[HB]", "Task Heartbeat"
	default:
		return "[EVT]", "Task Event"
	}
}

func (n *FeishuNotifier) writeError(message string) {
	if n.ErrorLogPath == "" {
		return
	}
	_ = os.MkdirAll(dirOf(n.ErrorLogPath), 0o755)
	fp, err := os.OpenFile(n.ErrorLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer fp.Close()
	_, _ = fp.WriteString(runtime.NowISO() + " " + message + "\n")
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

func (n *FeishuNotifier) buildText(payload map[string]any) string {
	event := asString(payload["event"])
	tag, label := eventStyle(event)
	lines := []string{
		fmt.Sprintf("%s [%s] %s | %s", tag, event, label, asStringDefault(payload["project"], "-")),
		fmt.Sprintf("job: %s", asStringDefault(payload["job_name"], "-")),
		fmt.Sprintf("run_id: %s", asStringDefault(payload["run_id"], "-")),
		fmt.Sprintf("host: %s", asStringDefault(payload["host"], "-")),
	}
	if s := asString(payload["cwd"]); s != "" {
		lines = append(lines, "cwd: "+s)
	}
	if b := asString(payload["git_branch"]); b != "" || asString(payload["git_commit"]) != "" {
		lines = append(lines, fmt.Sprintf("git: %s@%s", defaultDash(b), defaultDash(asString(payload["git_commit"]))))
	}
	if s := asString(payload["start_time"]); s != "" {
		lines = append(lines, "start: "+s)
	}
	if s := asString(payload["end_time"]); s != "" {
		lines = append(lines, "end: "+s)
	}
	if ec := payload["exit_code"]; ec != nil {
		lines = append(lines, fmt.Sprintf("exit_code: %v", ec))
	}
	if d := payload["duration"]; d != nil {
		lines = append(lines, fmt.Sprintf("duration: %vs", d))
	}
	if s := asString(payload["log_path"]); s != "" {
		lines = append(lines, "log: "+s)
	}
	if s := asString(payload["cmd"]); s != "" {
		lines = append(lines, "cmd: "+s)
	}
	return joinLines(lines)
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}

func asString(v any) string {
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func asStringDefault(v any, def string) string {
	s := asString(v)
	if s == "" {
		return def
	}
	return s
}

func defaultDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (n *FeishuNotifier) BuildMessage(payload map[string]any) map[string]any {
	event := asString(payload["event"])
	tag, label := eventStyle(event)
	if n.MessageType == "post" {
		content := []any{
			[]any{map[string]any{"tag": "text", "text": fmt.Sprintf("%s event: %s", tag, event)}},
			[]any{map[string]any{"tag": "text", "text": fmt.Sprintf("project: %s", asStringDefault(payload["project"], "-"))}},
			[]any{map[string]any{"tag": "text", "text": fmt.Sprintf("job: %s", asStringDefault(payload["job_name"], "-"))}},
			[]any{map[string]any{"tag": "text", "text": fmt.Sprintf("run_id: %s", asStringDefault(payload["run_id"], "-"))}},
			[]any{map[string]any{"tag": "text", "text": fmt.Sprintf("host: %s", asStringDefault(payload["host"], "-"))}},
			[]any{map[string]any{"tag": "text", "text": fmt.Sprintf("cwd: %s", asStringDefault(payload["cwd"], "-"))}},
		}
		if b := asString(payload["git_branch"]); b != "" || asString(payload["git_commit"]) != "" {
			content = append(content, []any{map[string]any{"tag": "text", "text": fmt.Sprintf("git: %s@%s", defaultDash(b), defaultDash(asString(payload["git_commit"])))}})
		}
		if s := asString(payload["start_time"]); s != "" {
			content = append(content, []any{map[string]any{"tag": "text", "text": "start: " + s}})
		}
		if s := asString(payload["end_time"]); s != "" {
			content = append(content, []any{map[string]any{"tag": "text", "text": "end: " + s}})
		}
		if d := payload["duration"]; d != nil {
			content = append(content, []any{map[string]any{"tag": "text", "text": fmt.Sprintf("duration: %vs", d)}})
		}
		if ec := payload["exit_code"]; ec != nil {
			content = append(content, []any{map[string]any{"tag": "text", "text": fmt.Sprintf("exit_code: %v", ec)}})
		}
		if s := asString(payload["log_path"]); s != "" {
			content = append(content, []any{map[string]any{"tag": "text", "text": "log: " + s}})
		}
		if s := asString(payload["cmd"]); s != "" {
			content = append(content, []any{map[string]any{"tag": "text", "text": "cmd: " + s}})
		}
		return map[string]any{
			"msg_type": "post",
			"content": map[string]any{
				"post": map[string]any{
					"zh_cn": map[string]any{
						"title":   fmt.Sprintf("%s %s · %s", tag, label, asStringDefault(payload["project"], "-")),
						"content": content,
					},
				},
			},
		}
	}
	return map[string]any{
		"msg_type": "text",
		"content":  map[string]any{"text": n.buildText(payload)},
	}
}

func (n *FeishuNotifier) Send(payload map[string]any) bool {
	body := n.BuildMessage(payload)
	event := asStringDefault(payload["event"], "UNKNOWN")
	if n.DryRun {
		fmt.Fprintf(os.Stderr, "[trainpulse][dry-run][%s] %s\n", event, n.buildText(payload))
		raw, _ := json.Marshal(body)
		fmt.Fprintf(os.Stderr, "[trainpulse][dry-run][payload] %s\n", raw)
		return true
	}
	if n.WebhookURL == "" {
		msg := fmt.Sprintf("[trainpulse][notify][%s] webhook_url is empty, skip notification", event)
		fmt.Fprintln(os.Stderr, msg)
		n.writeError(msg)
		return false
	}
	raw, err := json.Marshal(body)
	if err != nil {
		n.writeError("marshal body failed: " + err.Error())
		return false
	}
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: time.Duration(n.TimeoutSeconds) * time.Second}
	}
	if n.Retries < 1 {
		n.Retries = 1
	}
	delay := time.Second
	for attempt := 1; attempt <= n.Retries; attempt++ {
		req, _ := http.NewRequest(http.MethodPost, n.WebhookURL, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				parsed := map[string]any{}
				if len(b) == 0 || json.Unmarshal(b, &parsed) == nil {
					if code := fmt.Sprintf("%v", parsed["code"]); code == "" || code == "0" || code == "<nil>" {
						return true
					}
					err = fmt.Errorf("feishu code=%v msg=%v", parsed["code"], parsed["msg"])
				} else {
					err = fmt.Errorf("invalid json response: %s", string(b))
				}
			} else {
				err = fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
			}
		}
		n.writeError(fmt.Sprintf("attempt=%d send failed: %v", attempt, err))
		if attempt < n.Retries {
			time.Sleep(delay)
			delay *= 2
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
		}
	}
	fmt.Fprintf(os.Stderr, "[trainpulse][notify][%s] delivery failed after %d attempt(s)\n", event, n.Retries)
	return false
}
