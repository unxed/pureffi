// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

package purego_test

import (
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

func TestRegisterFunc_FastPathAllocations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses SyscallN which behaves differently")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("fast path only supported on amd64 and arm64")
	}

	libPath := getSystemLibrary()
	if libPath == "" {
		t.Skip("system library not found for this OS")
	}

	handle, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatalf("Dlopen failed: %v", err)
	}
	defer purego.Dlclose(handle)

	// We use "abs" (takes 1 int, returns 1 int) from libc as a simple test
	var abs func(int) int
	purego.RegisterLibFunc(&abs, handle, "abs")

	// Verify it actually works first
	if got := abs(-42); got != 42 {
		t.Fatalf("abs(-42) = %d; want 42", got)
	}

	// Benchmark/check allocations
	allocs := testing.AllocsPerRun(100, func() {
		_ = abs(-42)
	})

	// Expected allocations: 1
	// pureffi's fast path is zero-allocation (it uses sync.Pool for the context),
	// but the underlying goffi engine currently allocates exactly 1 time per FFI call.
	// This happens because goffi/internal/syscall/syscall_unix_*.go calls
	// runtime_cgocall(..., unsafe.Pointer(&args)). Since runtime_cgocall lacks
	// the //go:noescape directive, the local 'args' struct escapes to the heap.
	// We expect this to drop to 0 once goffi is patched.
	if allocs > 1 {
		t.Errorf("RegisterFunc fast path allocated %v times; want <= 1", allocs)
	}
}
