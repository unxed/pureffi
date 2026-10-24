//go:build linux && amd64

package purego_test

import (
	"testing"

	"github.com/ebitengine/purego"
)

func TestDlopenHonorsRTLDLocal(t *testing.T) {
	if _, err := purego.Dlsym(purego.RTLD_DEFAULT, "cbrt"); err == nil {
		t.Skip("cbrt is already globally visible in this process")
	}

	h, err := purego.Dlopen("libm.so.6", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		t.Skipf("libm.so.6 is unavailable: %v", err)
	}
	defer purego.Dlclose(h)

	if _, err := purego.Dlsym(h, "cbrt"); err != nil {
		t.Fatalf("cbrt is not available through the local handle: %v", err)
	}
	if _, err := purego.Dlsym(purego.RTLD_DEFAULT, "cbrt"); err == nil {
		t.Fatal("cbrt from an RTLD_LOCAL library became visible through RTLD_DEFAULT")
	}
}
