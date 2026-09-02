//go:build (freebsd || openbsd || netbsd || dragonfly || solaris || illumos) && !arm

// This file must NOT be named dlfcn_freebsd.go. A *_GOOS.go suffix adds an
// implicit build constraint that is ANDed with the //go:build line, so the
// netbsd/openbsd/dragonfly/solaris/illumos alternatives above would never fire
// and Dlopen/Dlsym/Dlclose would be undefined on every BSD except FreeBSD.

package purego

import (
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

const (
	RTLD_DEFAULT = ^uintptr(0) - 1
	RTLD_LAZY    = 1
	RTLD_NOW     = 2
	RTLD_LOCAL   = 0
	RTLD_GLOBAL  = 0x100
)

// Dlopen examines the dynamic library or bundle file specified by path.
func Dlopen(path string, mode int) (uintptr, error) {
	ptr, err := ffi.LoadLibrary(path)
	if err != nil {
		return 0, Dlerror{err.Error()}
	}
	return uintptr(ptr), nil
}

// Dlsym takes a "handle" of a dynamic library returned by Dlopen and the symbol name.
func Dlsym(handle uintptr, name string) (uintptr, error) {
	ptr, err := ffi.GetSymbol(unsafe.Pointer(handle), name)
	if err != nil {
		return 0, Dlerror{err.Error()}
	}
	return uintptr(ptr), nil
}

// Dlclose decrements the reference count on the dynamic library handle.
func Dlclose(handle uintptr) error {
	err := ffi.FreeLibrary(unsafe.Pointer(handle))
	if err != nil {
		return Dlerror{err.Error()}
	}
	return nil
}
