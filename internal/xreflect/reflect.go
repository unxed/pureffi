// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

package xreflect

import "reflect"

// TypeAssert is a wrapper for reflect.TypeAssert to bridge Go version gaps.
// Since we enforce Go 1.25.5 minimum, we can use the native version safely.
func TypeAssert[T any](v reflect.Value) (T, bool) {
	return reflect.TypeAssert[T](v)
}