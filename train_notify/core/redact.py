from __future__ import annotations

import re


def default_redact_patterns() -> list[str]:
    return [
        r"(?i)(--?(?:token|secret|password|apikey|api-key)\s+)(\S+)",
        r"(?i)((?:token|secret|password|apikey|api_key)\s*=\s*)([^\s]+)",
        r"(?i)(bearer\s+)([A-Za-z0-9._~+/=-]+)",
    ]


def redact_text(value: str, custom_patterns: list[str] | None = None) -> str:
    text = value
    patterns = default_redact_patterns()
    if custom_patterns:
        patterns.extend(custom_patterns)

    def _replace(match: re.Match[str]) -> str:
        if match.lastindex and match.lastindex >= 1:
            prefix = match.group(1)
            if prefix:
                return f"{prefix}[REDACTED]"
        return "[REDACTED]"

    for pattern in patterns:
        try:
            text = re.sub(pattern, _replace, text)
        except re.error:
            continue
    return text
