package strings

import (
	"testing"
	"unsafe"
)

func TestHasSuffixExact(t *testing.T) {
	if !hasSuffix("alpha", "alpha") {
		t.Fatal("exact suffix was not recognized")
	}
}

func TestHasSuffixShorter(t *testing.T) {
	if hasSuffix("go", "golang") {
		t.Fatal("longer suffix was incorrectly recognized")
	}
}

func TestCStringAddsTerminator(t *testing.T) {
	got := CString("hello")
	if string((*[6]byte)(unsafe.Pointer(got))[:]) != "hello\x00" {
		t.Fatalf("CString returned %q", string((*[6]byte)(unsafe.Pointer(got))[:]))
	}
}

func TestCStringPreservesTerminator(t *testing.T) {
	value := "hello\x00"
	got := CString(value)
	if got != (*byte)(unsafe.Pointer(unsafe.StringData(value))) {
		t.Fatal("CString did not reuse an already terminated string")
	}
}

func TestGoStringNil(t *testing.T) {
	if got := GoString(0); got != "" {
		t.Fatalf("GoString(nil) = %q", got)
	}
}

func TestGoStringCopiesCString(t *testing.T) {
	buf := []byte("world\x00ignored")
	if got := GoString(uintptr(unsafe.Pointer(&buf[0]))); got != "world" {
		t.Fatalf("GoString = %q", got)
	}
}
