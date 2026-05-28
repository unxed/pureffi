// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

package purego_test

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

const cIntegrationSource = `
#include <stdint.h>
#include <string.h>
#include <stdlib.h>
#include <stdarg.h>

typedef struct {
    int32_t x;
    int32_t y;
} MyPoint;

typedef struct {
    double x;
    double y;
    double w;
    double h;
} MyRect;

int32_t c_add_ints(int32_t a, int32_t b) {
    return a + b;
}

double c_add_doubles(double a, double b) {
    return a + b;
}

char* c_reverse_string(const char* s) {
    if (!s) return NULL;
    size_t len = strlen(s);
    char* rev = (char*)malloc(len + 1);
    if (!rev) return NULL;
    for (size_t i = 0; i < len; i++) {
        rev[i] = s[len - 1 - i];
    }
    rev[len] = '\0';
    return rev;
}

typedef uintptr_t (*my_callback_fn)(uintptr_t, uintptr_t);
uintptr_t c_call_callback(my_callback_fn cb, uintptr_t a, uintptr_t b) {
    if (!cb) return 0;
    return cb(a, b);
}

int64_t c_sum_variadic(int64_t count, ...) {
    va_list ap;
    va_start(ap, count);
    int64_t sum = 0;
    for (int64_t i = 0; i < count; i++) {
        sum += va_arg(ap, int64_t);
    }
    va_end(ap);
    return sum;
}

int32_t c_sum_point(MyPoint p) {
    return p.x + p.y;
}

MyPoint c_make_point(int32_t x, int32_t y) {
    MyPoint p;
    p.x = x;
    p.y = y;
    return p;
}

double c_sum_rect(MyRect r) {
    return r.x + r.y + r.w + r.h;
}
`

func compileAndGetLib(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	cFile := filepath.Join(tmpDir, "integration.c")
	if err := os.WriteFile(cFile, []byte(cIntegrationSource), 0644); err != nil {
		t.Fatalf("failed to write C source: %v", err)
	}

	var libFile string
	if runtime.GOOS == "windows" {
		libFile = filepath.Join(tmpDir, "integration.dll")
	} else if runtime.GOOS == "darwin" {
		libFile = filepath.Join(tmpDir, "integration.dylib")
	} else {
		libFile = filepath.Join(tmpDir, "integration.so")
	}

	cc := os.Getenv("CC")
	if cc == "" {
		if runtime.GOOS == "windows" {
			cc = "gcc"
		} else {
			cc = "cc"
		}
	}

	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"-shared", "-o", libFile, cFile}
	} else {
		args = []string{"-shared", "-fPIC", "-o", libFile, cFile}
	}

	cmd := exec.Command(cc, args...)
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping integration test: C compiler not available: %v", err)
	}

	return libFile
}

func TestPureffiIntegration(t *testing.T) {
	// 1. Test Dlerror and Error Types
	_, errNonexistent := purego.Dlopen("nonexistent_library_xyz.so", purego.RTLD_NOW)
	if errNonexistent == nil {
		t.Error("expected Dlopen of nonexistent library to fail")
	} else if runtime.GOOS != "windows" {
		if _, ok := errNonexistent.(purego.Dlerror); !ok {
			t.Errorf("expected error of type purego.Dlerror, got %T", errNonexistent)
		}
	}

	libPath := compileAndGetLib(t)
	handle, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatalf("Dlopen failed: %v", err)
	}
	defer purego.Dlclose(handle)

	// 2. Test RegisterFunc with Integers
	symAddInts, err := purego.Dlsym(handle, "c_add_ints")
	if err != nil {
		t.Fatalf("Dlsym c_add_ints failed: %v", err)
	}
	var addInts func(int32, int32) int32
	purego.RegisterFunc(&addInts, symAddInts)

	if got := addInts(15, 27); got != 42 {
		t.Errorf("addInts(15, 27) = %d, want 42", got)
	}

	// 3. Test RegisterLibFunc (shortcut helper)
	var addIntsLib func(int32, int32) int32
	purego.RegisterLibFunc(&addIntsLib, handle, "c_add_ints")

	if got := addIntsLib(20, 22); got != 42 {
		t.Errorf("RegisterLibFunc: addIntsLib(20, 22) = %d, want 42", got)
	}

	// 4. Test RegisterFunc with Floats
	if runtime.GOARCH == "386" || runtime.GOARCH == "arm" {
		t.Log("skipping float/double tests on 32-bit platforms")
	} else {
		symAddDoubles, err := purego.Dlsym(handle, "c_add_doubles")
		if err != nil {
			t.Fatalf("Dlsym c_add_doubles failed: %v", err)
		}
		var addDoubles func(float64, float64) float64
		purego.RegisterFunc(&addDoubles, symAddDoubles)

		if got := addDoubles(3.14, 2.86); math.Abs(got-6.0) > 1e-9 {
			t.Errorf("addDoubles(3.14, 2.86) = %f, want 6.0", got)
		}
	}

	// 5. Test Strings (CString / GoString / Marshalling)
	symRevStr, err := purego.Dlsym(handle, "c_reverse_string")
	if err != nil {
		t.Fatalf("Dlsym c_reverse_string failed: %v", err)
	}
	var reverseString func(string) string
	purego.RegisterFunc(&reverseString, symRevStr)

	if got := reverseString("pureffi"); got != "ifferup" {
		t.Errorf("reverseString(%q) = %q, want %q", "pureffi", got, "ifferup")
	}

	// Explicit CString & GoString manual test
	testStr := "Hello, pureffi!"
	cStr := purego.CString(testStr)
	if cStr == nil {
		t.Error("CString returned nil")
	} else {
		if got := purego.GoString(cStr); got != testStr {
			t.Errorf("GoString(CString(%q)) = %q, want %q", testStr, got, testStr)
		}
	}

	// 6. Test NewCallback (Go called from C)
	symCallCallback, err := purego.Dlsym(handle, "c_call_callback")
	if err != nil {
		t.Fatalf("Dlsym c_call_callback failed: %v", err)
	}
	var callCallback func(uintptr, uintptr, uintptr) uintptr
	purego.RegisterFunc(&callCallback, symCallCallback)

	// Register Go function as C-compatible callback
	goCallback := func(a, b uintptr) uintptr {
		return a * b
	}
	cbPtr := purego.NewCallback(goCallback)

	if got := callCallback(cbPtr, 6, 7); got != 42 {
		t.Errorf("callCallback(cb, 6, 7) = %d, want 42", got)
	}

	// 7. Test SyscallN (direct raw invocation)
	r1, _, _ := purego.SyscallN(symAddInts, 10, 20)
	if int32(r1) != 30 {
		t.Errorf("SyscallN(c_add_ints, 10, 20) returned %d, want 30", r1)
	}

	// 8. Test C-variadic calls (using ...any Go-signature)
	symSumVariadic, err := purego.Dlsym(handle, "c_sum_variadic")
	if err != nil {
		t.Fatalf("Dlsym c_sum_variadic failed: %v", err)
	}
	var sumVariadic func(count int, args ...any) int
	purego.RegisterFunc(&sumVariadic, symSumVariadic)

	if got := sumVariadic(3, 10, 20, 30); got != 60 {
		t.Errorf("sumVariadic(3, 10, 20, 30) = %d, want 60", got)
	}

	// 9. Test Struct Argument Passing (<=8B struct by-value)
	type MyPoint struct {
		X int32
		Y int32
	}
	symSumPoint, err := purego.Dlsym(handle, "c_sum_point")
	if err != nil {
		t.Fatalf("Dlsym c_sum_point failed: %v", err)
	}
	var sumPoint func(MyPoint) int32
	purego.RegisterFunc(&sumPoint, symSumPoint)

	if got := sumPoint(MyPoint{X: 18, Y: 24}); got != 42 {
		t.Errorf("sumPoint(18, 24) = %d, want 42", got)
	}

	// 10. Test Struct Return (<=8B struct return by-value)
	symMakePoint, err := purego.Dlsym(handle, "c_make_point")
	if err != nil {
		t.Fatalf("Dlsym c_make_point failed: %v", err)
	}
	var makePoint func(int32, int32) MyPoint
	purego.RegisterFunc(&makePoint, symMakePoint)

	gotPoint := makePoint(100, 200)
	if gotPoint.X != 100 || gotPoint.Y != 200 {
		t.Errorf("makePoint(100, 200) = %+v, want {X:100, Y:200}", gotPoint)
	}

	// 11. Test Large Struct Argument (32B rect, HFA/stack/reference depending on platform)
	if runtime.GOARCH == "386" || runtime.GOARCH == "arm" {
		t.Log("skipping large struct tests on 32-bit platforms")
	} else {
		type MyRect struct {
			X float64
			Y float64
			W float64
			H float64
		}
		symSumRect, err := purego.Dlsym(handle, "c_sum_rect")
		if err != nil {
			t.Fatalf("Dlsym c_sum_rect failed: %v", err)
		}
		var sumRect func(MyRect) float64
		purego.RegisterFunc(&sumRect, symSumRect)

		rect := MyRect{X: 1.5, Y: 2.5, W: 3.5, H: 4.5} // sum is 12.0
		if got := sumRect(rect); math.Abs(got-12.0) > 1e-9 {
			t.Errorf("sumRect(1.5, 2.5, 3.5, 4.5) = %f, want 12.0", got)
		}
	}
}