// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

package purego_test

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

func TestCStringGoString(t *testing.T) {
	libPath := getSystemLibrary()
	if libPath == "" {
		t.Skip("system library not found for this OS")
	}

	s := "Hello, CString!"
	cStr := purego.CString(s)
	if cStr == nil {
		t.Fatal("CString returned nil")
	}

	goStr := purego.GoString(cStr)
	if goStr != s {
		t.Fatalf("GoString returned %q, want %q", goStr, s)
	}

	// Try to free it just to make sure we don't leak in the test
	var handle uintptr
	var handleValid bool
	if runtime.GOOS == "windows" {
		h, err := purego.Dlopen("msvcrt.dll", 0)
		if err != nil {
			h, _ = purego.Dlopen("ucrtbase.dll", 0)
		}
		if h != 0 {
			handle = h
			handleValid = true
		}
	} else {
		if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" || runtime.GOOS == "netbsd" {
			handle = ^uintptr(0) - 1 // RTLD_DEFAULT
		} else {
			handle = 0
		}
		handleValid = true
	}
	if handleValid {
		freeSym, err := purego.Dlsym(handle, "free")
		if err == nil && freeSym != 0 {
			var free func(unsafe.Pointer)
			purego.RegisterFunc(&free, freeSym)
			free(unsafe.Pointer(cStr))
		}
	}
}
