// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

//go:build (amd64 || arm64) && (darwin || freebsd || linux || netbsd) && !windows

package purego

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/types"
)

// The fast wrappers are uintptr-typed closures installed under the caller's
// function type, so only signatures whose arguments are all pointer-sized may
// use them: anything narrower lays out the register spill area differently.
func TestTryRegisterFastPath_OnlyPointerSizedArgs(t *testing.T) {
	cases := []struct {
		name string
		fn   any
		want bool
	}{
		{"uintptr", new(func(uintptr, uintptr) uintptr), true},
		{"int int64 uint uint64", new(func(int, int64, uint, uint64) int), true},
		{"pointers", new(func(*byte, unsafe.Pointer) int32), true},
		{"trailing float64", new(func(uintptr, float64)), true},
		{"trailing float32", new(func(uintptr, float32)), true},
		{"int32", new(func(int32, uintptr)), false},
		{"uint32", new(func(uintptr, uint32)), false},
		{"int16", new(func(int16)), false},
		{"uint8", new(func(uint8)), false},
		{"bool", new(func(bool, uintptr)), false},
		{"proc_pidinfo", new(func(int32, int32, uint64, *[96]byte, int32) int32), false},
		{"int32 before float", new(func(int32, float64)), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn := reflect.ValueOf(c.fn).Elem()
			var cif types.CallInterface
			if got := tryRegisterFastPath(fn, &cif, 0, fn.Type()); got != c.want {
				t.Fatalf("tryRegisterFastPath(%v) = %v, want %v", fn.Type(), got, c.want)
			}
		})
	}
}
