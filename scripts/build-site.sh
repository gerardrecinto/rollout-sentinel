#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
mkdir -p _site
cp site/index.html _site/
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o _site/sentinel.wasm ./site/demo
if [[ -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ]]; then
  cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" _site/
else
  cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" _site/
fi
touch _site/.nojekyll
