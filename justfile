gcflags := "-B"
ldflags := ""

_default:
  @just --list


# Build: run tests with race detector
build: lint test

# Run linters
lint:
  @echo "==> Running lint"
  golangci-lint run

# Run tests with race detector and coverage (fuzz targets run seed corpus only)
test:
  gotestsum -- -gcflags='{{gcflags}}' -race -ldflags='{{ldflags}}' -cover .

# Run all fuzz targets for FUZZ_TIME each and save new corpus to testdata
FUZZ_TIME := env_var_or_default('FUZZ_TIME', '30s')
FUZZ_CACHE := `go env GOCACHE` + "/fuzz/github.com/kadaan/ahocorasick"
FUZZ_TARGETS := "FuzzMatch FuzzNewMatcher FuzzMatchThreadSafeConcurrent"

fuzz:
  #!/usr/bin/env bash
  set -euo pipefail
  TESTDATA="{{justfile_directory()}}/testdata/fuzz"
  for target in {{FUZZ_TARGETS}}; do
    echo "==> Fuzzing ${target} for {{FUZZ_TIME}}"
    go test -gcflags='{{gcflags}}' -ldflags='{{ldflags}}' -fuzz="^${target}$" -fuzztime={{FUZZ_TIME}} .
    mkdir -p "${TESTDATA}/${target}"
    if ls "{{FUZZ_CACHE}}/${target}/"* 2>/dev/null | head -1 > /dev/null; then
      cp -n "{{FUZZ_CACHE}}/${target}/"* "${TESTDATA}/${target}/" 2>/dev/null || true
    fi
  done

# Run benchmarks
bench:
  go test -gcflags='{{gcflags}}' -ldflags='{{ldflags}}' -benchmem -bench .
