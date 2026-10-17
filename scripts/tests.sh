#!/usr/bin/env bash
set -euo pipefail

go test ./...
node scripts/build.mjs
node --test --experimental-strip-types "tests/node/*.test.ts"
