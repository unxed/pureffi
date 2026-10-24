//go:build !windows && !arm

package purego

import (
	"strings"
	"testing"
)

func TestSyscallNPreparationErrors(t *testing.T) {
	syscallOnce.Do(initSyscallCIFs)

	tested := false
	for nArgs := 16; nArgs <= 32; nArgs++ {
		if syscallCIFErrors[nArgs] == nil {
			continue
		}
		tested = true

		func() {
			defer func() {
				got := recover()
				if got == nil {
					t.Fatalf("SyscallN with %d arguments did not panic", nArgs)
				}
				message, ok := got.(string)
				if !ok || !strings.Contains(message, "failed to prepare SyscallN call interface") {
					t.Fatalf("SyscallN with %d arguments panicked with %v, want a preparation error", nArgs, got)
				}
			}()

			SyscallN(1, make([]uintptr, nArgs)...)
		}()
	}
	if !tested {
		t.Skip("goffi supports SyscallN with 16 through 32 arguments on this platform")
	}
}
