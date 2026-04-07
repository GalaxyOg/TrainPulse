package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultConfigPath      = "~/.config/trainpulse/config.toml"
	DefaultStorePath       = "~/.local/state/trainpulse/runs.db"
	DefaultErrorLogPath    = "~/.local/state/trainpulse/notifier_errors.log"
	DefaultHeartbeatMinute = 30
)

type FileConfig struct {
	WebhookURL       string
	MessageType      string
	StorePath        string
	ErrorLogPath     string
	HeartbeatMinutes int
	DryRun           *bool
	Redact           []string
}

type Runtime struct {
	ConfigPath        string
	WebhookURL        string
	MessageType       string
	StorePath         string
	ErrorLogPath      string
	HeartbeatMinutes  int
	DryRun            bool
	Redact            []string
	DryRunExplicitCLI bool
}

type RuntimeInput struct {
	ConfigPath       string
	WebhookURL       *string
	MessageType      *string
	StorePath        *string
	ErrorLogPath     *string
	HeartbeatMinutes *int
	DryRun           *bool
	Redact           *[]string
}

func ExpandPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func ParseBoolEnv(s string) (bool, bool) {
	v := strings.TrimSpace(strings.ToLower(s))
	switch v {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func parseTomlValue(raw string) any {
	value := strings.TrimSpace(raw)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	lower := strings.ToLower(value)
	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return []string{}
		}
		parts := splitCSV(inner)
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			v := parseTomlValue(p)
			s, ok := v.(string)
			if ok {
				out = append(out, s)
			}
		}
		return out
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return value
}

func splitCSV(s string) []string {
	parts := make([]string, 0)
	cur := strings.Builder{}
	inSingle := false
	inDouble := false
	for _, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			cur.WriteRune(r)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			cur.WriteRune(r)
		case ',':
			if inSingle || inDouble {
				cur.WriteRune(r)
				continue
			}
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		parts = append(parts, strings.TrimSpace(cur.String()))
	}
	return parts
}

func LoadFile(path string) (FileConfig, error) {
	filePath := ExpandPath(path)
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, err
	}
	defer f.Close()

	section := ""
	cfg := FileConfig{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != "" && section != "trainpulse" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := parseTomlValue(strings.TrimSpace(kv[1]))
		switch key {
		case "webhook_url":
			if s, ok := val.(string); ok {
				cfg.WebhookURL = s
			}
		case "message_type":
			if s, ok := val.(string); ok {
				cfg.MessageType = strings.ToLower(s)
			}
		case "store_path":
			if s, ok := val.(string); ok {
				cfg.StorePath = s
			}
		case "error_log_path":
			if s, ok := val.(string); ok {
				cfg.ErrorLogPath = s
			}
		case "heartbeat_minutes":
			if i, ok := val.(int); ok {
				cfg.HeartbeatMinutes = i
			}
		case "dry_run":
			if b, ok := val.(bool); ok {
				cfg.DryRun = &b
			}
		case "redact":
			if arr, ok := val.([]string); ok {
				cfg.Redact = append([]string{}, arr...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return FileConfig{}, err
	}
	return cfg, nil
}

func validateMessageType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "post" || s == "text" {
		return s
	}
	return "text"
}

func ResolveRuntime(in RuntimeInput) (Runtime, error) {
	cfgPath := in.ConfigPath
	if strings.TrimSpace(cfgPath) == "" {
		cfgPath = DefaultConfigPath
	}
	fileCfg, err := LoadFile(cfgPath)
	if err != nil {
		return Runtime{}, fmt.Errorf("load config: %w", err)
	}
	env := os.Environ()
	envMap := map[string]string{}
	for _, item := range env {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	pickString := func(cli *string, envKey, fileValue, def string) string {
		if cli != nil {
			return *cli
		}
		if v, ok := envMap[envKey]; ok && v != "" {
			return v
		}
		if fileValue != "" {
			return fileValue
		}
		return def
	}

	heartbeat := DefaultHeartbeatMinute
	if in.HeartbeatMinutes != nil {
		heartbeat = *in.HeartbeatMinutes
	} else if v := envMap["TRAINPULSE_HEARTBEAT_MINUTES"]; v != "" {
		if iv, err := strconv.Atoi(v); err == nil {
			heartbeat = iv
		}
	} else if fileCfg.HeartbeatMinutes > 0 {
		heartbeat = fileCfg.HeartbeatMinutes
	}
	if heartbeat <= 0 {
		heartbeat = DefaultHeartbeatMinute
	}

	dryRun := false
	if in.DryRun != nil {
		dryRun = *in.DryRun
	} else if v := envMap["TRAINPULSE_DRY_RUN"]; v != "" {
		if parsed, ok := ParseBoolEnv(v); ok {
			dryRun = parsed
		}
	} else if fileCfg.DryRun != nil {
		dryRun = *fileCfg.DryRun
	}

	redact := []string{}
	if in.Redact != nil {
		redact = append(redact, (*in.Redact)...)
	} else if v := envMap["TRAINPULSE_REDACT"]; strings.TrimSpace(v) != "" {
		for _, x := range strings.Split(v, ",") {
			x = strings.TrimSpace(x)
			if x != "" {
				redact = append(redact, x)
			}
		}
	} else if len(fileCfg.Redact) > 0 {
		redact = append(redact, fileCfg.Redact...)
	}

	storePath := ExpandPath(pickString(in.StorePath, "TRAINPULSE_STORE_PATH", fileCfg.StorePath, DefaultStorePath))
	errorLogPath := ExpandPath(pickString(in.ErrorLogPath, "TRAINPULSE_ERROR_LOG_PATH", fileCfg.ErrorLogPath, DefaultErrorLogPath))

	r := Runtime{
		ConfigPath:        ExpandPath(cfgPath),
		WebhookURL:        pickString(in.WebhookURL, "TRAINPULSE_WEBHOOK_URL", fileCfg.WebhookURL, ""),
		MessageType:       validateMessageType(pickString(in.MessageType, "TRAINPULSE_MESSAGE_TYPE", fileCfg.MessageType, "text")),
		StorePath:         storePath,
		ErrorLogPath:      errorLogPath,
		HeartbeatMinutes:  heartbeat,
		DryRun:            dryRun,
		Redact:            redact,
		DryRunExplicitCLI: in.DryRun != nil,
	}
	return r, nil
}

func ExampleConfig() string {
	return `[trainpulse]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/your-webhook-token"
message_type = "post"
store_path = "~/.local/state/trainpulse/runs.db"
error_log_path = "~/.local/state/trainpulse/notifier_errors.log"
heartbeat_minutes = 30
dry_run = false
redact = ["(?i)(token=)\\S+"]
`
}
