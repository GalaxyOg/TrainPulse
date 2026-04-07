package runtime

import "time"

var utcPlus8 = time.FixedZone("UTC+8", 8*3600)

func NowISO() string {
	return time.Now().In(utcPlus8).Format(time.RFC3339)
}

func NowCompact() string {
	return time.Now().In(utcPlus8).Format("20060102-150405")
}

func ParseISO(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, true
	}
	// tolerate fractional seconds formats
	t, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t, true
	}
	return time.Time{}, false
}
