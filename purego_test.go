package purego_test

import (
	"runtime"
	"testing"
    "unsafe"

	"github.com/ebitengine/purego"
)

func getLibc() string {
	switch runtime.GOOS {
	case "darwin":
		return "libSystem.B.dylib"
	case "windows":
		return "msvcrt.dll"
	default:
		return "libc.so.6"
	}
}

func TestStrlen(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	var strlen func(string) int
	purego.RegisterLibFunc(&strlen, handle, "strlen")

	if l := strlen("hello"); l != 5 {
		t.Errorf("strlen(hello) = %d, want 5", l)
	}
}

func TestVariadic(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	var sprintf func(buf []byte, format string, args ...any) int
	purego.RegisterLibFunc(&sprintf, handle, "sprintf")

	buf := make([]byte, 100)
	// We use the traditional purego style (args mapped linearly).
	n := sprintf(buf, "hello %s %d\x00", "world", 42)
	if n <= 0 {
		t.Errorf("sprintf failed")
	}
	s := string(buf[:n])
	if s != "hello world 42" {
		t.Errorf("sprintf result = %q, want 'hello world 42'", s)
	}
}

func TestAutoVariadicDetection(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	// Look mom, no purego.Variadic marker!
	var sprintf func(buf []byte, format string, args ...any) int
	purego.RegisterLibFunc(&sprintf, handle, "sprintf")

	buf := make([]byte, 100)
	// The runtime uses dladdr to detect this is 'sprintf', NOT 'objc_msgSend',
	// and automatically triggers the correct Apple Silicon stack packing!
	n := sprintf(buf, "hello %s %d\x00", "world", 42)
	if n <= 0 {
		t.Errorf("sprintf failed")
	}
	s := string(buf[:n])
	if s != "hello world 42" {
		t.Errorf("sprintf result = %q, want 'hello world 42'", s)
	}
}

func TestSyscallN(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	sym, err := purego.Dlsym(handle, "abs")
	if err != nil {
		t.Fatalf("abs not found: %v", err)
	}

	r1, _, _ := purego.SyscallN(sym, ^uintptr(41)) // -42 in two's complement for uintptr
	if int32(r1) != 42 {
		t.Errorf("SyscallN abs(-42) = %d, want 42", int32(r1))
	}
}
func TestSyscallNErrno(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	// We use 'mkdir' to intentionally trigger an error
	sym, err := purego.Dlsym(handle, "mkdir")
	if err != nil {
		t.Skipf("mkdir not found: %v", err)
	}

	// Null byte path is invalid, will trigger errno (usually ENOENT or EFAULT)
	invalidPath := "\x00"
	ptr := uintptr(unsafe.Pointer(unsafe.StringData(invalidPath)))

	r1, _, errNo := purego.SyscallN(sym, ptr, 0777)

	if int32(r1) != -1 {
		t.Errorf("mkdir returned %d, want -1", int32(r1))
	}

	if errNo == 0 {
		t.Errorf("expected errno to be non-zero after failing syscall")
	} else {
		t.Logf("captured errno successfully: %d", errNo)
	}
}

func TestSliceAsPointer(t *testing.T) {
	// In purego, passing a slice should pass a pointer to its first element
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	var qsort func(base []int32, num uintptr, size uintptr, compar uintptr)
	purego.RegisterLibFunc(&qsort, handle, "qsort")

	// Create a callback for qsort
	compar := func(a, b *int32) int32 {
		return *a - *b
	}
	cb := purego.NewCallback(compar)

	arr := []int32{5, 2, 9, 1, 5, 6}
	qsort(arr, uintptr(len(arr)), 4, cb)

	// Check if array is sorted
	expected := []int32{1, 2, 5, 5, 6, 9}
	for i := range arr {
		if arr[i] != expected[i] {
			t.Fatalf("qsort failed, got %v, want %v", arr, expected)
		}
	}
}

func TestCallbackWithCDecl(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	var qsort func(base []int32, num uintptr, size uintptr, compar uintptr)
	purego.RegisterLibFunc(&qsort, handle, "qsort")

	// We create a callback for qsort, but this time WITH purego.CDecl
	compar := func(_ purego.CDecl, a, b *int32) int32 {
		return *a - *b
	}
	cb := purego.NewCallback(compar)

	arr := []int32{9, 8, 7, 6}
	qsort(arr, uintptr(len(arr)), 4, cb)

	if arr[0] != 6 || arr[3] != 9 {
		t.Fatalf("qsort with CDecl callback failed, got %v", arr)
	}
}

func TestFloatAndBool(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	// We'll use floor() to test float64 passing and returning
	symFloor, err := purego.Dlsym(handle, "floor")
	if err == nil {
		var floor func(float64) float64
		purego.RegisterFunc(&floor, symFloor)
		if res := floor(3.99); res != 3.0 {
			t.Errorf("floor(3.99) = %f, want 3.0", res)
		}
	}
}
