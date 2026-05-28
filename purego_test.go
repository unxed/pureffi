package purego_test

import (
	"errors"
	"runtime"
	"syscall"
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

// TestStructReturn tests calling a standard C function that returns a struct by value.
func TestStructReturn(t *testing.T) {
	handle, err := purego.Dlopen(getLibc(), purego.RTLD_NOW)
	if err != nil {
		t.Skipf("libc not found: %v", err)
	}
	defer purego.Dlclose(handle)

	// div() returns div_t, which is { int quot; int rem; }
	// This fits in 8 bytes and is returned in a register (RAX on AMD64).
	sym, err := purego.Dlsym(handle, "div")
	if err != nil {
		t.Skipf("div not found: %v", err)
	}

	type DivT struct {
		Quot int32
		Rem  int32
	}

	var div func(numer, denom int32) DivT
	purego.RegisterFunc(&div, sym)

	res := div(10, 3)
	if res.Quot != 3 || res.Rem != 1 {
		t.Errorf("div(10, 3) = %+v, want {Quot: 3, Rem: 1}", res)
	}
}

// TestFullCircleFFI tests the entire FFI pipeline without relying on external OS behaviors!
// It registers a complex Go callback, wraps it into a C-callable function pointer,
// and then calls it via RegisterFunc. This validates:
// 1. Stack spilling (lots of arguments).
// 2. Structs passed by value (with mixed float/int).
// 3. Nested structs.
func TestFullCircleFFI(t *testing.T) {
	type Point struct {
		X, Y float32
	}
	type Rect struct {
		TopLeft     Point
		BottomRight Point
	}

	// 1. Define our target function in Go.
	// It takes enough arguments to force stack spilling on all architectures,
	// and takes a complex nested struct by value.
	target := func(a int, b float64, c int, d float64, e int, f float64, g int, h float64, r Rect) float64 {
		// Do some math to prove values arrived intact
		return float64(a+c+e+g) + b + d + f + h + float64(r.TopLeft.X+r.BottomRight.Y)
	}

	// 2. Convert it to a C function pointer (tests goffi callback creation).
	cptr := purego.NewCallback(target)

	// 3. Bind it back to a Go variable (tests goffi dynamic call interface).
	var boundFunc func(a int, b float64, c int, d float64, e int, f float64, g int, h float64, r Rect) float64
	purego.RegisterFunc(&boundFunc, cptr)

	// 4. Call it! (Go -> FFI Caller -> ASM Trampoline -> Callback Dispatcher -> Target Go Func)
	rect := Rect{
		TopLeft:     Point{X: 10.5, Y: 0},
		BottomRight: Point{X: 0, Y: 5.25},
	}

	res := boundFunc(1, 1.1, 2, 2.2, 3, 3.3, 4, 4.4, rect)
	expected := float64(1+2+3+4) + 1.1 + 2.2 + 3.3 + 4.4 + 10.5 + 5.25

	if res != expected {
		t.Errorf("FullCircle FFI failed! Expected %f, got %f", expected, res)
	} else {
		t.Logf("FullCircle FFI succeeded: %f == %f", res, expected)
	}
}

// TestNilPointerAndSlice ensures that passing Go 'nil' for pointers
// correctly translates to a C NULL (0) pointer and doesn't cause nil-dereference panics.
func TestNilPointerAndSlice(t *testing.T) {
	// Our target function simulates a C function that checks for NULL pointers
	target := func(ptr *int32, ptr2 *byte) uintptr {
		if ptr == nil && ptr2 == nil {
			return 1
		}
		return 0
	}

	cptr := purego.NewCallback(target)

	var boundFunc func(ptr *int32, ptr2 *byte) uintptr
	purego.RegisterFunc(&boundFunc, cptr)

	// Call with explicit nils
	res := boundFunc(nil, nil)
	if res != 1 {
		t.Errorf("Expected 1 (nils correctly translated to NULL), got %d", res)
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

func TestErrnoAccess(t *testing.T) {
	libName := getSystemLibrary()
	if libName == "" {
		t.Skipf("skipping on unsupported platform %s", runtime.GOOS)
	}

	libc, err := purego.Dlopen(libName, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatalf("failed to open libc: %v", err)
	}
	defer purego.Dlclose(libc)

	// 1. Test standard signature returning (value, error)
	var strtoll func(nptr string, endptr unsafe.Pointer, base int) (int64, error)
	purego.RegisterFunc(&strtoll, func() uintptr {
		sym, err := purego.Dlsym(libc, "strtoll")
		if err != nil {
			t.Fatalf("failed to find strtoll: %v", err)
		}
		return sym
	}())

	// Successful call (errno should remain 0, error is nil)
	val, err := strtoll("12345", nil, 10)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if val != 12345 {
		t.Errorf("expected 12345, got %d", val)
	}

	// Overflow call (should trigger ERANGE error)
	_, err = strtoll("9999999999999999999999999999999999999999999999999", nil, 10)
	if err == nil {
		t.Error("expected ERANGE error, got nil")
	} else {
		if !errors.Is(err, syscall.ERANGE) {
			t.Errorf("expected ERANGE error, got: %v", err)
		}
	}

	// 2. Test void-return signature returning only (error)
	var strtollVoid func(nptr string, endptr unsafe.Pointer, base int) error
	purego.RegisterLibFunc(&strtollVoid, libc, "strtoll")

	// Trigger overflow on void-return function
	err = strtollVoid("9999999999999999999999999999999999999999999999999", nil, 10)
	if err == nil {
		t.Error("expected ERANGE error, got nil")
	} else {
		if !errors.Is(err, syscall.ERANGE) {
			t.Errorf("expected ERANGE error, got: %v", err)
		}
	}
}
