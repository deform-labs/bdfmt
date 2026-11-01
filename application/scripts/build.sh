#!/usr/bin/env bash
set -euo pipefail

mkdir -p build
go build -buildvcs=false -o build/bdf ./cmd/bdf
go build -buildvcs=false -o build/bdfgen ./cmd/bdfgen

if [[ "$(uname -s)" == "Linux" ]] && command -v javac >/dev/null 2>&1; then
    javac_path="$(readlink -f "$(command -v javac)")"
    java_home="$(dirname "$(dirname "$javac_path")")"
    mkdir -p ../java-lib/build
    CGO_CFLAGS="-I${java_home}/include -I${java_home}/include/linux" \
        go build -buildvcs=false -tags jni -buildmode=c-shared \
        -o ../java-lib/build/libbdfjni.so ./java-lib/native
fi
