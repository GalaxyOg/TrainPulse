package doctor

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/trainpulse/trainpulse/internal/config"
	"github.com/trainpulse/trainpulse/internal/store"
)

type Item struct {
	Name    string
	OK      bool
	Message string
}

type Report struct {
	Items []Item
}

func (r Report) AllOK() bool {
	for _, it := range r.Items {
		if !it.OK {
			return false
		}
	}
	return true
}

func Run(rt config.Runtime) Report {
	items := []Item{}

	cfgPath := rt.ConfigPath
	if _, err := os.Stat(cfgPath); err == nil {
		items = append(items, Item{Name: "config-file", OK: true, Message: cfgPath})
	} else if os.IsNotExist(err) {
		items = append(items, Item{Name: "config-file", OK: true, Message: "not found (allowed)"})
	} else {
		items = append(items, Item{Name: "config-file", OK: false, Message: err.Error()})
	}

	if _, err := config.LoadFile(rt.ConfigPath); err != nil {
		items = append(items, Item{Name: "config-parse", OK: false, Message: err.Error()})
	} else {
		items = append(items, Item{Name: "config-parse", OK: true, Message: "ok"})
	}

	if st, err := store.New(rt.StorePath); err != nil {
		items = append(items, Item{Name: "sqlite-store", OK: false, Message: err.Error()})
	} else {
		_ = st.Close()
		items = append(items, Item{Name: "sqlite-store", OK: true, Message: rt.StorePath})
	}

	items = append(items, checkBin("git", true))
	items = append(items, checkBin("tmux", false))

	if rt.WebhookURL == "" {
		items = append(items, Item{Name: "webhook-url", OK: true, Message: "empty (notification disabled unless dry-run)"})
	} else if u, err := url.Parse(rt.WebhookURL); err != nil || u.Scheme == "" || u.Host == "" {
		items = append(items, Item{Name: "webhook-url", OK: false, Message: "invalid URL"})
	} else {
		items = append(items, Item{Name: "webhook-url", OK: true, Message: sanitizeURL(rt.WebhookURL)})
	}

	if err := ensureWritable(rt.ErrorLogPath); err != nil {
		items = append(items, Item{Name: "error-log", OK: false, Message: err.Error()})
	} else {
		items = append(items, Item{Name: "error-log", OK: true, Message: rt.ErrorLogPath})
	}

	return Report{Items: items}
}

func checkBin(name string, required bool) Item {
	p, err := exec.LookPath(name)
	if err != nil {
		if required {
			return Item{Name: "bin-" + name, OK: false, Message: "not found"}
		}
		return Item{Name: "bin-" + name, OK: true, Message: "not found (optional)"}
	}
	return Item{Name: "bin-" + name, OK: true, Message: p}
}

func sanitizeURL(u string) string {
	if idx := strings.Index(u, "/hook/"); idx >= 0 {
		return u[:idx+6] + "***"
	}
	return u
}

func ensureWritable(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	fp, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return fp.Close()
}
