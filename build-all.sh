#!/usr/bin/env bash
set -e

echo "=================================================================="
echo "🚀 SpecForge v3.0.0 — Multi-Platform Cross-Compiler (Production)"
echo "=================================================================="

mkdir -p dist
export CGO_ENABLED=0

echo "Compiling for Windows (x86_64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o dist/specforge-windows-amd64.exe .

echo "Compiling for macOS Apple Silicon (M1/M2/M3/M4)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -trimpath -o dist/specforge-darwin-arm64 .

echo "Compiling for macOS Intel (x86_64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o dist/specforge-darwin-amd64 .

echo "Compiling for Linux Server / CI (x86_64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o dist/specforge-linux-amd64 .

echo "=================================================================="
echo "✓ All binaries built successfully into ./dist/"
echo "=================================================================="
