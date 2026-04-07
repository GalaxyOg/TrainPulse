package context

import "regexp"

func RedactText(input string, patterns []string) string {
	out := input
	for _, p := range patterns {
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		out = re.ReplaceAllString(out, "[REDACTED]")
	}
	return out
}
