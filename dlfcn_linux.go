//go:build linux || android

package purego

import (
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

const (
	RTLD_DEFAULT = 0
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