#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$ROOT_DIR/build"
CLASSES_DIR="$BUILD_DIR/classes"

mkdir -p "$CLASSES_DIR"
find "$CLASSES_DIR" -type f -delete

javac --release 17 -Xlint:all -d "$CLASSES_DIR" \
    $(find "$ROOT_DIR/src/main/java" -name '*.java' -print)
jar --create --file "$BUILD_DIR/bdf-java.jar" -C "$CLASSES_DIR" .

printf 'Created %s\n' "$BUILD_DIR/bdf-java.jar"
