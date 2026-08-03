//go:build windows && !arm

package purego

import (
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
)

const (
	RTLD_DEFAULT = 0
	RTLD_LAZY    = 1
	RTLD_NOW     = 2
	RTLD_LOCAL   = 4
	RTLD_GLOBAL  = 8
)

func Dlopen(path string, mode int) (uintptr, error) {
	ptr, err := ffi.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(ptr), nil
}

func Dlsym(handle uintptr, name string) (uintptr, error) {
	ptr, err := ffi.GetSymbol(unsafe.Pointer(handle), name)
	if err != nil {
		return 0, err
	}
	return uintptr(ptr), nil
}

func Dlclose(handle uintptr) error {
	return ffi.FreeLibrary(unsafe.Pointer(handle))
}
