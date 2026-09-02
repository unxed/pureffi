// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

//go:build linux && (amd64 || arm64)

package purego_test

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goffiHasProfileU reports whether the goffi module in the build list carries
// the universal ("Profile U") build. The empty-SONAME dl imports live in
// internal/dl/dl_universal.go, which only exists there, so listing that
// package under the tag is a cheap and honest probe: on an older goffi the
// -tags goffi_universal build silently falls back to the glibc flavour and
// would legitimately carry a DT_NEEDED.
func goffiHasProfileU(t *testing.T) bool {
	t.Helper()
	out, err := exec.Command("go", "list", "-tags", "goffi_universal",
		"-f", "{{.GoFiles}}", "github.com/go-webgpu/goffi/internal/dl").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "dl_universal.go")
}

// TestUniversalNoDTNeeded builds the probe as a CGO-free -tags goffi_universal
// binary and asserts it names no libc.
//
// This is the contract that lets pureffi be used in a goffi Profile U build:
// pureffi must contribute no //go:cgo_import_dynamic directive of its own, or
// the linker emits a DT_NEEDED entry, the binary is pinned to one libc, and
// the "one artifact for glibc and musl" guarantee is gone. Stripping PT_INTERP
// is a separate post-link step, so it is checked by goffi-audit rather than
// here.
func TestUniversalNoDTNeeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: builds a binary")
	}
	if !goffiHasProfileU(t) {
		t.Skip("skipping: the goffi in use has no universal (Profile U) build")
	}

	out := filepath.Join(t.TempDir(), "universal-probe")
	cmd := exec.Command("go", "build", "-tags", "goffi_universal",
		"-o", out, "github.com/ebitengine/purego/cmd/universal-probe")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build universal probe: %v\n%s", err, b)
	}

	f, err := elf.Open(out)
	if err != nil {
		t.Fatalf("open ELF: %v", err)
	}
	defer func() { _ = f.Close() }()

	libs, err := f.ImportedLibraries()
	if err != nil {
		t.Fatalf("read DT_NEEDED: %v", err)
	}
	if len(libs) > 0 {
		t.Errorf("universal binary has DT_NEEDED %v, want none", libs)
	}
}
