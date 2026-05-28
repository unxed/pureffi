// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2026 unxed

//go:build !((amd64 || arm64) && (darwin || freebsd || linux || netbsd) && !windows)

package purego

import (
	"reflect"

	"github.com/go-webgpu/goffi/types"
)

func tryRegisterFastPath(fn reflect.Value, cif *types.CallInterface, cfn uintptr, ty reflect.Type) bool {
	return false
}
