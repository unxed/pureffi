// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

package purego_original_bench_test

import (
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

func getLibc() string {
	switch runtime.GOOS {
	case "darwin":
		return "/usr/lib/libSystem.B.dylib"
	case "windows":
		return "msvcrt.dll"
	default:
		return "libc.so.6"
	}
}

// BenchmarkFastPath проверяет производительность "быстрого пути" оригинального purego.
func BenchmarkFastPath(b *testing.B) {
	libc, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		b.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(libc)

	var abs func(int32) int32
	purego.RegisterLibFunc(&abs, libc, "abs")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = abs(-1)
	}
}

// BenchmarkSlowPath проверяет производительность "медленного пути" (fallback) оригинального purego.
func BenchmarkSlowPath(b *testing.B) {
	libc, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		b.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(libc)

	var strlen func(string) uintptr
	purego.RegisterLibFunc(&strlen, libc, "strlen")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strlen("benchmark\x00")
	}
}

// BenchmarkSyscallN проверяет производительность вызова SyscallN оригинального purego.
func BenchmarkSyscallN(b *testing.B) {
	libc, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		b.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(libc)

	abs, err := purego.Dlsym(libc, "abs")
	if err != nil {
		b.Skipf("abs symbol not found: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = purego.SyscallN(abs, 1)
	}
}