// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

package purego_test

import (
	"testing"

	"github.com/ebitengine/purego"
)

// BenchmarkFastPath проверяет производительность "быстрого пути" pureffi.
// Использует примитивные типы, которые покрыты генератором zz_fast_func.go.
func BenchmarkFastPath(b *testing.B) {
	libc, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		b.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(libc)

	// int, not int32: only pointer-sized arguments take the fast path.
	var abs func(int) int
	purego.RegisterLibFunc(&abs, libc, "abs")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = abs(-1)
	}
}

// BenchmarkFastPathPointer is the fast path with a pointer argument, which the
// wrapper takes as an unsafe.Pointer so the GC keeps its object alive.
func BenchmarkFastPathPointer(b *testing.B) {
	libc, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		b.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(libc)

	var strlen func(*byte) uintptr
	purego.RegisterLibFunc(&strlen, libc, "strlen")
	s := []byte("benchmark\x00")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strlen(&s[0])
	}
}

// BenchmarkSlowPath проверяет производительность "медленного пути" (fallback).
// Передаем строку, чтобы заставить pureffi использовать reflect.MakeFunc с авто-маршалингом string -> char*.
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

// BenchmarkSyscallN проверяет производительность SyscallN (оптимизирован с кэшированием CIF).
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
