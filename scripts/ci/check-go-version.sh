#!/usr/bin/env bash
set -euo pipefail

expected_major_minor="${EXPECTED_GO_MAJOR_MINOR:-1.26}"

fail() {
  echo "[go-version] FAIL: $*" >&2
  exit 1
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "$repo_root" ]]; then
  fail "not in a git repository"
fi
cd "$repo_root"

if [[ ! -f "go.mod" ]]; then
  fail "missing go.mod"
fi

go_line="$(grep -E '^go [0-9]+\.[0-9]+(\.[0-9]+)?$' go.mod | head -n 1 || true)"
if [[ -z "$go_line" ]]; then
  fail "cannot find a valid 'go <version>' line in go.mod"
fi
go_version="${go_line#go }"

case "$go_version" in
  "$expected_major_minor"|"$expected_major_minor".[0-9]*) ;;
  *)
    fail "go.mod uses go ${go_version}; expected ${expected_major_minor}.x (prevents go mod init fallback)"
    ;;
esac

tool_versions_note="(skip: .tool-versions not found)"
if [[ -f ".tool-versions" ]]; then
  tool_line="$(grep -E '^golang [0-9]+\.[0-9]+(\.[0-9]+)?$' .tool-versions | head -n 1 || true)"
  if [[ -z "$tool_line" ]]; then
    fail ".tool-versions exists but no valid 'golang <version>' entry"
  fi
  tool_version="${tool_line#golang }"
  case "$tool_version" in
    "$expected_major_minor"|"$expected_major_minor".[0-9]*) ;;
    *)
      fail ".tool-versions uses golang ${tool_version}; expected ${expected_major_minor}.x"
      ;;
  esac
  tool_versions_note="(.tool-versions=${tool_version})"
fi

echo "[go-version] OK: go.mod=${go_version} ${tool_versions_note}"
