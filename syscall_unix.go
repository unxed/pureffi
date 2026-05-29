//go:build !windows

package purego

import (
	"runtime"
	"sync"
	"syscall"
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
func getErrnoPtr() uintptr {
	errnoOnce.Do(initErrno)
	if errnoFn == 0 {
		return 0
	}
	var ptr uintptr
	_ = ffi.CallFunction(&errnoCif, unsafe.Pointer(errnoFn), unsafe.Pointer(&ptr), nil)
	return ptr
}

func clrAndGetErrno() (uintptr, func() error) {
	ptr := getErrnoPtr()
	if ptr != 0 {
		*(*int32)(unsafe.Pointer(ptr)) = 0
	}
	return ptr, func() error {
		if ptr != 0 {
			errno := *(*int32)(unsafe.Pointer(ptr))
			if errno != 0 {
				return syscall.Errno(errno)
			}
		}
		return nil
	}
}

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// On Unix, it gracefully locates and calls the thread-local __errno_location
// to capture the C errno value correctly.
// Global cache of pre-prepared CallInterface structures for SyscallN (0 to 32 arguments).
// Since SyscallN always treats all arguments as uintptr-sized pointers and returns a uintptr,
// we can pre-compute the CallInterface once for each argument count and reuse it.
var (
	syscallCIFs [33]types.CallInterface
	syscallOnce sync.Once
)

type syscallContext struct {
	args    [32]uintptr
	ffiArgs [32]unsafe.Pointer
	result  uintptr
	errVal  uintptr
}

var syscallPool = sync.Pool{
	New: func() any {
		ctx := &syscallContext{}
		// Pre-link ffiArgs to point permanently to ctx.args slots.
		// This completely eliminates pointer allocations during hot FFI calls.
		for i := 0; i < 32; i++ {
			ctx.ffiArgs[i] = unsafe.Pointer(&ctx.args[i])
		}
		return ctx
	},
}

func initSyscallCIFs() {
	for i := 0; i <= 32; i++ {
		argTypes := make([]*types.TypeDescriptor, i)
		for j := 0; j < i; j++ {
			argTypes[j] = types.PointerTypeDescriptor
		}
		_ = ffi.PrepareCallInterface(&syscallCIFs[i], types.DefaultCall, types.PointerTypeDescriptor, argTypes)
	}
}

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// On Unix, it uses pre-prepared CallInterface structures from the global cache
// and a sync.Pool monolithic context to achieve absolute maximum performance,
// dropping hot-path heap allocations down to 1 (the variadic args slice itself).
//
//go:uintptrescapes
func SyscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	if fn == 0 {
		panic("purego: fn is nil")
	}

	errnoOnce.Do(initErrno)
	syscallOnce.Do(initSyscallCIFs)

	nArgs := len(args)
	if nArgs > 32 {
		panic("purego: too many arguments to SyscallN")
	}

	cif := &syscallCIFs[nArgs]
	ctx := syscallPool.Get().(*syscallContext)

	// Copy arguments into the stable, pooled context array
	copy(ctx.args[:], args)
	ctx.result = 0
	ctx.errVal = 0

	// Perform actual C call.
	// ctx.ffiArgs is already pre-populated with addresses pointing into ctx.args.
	_ = ffi.CallFunction(cif, unsafe.Pointer(fn), unsafe.Pointer(&ctx.result), ctx.ffiArgs[:nArgs])

	r1 = ctx.result

	// Retrieve errno only if the call failed (returned -1).
	// We use 32-bit truncation int32(r1) == -1 to correctly handle both 32-bit sign-extended
	// and zero-extended error returns (e.g., 0xffffffff) on 64-bit architectures.
	// This matches standard POSIX C semantics, where errno is only valid on error,
	// and completely eliminates FFI/LockOSThread overhead on successful hot paths!
	if int32(r1) == -1 && errnoFn != 0 {
		runtime.LockOSThread()
		_ = ffi.CallFunction(&errnoCif, unsafe.Pointer(errnoFn), unsafe.Pointer(&ctx.errVal), nil)
		if ctx.errVal != 0 {
			err = uintptr(*(*int32)(unsafe.Pointer(ctx.errVal)))
		}
		runtime.UnlockOSThread()
	}

	syscallPool.Put(ctx)

	// Keep args alive to strictly honor the go:uintptrescapes contract.
	// This ensures that Go objects passed as uintptrs are not garbage collected
	// during the FFI execution.
	runtime.KeepAlive(args)

	return r1, 0, err
}
