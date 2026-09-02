// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

// universal-probe exercises the pureffi API from a single binary so that a
// goffi "Profile U" build (one CGO-free artifact for both glibc and musl) can
// be checked at run time on either libc.
//
// Build it the universal way and run the very same file on Debian and Alpine:
//
//	CGO_ENABLED=0 go build -tags goffi_universal -o /tmp/uprobe \
//	    github.com/ebitengine/purego/cmd/universal-probe
//	go run github.com/go-webgpu/goffi/cmd/goffi-strip-interp /tmp/uprobe
//	go run github.com/go-webgpu/goffi/cmd/goffi-audit /tmp/uprobe
//
// It prints PUREFFI-UNIVERSAL-PROBE-OK and exits 0 on success.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ebitengine/purego"
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "universal-probe: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if runtime.GOOS != "linux" {
		fmt.Println("universal-probe: universal builds are Linux-only; nothing to do on", runtime.GOOS)
		return
	}

	// musl aliases the reserved libc-family SONAMEs (libc.so.6, libm.so.6,
	// libpthread.so.0, ...) to its own loader, so the glibc name resolves on
	// both libcs. goffi's ffi.HostLibC() reports the real one when needed.
	handle, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		fail("Dlopen libc: %v", err)
	}

	var strlen func(string) int
	purego.RegisterLibFunc(&strlen, handle, "strlen")
	if got := strlen("hello"); got != 5 {
		fail("strlen(hello) = %d, want 5", got)
	}

	// atof lives in libc on both glibc and musl (sqrt does not: glibc keeps it
	// in libm), so it is the portable floating-point check.
	var atof func(string) float64
	purego.RegisterLibFunc(&atof, handle, "atof")
	if got := atof("3.5"); got != 3.5 {
		fail("atof(3.5) = %v, want 3.5", got)
	}

	getpid, err := purego.Dlsym(handle, "getpid")
	if err != nil {
		fail("Dlsym getpid: %v", err)
	}
	if pid, _, _ := purego.SyscallN(getpid); pid == 0 {
		fail("getpid returned 0")
	}

	// errno capture goes through __errno_location, which both libcs export.
	closeFn, err := purego.Dlsym(handle, "close")
	if err != nil {
		fail("Dlsym close: %v", err)
	}
	if _, _, errno := purego.SyscallN(closeFn, ^uintptr(0)); errno == 0 {
		fail("close(-1) did not set errno")
	}

	// A Go callback called back through the C ABI.
	cmp := purego.NewCallback(func(a, b *int32) int32 { return *a - *b })
	var qsort func(base *int32, n, size uintptr, cmp uintptr)
	purego.RegisterLibFunc(&qsort, handle, "qsort")
	arr := []int32{5, 3, 9, 1, 4}
	qsort(&arr[0], uintptr(len(arr)), 4, cmp)
	for i := 1; i < len(arr); i++ {
		if arr[i-1] > arr[i] {
			fail("qsort produced %v", arr)
		}
	}

	if err := purego.Dlclose(handle); err != nil {
		fail("Dlclose: %v", err)
	}

	fmt.Println("PUREFFI-UNIVERSAL-PROBE-OK")
}
