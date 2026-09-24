//go:build !arm

package purego

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-webgpu/goffi/types"
)

func TestPackArgString(t *testing.T) {
	got, kept := packArg(reflect.ValueOf("hello"))
	if buffer, ok := kept.([]byte); !ok || string(buffer) != "hello\x00" {
		t.Fatalf("kept value = %#v", kept)
	}
	ptr := *(*unsafe.Pointer)(got)
	if got := cStringToGoString(uintptr(ptr)); got != "hello" {
		t.Fatalf("packed string = %q", got)
	}
}

func TestPackArgTerminatedString(t *testing.T) {
	value := "hello\x00"
	got, kept := packArg(reflect.ValueOf(value))
	if kept != value {
		t.Fatalf("kept value = %#v", kept)
	}
	ptr := *(*unsafe.Pointer)(got)
	if got := cStringToGoString(uintptr(ptr)); got != "hello" {
		t.Fatalf("packed terminated string = %q", got)
	}
}

func TestPackArgBool(t *testing.T) {
	got, kept := packArg(reflect.ValueOf(true))
	if kept == nil || *(*uint8)(got) != 1 {
		t.Fatalf("packed true = %d, kept %#v", *(*uint8)(got), kept)
	}
}

func TestPackArgNumber(t *testing.T) {
	got, kept := packArg(reflect.ValueOf(int32(42)))
	if kept == nil || *(*int32)(got) != 42 {
		t.Fatalf("packed number = %d, kept %#v", *(*int32)(got), kept)
	}
}

func TestPackArgPointer(t *testing.T) {
	value := 42
	got, kept := packArg(reflect.ValueOf(&value))
	if kept == nil || *(*unsafe.Pointer)(got) != unsafe.Pointer(&value) {
		t.Fatalf("packed pointer = %p, kept %#v", *(*unsafe.Pointer)(got), kept)
	}
}

func TestPackArgArray(t *testing.T) {
	got, kept := packArg(reflect.ValueOf([2]byte{1, 2}))
	ptr := *(*unsafe.Pointer)(got)
	if _, ok := kept.([2]byte); !ok || *(*byte)(ptr) != 1 {
		t.Fatalf("packed array starts with %d, kept %#v", *(*byte)(ptr), kept)
	}
}

func TestPackArgStruct(t *testing.T) {
	type pair struct{ A, B int32 }
	got, kept := packArg(reflect.ValueOf(pair{A: 1, B: 2}))
	if _, ok := kept.(pair); !ok || *(*int32)(got) != 1 {
		t.Fatalf("packed struct starts with %d, kept %#v", *(*int32)(got), kept)
	}
}

func TestGoTypeToFfiTypeArray(t *testing.T) {
	got := goTypeToFfiType(reflect.TypeOf([3]uint16{}))
	if got.Kind != types.StructType || len(got.Members) != 3 || got.Size == 0 {
		t.Fatalf("array descriptor = %#v", got)
	}
}

func TestGoTypeToFfiTypeStruct(t *testing.T) {
	type pair struct {
		A int32
		B float64
	}
	got := goTypeToFfiType(reflect.TypeOf(pair{}))
	if got.Kind != types.StructType || len(got.Members) != 2 || got.Size == 0 {
		t.Fatalf("struct descriptor = %#v", got)
	}
}

func TestUnpackRetValues(t *testing.T) {
	t.Run("bool", func(t *testing.T) {
		ret := reflect.New(reflect.TypeOf(false))
		value := uint8(1)
		unpackRet(ret.Elem().Type(), unsafe.Pointer(&value), ret)
		if !ret.Elem().Bool() {
			t.Fatal("true return unpacked as false")
		}
	})
	t.Run("string", func(t *testing.T) {
		ret := reflect.New(reflect.TypeOf(""))
		data := []byte("ok\x00")
		addr := uintptr(unsafe.Pointer(&data[0]))
		unpackRet(ret.Elem().Type(), unsafe.Pointer(&addr), ret)
		if got := ret.Elem().String(); got != "ok" {
			t.Fatalf("string return = %q", got)
		}
	})
}
