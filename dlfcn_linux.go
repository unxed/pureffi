//go:build (linux || android) && !arm

package purego

import (
	"runtime"
	"sync"
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

var (
	dlopenOnce sync.Once
	dlopenFn   func(string, int32) uintptr
	dlerrorFn  func() string
	dlopenErr  error
)

func initDlopen() {
	dlopenAddr, err := Dlsym(RTLD_DEFAULT, "dlopen")
	if err != nil {
		dlopenErr = err
		return
	}
	dlerrorAddr, err := Dlsym(RTLD_DEFAULT, "dlerror")
	if err != nil {
		dlopenErr = err
		return
	}
	RegisterFunc(&dlopenFn, dlopenAddr)
	RegisterFunc(&dlerrorFn, dlerrorAddr)
}

// Dlopen examines the dynamic library or bundle file specified by path.
func Dlopen(path string, mode int) (uintptr, error) {
	dlopenOnce.Do(initDlopen)
	if dlopenErr != nil {
		return 0, dlopenErr
	}

	// dlerror is thread-local, so clear and read it on the same OS thread as dlopen.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dlerrorFn()
	handle := dlopenFn(path, int32(mode))
	if handle == 0 {
		if message := dlerrorFn(); message != "" {
			return 0, Dlerror{message}
		}
		return 0, Dlerror{"dlopen failed: " + path}
	}
	return handle, nil
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
