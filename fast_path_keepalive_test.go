// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

// The fast path this tests exists only on these targets (zz_fast_func.go).
//go:build (amd64 || arm64) && (darwin || freebsd || linux || netbsd)

package purego_test

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"weak"

	"github.com/ebitengine/purego"
)

type sleepSpec struct {
	sec  int64
	nsec int64
	_    [64]byte // keep it out of the tiny allocator
}

// A pointer argument must keep its object alive for the whole C call, even
// when the caller holds no other reference. The fast wrapper used to take it
// as a uintptr, so a GC during the call could collect the object while C was
// still using it.
func TestFastPathKeepsPointerArgumentsAlive(t *testing.T) {
	libPath := getSystemLibrary()
	if libPath == "" {
		t.Skip("system library not found for this OS")
	}
	h, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatalf("Dlopen failed: %v", err)
	}
	var nanosleep func(req, rem *sleepSpec) int32
	purego.RegisterLibFunc(&nanosleep, h, "nanosleep")

	type callState struct {
		rem    weak.Pointer[sleepSpec]
		active atomic.Bool
	}
	var current atomic.Pointer[callState]
	var collectedInCall atomic.Bool
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			s := current.Load()
			runtime.GC()
			// active is cleared only after the call returns, so seeing the
			// object gone and the call still running means it was collected
			// mid-call.
			if s != nil && s.rem.Value() == nil && s.active.Load() {
				collectedInCall.Store(true)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	for i := 0; i < 5; i++ {
		rem := &sleepSpec{}
		s := &callState{rem: weak.Make(rem)}
		s.active.Store(true)
		current.Store(s)
		nanosleep(&sleepSpec{nsec: 50 * int64(time.Millisecond)}, rem)
		s.active.Store(false)
	}
	close(stop)
	<-done
	if collectedInCall.Load() {
		t.Fatal("a pointer argument was garbage-collected while the C call was still running")
	}
}
