#!/usr/bin/env bash
# Windows, macOS ve Linux için tek dosyalık binary üretir.
set -euo pipefail
cd "$(dirname "$0")"
export CGO_ENABLED=0

mkdir -p dist

echo "Windows (amd64) derleniyor..."
GOOS=windows GOARCH=amd64 go build -o dist/aidat-takip-windows-amd64.exe .

echo "macOS (Apple Silicon) derleniyor..."
GOOS=darwin GOARCH=arm64 go build -o dist/aidat-takip-macos-arm64 .

echo "macOS (Intel) derleniyor..."
GOOS=darwin GOARCH=amd64 go build -o dist/aidat-takip-macos-amd64 .

echo "Linux (amd64) derleniyor..."
GOOS=linux GOARCH=amd64 go build -o dist/aidat-takip-linux-amd64 .

echo "Tamamlandı. Dosyalar: dist/"
ls -la dist/
