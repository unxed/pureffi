// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

// The fast path this tests exists only on these targets (zz_fast_func.go).
//go:build (amd64 || arm64) && (darwin || freebsd || linux || netbsd)

package purego_test

import (
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

type narrowBuf struct{ a, b, c [12]uint64 }

var narrowAbs func(a int32, b int32, c uint64, p *narrowBuf, n int32) int32

//go:noinline
func narrowCaller(depth int, keep *narrowBuf) int32 {
	var pad [64]byte
	if depth > 0 {
		r := narrowCaller(depth-1, keep)
		pad[depth&63] = byte(r)
		return r + int32(pad[depth&63]&0)
	}
	buf := &narrowBuf{}
	r := narrowAbs(-5, 7, 0, buf, 96)
	runtime.KeepAlive(keep)
	runtime.KeepAlive(buf)
	return r
}

// Regression test: a function registered with narrow (int32) arguments used to
// get the uintptr fast wrapper, whose register spill overran the caller's
// spill area whenever the wrapper grew the stack, leaving the integer 0x60 in
// a pointer slot of narrowCaller's frame. The GC then died with "invalid
// pointer found on stack". Small goroutine stacks at varying depths make the
// wrapper's prologue hit morestack; the background GC finds the bad slot.
func TestStressNarrowArgs(t *testing.T) {
	libPath := getSystemLibrary()
	if libPath == "" {
		t.Skip("no libc")
	}
	h, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	purego.RegisterLibFunc(&narrowAbs, h, "abs")
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
			}
		}
	}()
	defer close(stop)
	for i := 0; i < 20000; i++ {
		done := make(chan int32)
		d := i % 40
		go func() { done <- narrowCaller(d, &narrowBuf{}) }()
		if got := <-done; got != 5 {
			t.Fatalf("abs = %d", got)
		}
	}
}
