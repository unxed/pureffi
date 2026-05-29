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

	var abs func(int32) int32
	purego.RegisterLibFunc(&abs, libc, "abs")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = abs(-1)
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
