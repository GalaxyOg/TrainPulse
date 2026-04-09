#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-v0.2.4}"
OUT_DIR="dist"
APP="trainpulse"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p "$OUT_DIR"

build_one() {
  local goos="$1"
  local goarch="$2"
  local out_name="${APP}_${VERSION#v}_${goos}_${goarch}"
  local stage_dir="${OUT_DIR}/${out_name}"
  rm -rf "$stage_dir"
  mkdir -p "$stage_dir"

  echo "[build] ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -ldflags "-s -w -X github.com/trainpulse/trainpulse/internal/version.Version=${VERSION} -X github.com/trainpulse/trainpulse/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo dev) -X github.com/trainpulse/trainpulse/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o "${stage_dir}/${APP}" ./cmd/trainpulse

  cp README.md LICENSE "$stage_dir/"
  tar -C "$OUT_DIR" -czf "${OUT_DIR}/${out_name}.tar.gz" "$out_name"
  rm -rf "$stage_dir"

  echo "[ok] ${OUT_DIR}/${out_name}.tar.gz"
}

build_one linux amd64
build_one linux arm64

echo "done: release artifacts are in ${OUT_DIR}/"
