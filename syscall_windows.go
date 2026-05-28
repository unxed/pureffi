//go:build windows

package purego

import "syscall"

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// On Windows, it directly wraps Go's native syscall.SyscallN to accurately
// capture GetLastError() as the third return value.
func SyscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	r1, r2, errno := syscall.SyscallN(fn, args...)
	return r1, r2, uintptr(errno)
}

var (
	lazyKernel32     = syscall.NewLazyDLL("kernel32.dll")
	procSetLastError = lazyKernel32.NewProc("SetLastError")
)

func clrAndGetErrno() (uintptr, func() error) {
	_, _, _ = procSetLastError.Call(0)
	return 0, func() error {
		errno := syscall.GetLastError()
		if errno != nil && errno != syscall.Errno(0) {
			return errno
		}
		return nil
	}
}
