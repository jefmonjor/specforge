#!/usr/bin/env bash
set -e

echo "Compiling SpecForge v3.0.0 for Linux/macOS..."
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o specforge .
echo "Build complete: specforge"
