#!/usr/bin/env bash
# Builds plugin.wasm (Go -> WebAssembly reactor) and packs dist/<id>-<version>.zip, signed with
# dev.key (created on the first run). Trust dev.pub under Plugins -> Settings -> Trusted publisher
# keys, then upload the zip under Plugins -> Installed.
set -euo pipefail
cd "$(dirname "$0")"
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
tool() { go run ../../tools/loungeplugin "$@"; }
[ -f dev.key ] || tool keygen -out dev
tool pack -dir . -key dev.key -out dist
