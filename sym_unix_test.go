// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

//go:build !windows

package purego

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

const cTestSrc = `
#include <stdint.h>

uintptr_t my_unique_test_function_name(void) {
    return 999;
}
`

func compileAndLoadLib(t *testing.T) (uintptr, uintptr) {
	t.Helper()
	tmpDir := t.TempDir()
	cFile := filepath.Join(tmpDir, "testsym.c")
	if err := os.WriteFile(cFile, []byte(cTestSrc), 0644); err != nil {
		t.Fatalf("failed to write C source: %v", err)
	}

	var libFile string
	if runtime.GOOS == "darwin" {
		libFile = filepath.Join(tmpDir, "testsym.dylib")
	} else {
		libFile = filepath.Join(tmpDir, "testsym.so")
	}

	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}

	cmd := exec.Command(cc, "-shared", "-fPIC", "-o", libFile, cFile)
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping test: C compiler not available: %v", err)
	}

	handle, err := Dlopen(libFile, RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		t.Fatalf("Dlopen failed: %v", err)
	}

	sym, err := Dlsym(handle, "my_unique_test_function_name")
	if err != nil {
		_ = Dlclose(handle)
		t.Fatalf("Dlsym failed: %v", err)
	}

	return handle, sym
}

func TestGetSymbolName(t *testing.T) {
	handle, symAddr := compileAndLoadLib(t)
	defer func() { _ = Dlclose(handle) }()

	name := getSymbolName(symAddr)
	want := "my_unique_test_function_name"
	if name != want {
		t.Errorf("getSymbolName(%x) = %q, want %q", symAddr, name, want)
	}
}