//go:build !arm

package purego

import (
	"reflect"
	"testing"

	"github.com/go-webgpu/goffi/types"
)

func TestDlerrorErrorString(t *testing.T) {
	if got := (Dlerror{s: "failure"}).Error(); got != "failure" {
		t.Fatalf("Dlerror.Error() = %q", got)
	}
}

func TestGoTypeToFfiTypePrimitives(t *testing.T) {
	cases := []struct {
		name string
		value any
		want *types.TypeDescriptor
	}{
		{name: "int8", value: int8(0), want: types.SInt8TypeDescriptor},
		{name: "uint8", value: uint8(0), want: types.UInt8TypeDescriptor},
		{name: "float64", value: float64(0), want: types.DoubleTypeDescriptor},
		{name: "pointer", value: (*byte)(nil), want: types.PointerTypeDescriptor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := goTypeToFfiType(reflect.TypeOf(tc.value)); got != tc.want {
				t.Fatalf("goTypeToFfiType(%s) returned %p, want %p", tc.name, got, tc.want)
			}
		})
	}
}