package purego_test

import (
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

func getSystemLibrary() string {
	switch runtime.GOOS {
	case "darwin":
		return "/usr/lib/libSystem.B.dylib"
	case "linux":
		return "libc.so.6"
	case "freebsd":
		return "libc.so.7"
	case "windows":
		return "msvcrt.dll"
	default:
		return ""
	}
}

func TestRegisterFunc_ArrayArgument(t *testing.T) {
	libPath := getSystemLibrary()
	if libPath == "" {
		t.Skip("system library not found for this OS")
	}

	handle, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatalf("Dlopen failed: %v", err)
	}
	defer purego.Dlclose(handle)

	var strlen func([6]byte) uintptr
	purego.RegisterLibFunc(&strlen, handle, "strlen")

	tests := []struct {
		name string
		arg  [6]byte
		want uintptr
	}{
		{name: "hello", arg: [6]byte{'h', 'e', 'l', 'l', 'o', 0}, want: 5},
		{name: "empty", arg: [6]byte{0, 0, 0, 0, 0, 0}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strlen(tt.arg)
			if got != tt.want {
				t.Errorf("strlen() = %d, want %d", got, tt.want)
			}
		})
	}
}