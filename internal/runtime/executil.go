package runtime

import (
	"fmt"
	"os"
	"strings"
)

func NormalizeExitCode(code int) int {
	if code < 0 {
		return 128 + -code
	}
	return code
}

func DurationSeconds(startISO, endISO string) float64 {
	start, okStart := ParseISO(startISO)
	end, okEnd := ParseISO(endISO)
	if !okStart || !okEnd {
		return 0
	}
	d := end.Sub(start).Seconds()
	if d < 0 {
		return 0
	}
	return float64(int64(d*1000+0.5)) / 1000
}

func PIDExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscallKill(pid, 0)
	if err == nil {
		return true
	}
	if strings.Contains(err.Error(), "operation not permitted") {
		return true
	}
	return false
}

func EnsureParentDir(path string) error {
	dir := "."
	if i := strings.LastIndex(path, "/"); i >= 0 {
		dir = path[:i]
		if dir == "" {
			dir = "/"
		}
	}
	return os.MkdirAll(dir, 0o755)
}

func ValidateCommand(cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("missing command")
	}
	return nil
}
