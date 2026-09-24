package xreflect

import (
	"reflect"
	"testing"
)

func TestTypeAssertSuccess(t *testing.T) {
	got, ok := TypeAssert[string](reflect.ValueOf("value"))
	if !ok || got != "value" {
		t.Fatalf("TypeAssert success = %q, %v", got, ok)
	}
}

func TestTypeAssertFailure(t *testing.T) {
	got, ok := TypeAssert[int](reflect.ValueOf("value"))
	if ok || got != 0 {
		t.Fatalf("TypeAssert failure = %d, %v", got, ok)
	}
}
