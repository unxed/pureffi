//go:build arm

package purego

import (
	"errors"
)

type CDecl struct{}
type Variadic struct{}

// RTLD_* are declared so that code written against the real purego API still
// compiles on arm. goffi has no arm ABI backend yet (tracked as "pending" in
// goffi's docs/PLATFORMS.md), so every entry point below fails at run time
// rather than silently doing the wrong thing.
const (
	RTLD_DEFAULT = ^uintptr(0) - 1
	RTLD_LAZY    = 1
	RTLD_NOW     = 2
	RTLD_LOCAL   = 0
	RTLD_GLOBAL  = 0x100
)

func Dlopen(path string, mode int) (uintptr, error) {
	return 0, errors.New("purego: Dlopen not supported on arm")
}

func Dlsym(handle uintptr, name string) (uintptr, error) {
	return 0, errors.New("purego: Dlsym not supported on arm")
}

func Dlclose(handle uintptr) error {
	return errors.New("purego: Dlclose not supported on arm")
}

func RegisterLibFunc(fptr any, handle uintptr, name string) {
	panic("purego: RegisterLibFunc not supported on arm")
}

func RegisterFunc(fptr any, cfn uintptr) {
	panic("purego: RegisterFunc not supported on arm")
}

func NewCallback(fn any) uintptr {
	panic("purego: NewCallback not supported on arm")
}

func SyscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	panic("purego: SyscallN not supported on arm")
}

func CString(s string) *byte {
	panic("purego: CString not supported on arm")
}

func GoString(c *byte) string {
	panic("purego: GoString not supported on arm")
}
