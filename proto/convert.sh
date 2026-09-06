#!/usr/bin/env bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo ">> Mencari semua file .proto dan generate .pb.go"

find . -name "*.proto" -print0 | \
  xargs -0 protoc \
    --go_out=paths=source_relative:. \
    --go-grpc_out=paths=source_relative:.

echo "✅ Selesai generate semua .proto ke .pb.go"
