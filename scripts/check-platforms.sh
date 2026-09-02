#!/usr/bin/env bash
#
# check-platforms.sh — the single source of truth for pureffi's platform matrix.
#
# pureffi is a drop-in purego API on top of goffi, so its reach is goffi's reach.
# This script proves that, target by target, and fails when the table drifts:
#
#   full     Dlopen/Dlsym + RegisterFunc/RegisterLibFunc + NewCallback + SyscallN
#   load     Dlopen/Dlsym + NewCallback; RegisterFunc fails at run time because
#            goffi has no ABI backend for the arch (windows/386)
#   stub     compiles, every entry point fails at run time (linux/arm)
#   pending  not supported — the build is EXPECTED to fail
#
# CGO_ENABLED=1-only targets from purego's matrix (iOS, Android amd64/386/arm)
# are out of scope: goffi's contract is a zero-CGO build.
#
# The goffi version comes from go.mod's replace directive. To validate against
# an unreleased goffi checkout or branch before tagging:
#
#   GOFFI_REPLACE=/path/to/goffi scripts/check-platforms.sh
#   GOFFI_REPLACE=github.com/unxed/goffi@main scripts/check-platforms.sh

set -uo pipefail

GOFFI="github.com/go-webgpu/goffi"
FAKECGO_STD="-gcflags=${GOFFI}/internal/fakecgo=-std"

cd "$(dirname "$0")/.."

if [ -n "${GOFFI_REPLACE:-}" ]; then
  echo "==> Overriding ${GOFFI} => ${GOFFI_REPLACE}"
  cp go.mod /tmp/pureffi-go.mod.orig
  cp go.sum /tmp/pureffi-go.sum.orig 2>/dev/null || true
  trap 'cp /tmp/pureffi-go.mod.orig go.mod; cp /tmp/pureffi-go.sum.orig go.sum 2>/dev/null || true' EXIT
  go mod edit -replace "${GOFFI}=${GOFFI_REPLACE}"
  GOFLAGS=-mod=mod go mod tidy >/dev/null 2>&1 || true
fi

# Format: goos/goarch|tier|extra go build flags
#
# FreeBSD and NetBSD need the fakecgo=-std gcflag: goffi's fakecgo publishes
# environ/__progname (and __ps_strings on NetBSD) via //go:cgo_export_dynamic,
# and rtld resolves libc's undefined references against them at startup.
TARGETS=(
  "linux/amd64|full|"
  "linux/arm64|full|"
  "darwin/amd64|full|"
  "darwin/arm64|full|"
  "windows/amd64|full|"
  "windows/arm64|full|"
  "freebsd/amd64|full|${FAKECGO_STD}"
  "freebsd/arm64|full|${FAKECGO_STD}"
  # NetBSD is carried by the pinned goffi release, so these are full like the
  # other BSD rows. They were "pending" until go.mod caught up; the STALE
  # check below is what noticed and failed the build.
  "netbsd/amd64|full|${FAKECGO_STD}"
  "netbsd/arm64|full|${FAKECGO_STD}"
  "android/arm64|full|"

  "windows/386|load|"
  "linux/arm|stub|"

  # goffi has no ABI backend for these yet. See goffi ROADMAP.md,
  # "Architecture Expansion".
  "linux/386|pending|"
  "linux/loong64|pending|"
  "linux/ppc64le|pending|"
  "linux/riscv64|pending|"
  "linux/s390x|pending|"
)

FAILED=0
declare -a SUMMARY

for entry in "${TARGETS[@]}"; do
  IFS='|' read -r target tier flags <<< "${entry}"
  goos="${target%%/*}"
  goarch="${target##*/}"

  # shellcheck disable=SC2086 # flags is intentionally word-split (may be empty)
  out=$(GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build ${flags} ./... 2>&1)
  rc=$?

  if [ "${tier}" = "pending" ]; then
    if [ ${rc} -eq 0 ]; then
      echo "  ❌ ${target} builds but is listed as pending"
      echo "     goffi grew a backend for this arch. Promote the row here and in README.md."
      FAILED=1
      SUMMARY+=("STALE   ${target}")
    else
      echo "  ⏳ ${target} pending (expected)"
      SUMMARY+=("pending ${target}")
    fi
    continue
  fi

  if [ ${rc} -eq 0 ]; then
    echo "  ✅ ${target} (${tier})"
    SUMMARY+=("ok      ${target} [${tier}]")
  else
    echo "  ❌ ${target} FAILED to build"
    echo "${out}" | head -20
    FAILED=1
    SUMMARY+=("FAIL    ${target}")
  fi
done

echo ""
if [ ${FAILED} -eq 1 ]; then
  echo "❌ Platform matrix check FAILED"
  printf '%s\n' "${SUMMARY[@]}"
  exit 1
fi

echo "✅ Platform matrix check passed"
printf '   %s\n' "${SUMMARY[@]}"
