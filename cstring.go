// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

package purego

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

var (
	mallocFn   uintptr
	mallocCif  types.CallInterface
	mallocOnce sync.Once
)

func initMalloc() {
	var handle uintptr
	if runtime.GOOS == "windows" {
		h, err := Dlopen("msvcrt.dll", 0)
		if err != nil {
			h, _ = Dlopen("ucrtbase.dll", 0)
		}
		if h == 0 {
			return
		}
		handle = h
	} else {
		if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" || runtime.GOOS == "netbsd" {
			handle = ^uintptr(0) - 1 // RTLD_DEFAULT is -2
		} else {
			handle = 0 // RTLD_DEFAULT is 0 on Linux
		}
	}

	sym, _ := Dlsym(handle, "malloc")
	if sym != 0 {
		mallocFn = sym
		_ = ffi.PrepareCallInterface(&mallocCif, types.DefaultCall, types.PointerTypeDescriptor, []*types.TypeDescriptor{
			types.PointerTypeDescriptor, // size_t
		})
	}
}

// CString converts a Go string to a null-terminated C string.
// The C string is allocated in the C heap using malloc.
// It is the caller's responsibility to arrange for it to be
// freed, such as by calling free.
func CString(s string) *byte {
	mallocOnce.Do(initMalloc)
	if mallocFn == 0 {
		panic("purego: malloc not found")
	}

	size := uintptr(len(s) + 1)
	var ptr *byte
	args := []unsafe.Pointer{unsafe.Pointer(&size)}
	err := ffi.CallFunction(&mallocCif, unsafe.Pointer(mallocFn), unsafe.Pointer(&ptr), args)
	if err != nil {
		panic(err)
	}
	if ptr == nil {
		panic("purego: malloc failed")
	}

	slice := unsafe.Slice(ptr, size)
	copy(slice, s)
	slice[len(s)] = 0
	return ptr
}

// GoString converts a null-terminated C string and copies it into
// a Go allocated string.
func GoString(c *byte) string {
	if c == nil {
		return ""
	}
	var length int
	for {
		if *(*byte)(unsafe.Add(unsafe.Pointer(c), uintptr(length))) == '\x00' {
			break
		}
		length++
	}
	return string(unsafe.Slice(c, length))
}
