#!/usr/bin/env bash
set -euo pipefail

mkdir -p build
go build -buildvcs=false -o build/bdf ./cmd/bdf
go build -buildvcs=false -o build/bdfgen ./cmd/bdfgen
