//go:build !windows

package purego

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

var (
	errnoFn   uintptr
	errnoCif  types.CallInterface
	errnoOnce sync.Once
)

func initErrno() {
	var rtldDefault uintptr
	if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" || runtime.GOOS == "netbsd" {
		rtldDefault = ^uintptr(0) - 1 // RTLD_DEFAULT is -2 on these platforms
	} else {
		rtldDefault = 0 // RTLD_DEFAULT is 0 on Linux
	}

	sym, err := Dlsym(rtldDefault, "__errno_location") // Linux
	if err != nil {
		sym, err = Dlsym(rtldDefault, "__error") // macOS / FreeBSD
	}

	if err == nil && sym != 0 {
		errnoFn = sym
		ffi.PrepareCallInterface(&errnoCif, types.DefaultCall, types.PointerTypeDescriptor, nil)
	}
}

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// On Unix, it gracefully locates and calls the thread-local __errno_location
// to capture the C errno value correctly.
func SyscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	if fn == 0 {
		panic("purego: fn is nil")
	}

	errnoOnce.Do(initErrno)

	cif := &types.CallInterface{}
	argTypes := make([]*types.TypeDescriptor, len(args))
	ffiArgs := make([]unsafe.Pointer, len(args))

	for i := range args {
		argTypes[i] = types.PointerTypeDescriptor
		ffiArgs[i] = unsafe.Pointer(&args[i])
	}

	if e := ffi.PrepareCallInterface(cif, types.DefaultCall, types.PointerTypeDescriptor, argTypes); e != nil {
		panic(e)
	}

	var result uintptr
	var errnoPtr uintptr

	if errnoFn != 0 {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Retrieve pointer to thread-local errno
		ffi.CallFunction(&errnoCif, unsafe.Pointer(errnoFn), unsafe.Pointer(&errnoPtr), nil)
		if errnoPtr != 0 {
			*(*int32)(unsafe.Pointer(errnoPtr)) = 0 // Clear errno before the FFI call
		}
	}

	// Perform actual C call
	_ = ffi.CallFunction(cif, unsafe.Pointer(fn), unsafe.Pointer(&result), ffiArgs)

	if errnoPtr != 0 {
		return result, 0, uintptr(*(*int32)(unsafe.Pointer(errnoPtr)))
	}

	return result, 0, 0
}