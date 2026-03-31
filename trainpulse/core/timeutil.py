from __future__ import annotations

from datetime import datetime, timedelta, timezone


UTC_PLUS_8 = timezone(timedelta(hours=8))


def now_iso() -> str:
    return datetime.now(UTC_PLUS_8).isoformat()


def now_compact() -> str:
    return datetime.now(UTC_PLUS_8).strftime("%Y%m%d-%H%M%S")
