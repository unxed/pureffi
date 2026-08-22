//go:build !windows && !arm

package purego

import (
	"runtime"
	"structs"
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

type dl_info struct {
	_         structs.HostLayout
	dli_fname uintptr
	dli_fbase uintptr
	dli_sname uintptr
	dli_saddr uintptr
}

var (
	dladdrFn   uintptr
	dladdrCif  types.CallInterface
	dladdrOnce sync.Once
)

// getSymbolName uses POSIX dladdr to reverse-lookup the name of a C function by its address.
func getSymbolName(addr uintptr) string {
	dladdrOnce.Do(func() {
		var rtldDefault uintptr
		if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" || runtime.GOOS == "netbsd" {
			rtldDefault = ^uintptr(0) - 1 // RTLD_DEFAULT is -2
		} else {
			rtldDefault = 0 // RTLD_DEFAULT is 0
		}

		sym, err := Dlsym(rtldDefault, "dladdr")
		if err == nil && sym != 0 {
			dladdrFn = sym
			ffi.PrepareCallInterface(&dladdrCif, types.DefaultCall, types.SInt32TypeDescriptor, []*types.TypeDescriptor{
				types.PointerTypeDescriptor,
				types.PointerTypeDescriptor,
			})
		}
	})

	if dladdrFn == 0 {
		return ""
	}

	var info dl_info
	var res int32

	// goffi expects a pointer to the value that will be passed to C.
	// Since C expects a pointer to the info struct, we must pass a pointer TO the pointer containing the struct's address.
	infoPtr := unsafe.Pointer(&info)

	args := []unsafe.Pointer{
		unsafe.Pointer(&addr),
		unsafe.Pointer(&infoPtr),
	}

	_, _ = ffi.CallFunction(&dladdrCif, unsafe.Pointer(dladdrFn), unsafe.Pointer(&res), args)

	if res != 0 && info.dli_sname != 0 {
		return cStringToGoString(info.dli_sname)
	}
	return ""
}
