#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if ! command -v protoc >/dev/null 2>&1; then
  echo "❌ Error: 'protoc' tidak ditemukan di PATH." >&2
  exit 1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo "❌ Error: 'protoc-gen-go' tidak ditemukan di PATH. Jalankan: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest" >&2
  exit 1
fi

echo ">> Mencari semua file .proto dan generate .pb.go..."

GRPC_FLAG=()
if command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  GRPC_FLAG=(--go-grpc_out=paths=source_relative:.)
fi

find . -type f -name "*.proto" -print0 | \
  xargs -0 protoc \
    --go_out=paths=source_relative:. \
    "${GRPC_FLAG[@]}"

if command -v goimports >/dev/null 2>&1; then
  echo ">> Merapikan format .pb.go dengan goimports..."
  goimports -local github.com/PakaiWA/whatsmeow -w .
else
  echo "⚠️ Warning: 'goimports' tidak ditemukan, lewati formatting lokal."
fi

echo "✅ Selesai generate semua .proto ke .pb.go"
