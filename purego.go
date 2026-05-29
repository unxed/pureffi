package purego

import (
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// CDecl marks a function as being called using the __cdecl calling convention.
type CDecl struct{}

// Variadic marks a function as a true C-variadic function (like printf).
// This instructs the runtime to correctly flush variadic arguments to the stack
// on platforms that require it (such as Apple Silicon ARM64 AAPCS64).
// It must be the first argument to the function.
type Variadic struct{}

// RegisterLibFunc is a wrapper around RegisterFunc that uses the C function returned from Dlsym(handle, name).
func RegisterLibFunc(fptr any, handle uintptr, name string) {
	sym, err := Dlsym(handle, name)
	if err != nil {
		panic(err)
	}
	RegisterFunc(fptr, sym)
}

// RegisterFunc takes a pointer to a Go function representing the calling convention of the C function.
func RegisterFunc(fptr any, cfn uintptr) {
	if cfn == 0 {
		panic("purego: cfn is nil")
	}
	fn := reflect.ValueOf(fptr).Elem()
	ty := fn.Type()
	if ty.Kind() != reflect.Func {
		panic("purego: fptr must be a function pointer")
	}

	numOut := ty.NumOut()
	if numOut > 2 {
		panic("purego: function can only return up to two values")
	}

	var errorType = reflect.TypeOf((*error)(nil)).Elem()
	hasError := false
	if numOut == 2 {
		if ty.Out(1) != errorType {
			panic("purego: second return value must be an error")
		}
		hasError = true
	} else if numOut == 1 {
		if ty.Out(0) == errorType {
			hasError = true
		}
	}

	convention := types.DefaultCall
	numIn := ty.NumIn()
	isCVariadic := false
	startIn := 0

	if numIn > 0 {
		if ty.In(0) == reflect.TypeOf(CDecl{}) {
			convention = types.UnixCallingConvention // Equivalent of explicit CDecl on most platforms
			startIn = 1
		} else if ty.In(0) == reflect.TypeOf(Variadic{}) {
			isCVariadic = true
			startIn = 1
		}
	}

	isGoVariadic := ty.IsVariadic()
	fixedIn := numIn
	if isGoVariadic {
		fixedIn--
	}

	// MAGIC HEURISTIC: Transparently detect true C-variadic functions vs objc_msgSend
	if isGoVariadic && !isCVariadic {
		symName := getSymbolName(cfn)
		if strings.HasPrefix(symName, "objc_msgSend") {
			// Objective-C message dispatchers are NOT true C-variadic functions under the ABI.
			isCVariadic = false
		} else if symName == "" && runtime.GOOS == "darwin" {
			// Safety fallback: if dladdr fails on macOS, we preserve purego legacy behavior
			// to protect objc_msgSend from crashing.
			isCVariadic = false
		} else {
			// On other platforms, or for any other function (printf, sprintf), it's a true C-variadic.
			isCVariadic = true
		}
	}

	var fixedArgTypes []*types.TypeDescriptor
	for i := startIn; i < fixedIn; i++ {
		fixedArgTypes = append(fixedArgTypes, goArgTypeToFfiType(ty.In(i)))
	}

	var retType *types.TypeDescriptor = types.VoidTypeDescriptor
	if numOut == 2 {
		retType = goTypeToFfiType(ty.Out(0))
	} else if numOut == 1 && !hasError {
		retType = goTypeToFfiType(ty.Out(0))
	}

	var fixedCif types.CallInterface
	if !isGoVariadic || !isCVariadic {
		if !isGoVariadic {
			err := ffi.PrepareCallInterface(&fixedCif, convention, retType, fixedArgTypes)
			if err != nil {
				panic(err)
			}
		}
	}

	if !isGoVariadic && !hasError && tryRegisterFastPath(fn, &fixedCif, cfn, ty) {
		return
	}

	v := reflect.MakeFunc(ty, func(args []reflect.Value) []reflect.Value {
		var keepAliveBuf [16]any
		keepAlive := keepAliveBuf[:0]
		defer func() {
			runtime.KeepAlive(keepAlive)
			runtime.KeepAlive(args)
		}()
		if hasError {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}

		var getErr func() error
		if hasError {
			_, getErr = clrAndGetErrno()
		}

		var ffiArgsBuf [16]unsafe.Pointer
		ffiArgs := ffiArgsBuf[:0]

		// Pack fixed args
		for i := startIn; i < fixedIn; i++ {
			ptr, kept := packArg(args[i])
			ffiArgs = append(ffiArgs, ptr)
			if kept != nil {
				keepAlive = append(keepAlive, kept)
			}
		}

		var actualArgTypes []*types.TypeDescriptor
		if isGoVariadic {
			actualArgTypes = make([]*types.TypeDescriptor, 0, len(fixedArgTypes)+args[numIn-1].Len())
			actualArgTypes = append(actualArgTypes, fixedArgTypes...)

			// Pack variadic args ("purego" style: unpacked from slice)
			varSlice := args[numIn-1]
			for i := 0; i < varSlice.Len(); i++ {
				elem := varSlice.Index(i)
				if elem.Kind() == reflect.Interface {
					elem = elem.Elem()
				}
				actualArgTypes = append(actualArgTypes, goArgTypeToFfiType(elem.Type()))
				ptr, kept := packArg(elem)
				ffiArgs = append(ffiArgs, ptr)
				if kept != nil {
					keepAlive = append(keepAlive, kept)
				}
			}
		}

		var cif *types.CallInterface
		if isGoVariadic {
			var dynamicCif types.CallInterface
			if isCVariadic {
				// True C-variadic function (like printf).
				// Under the hood, goffi handles the complex Apple ARM64 stack flushes automatically here!
				err := ffi.PrepareVariadicCallInterface(&dynamicCif, convention, len(fixedArgTypes), retType, actualArgTypes)
				if err != nil {
					panic(err)
				}
			} else {
				// Purego backwards compatibility: dynamic but NOT C-variadic (like objc_msgSend)
				err := ffi.PrepareCallInterface(&dynamicCif, convention, retType, actualArgTypes)
				if err != nil {
					panic(err)
				}
			}
			cif = &dynamicCif
		} else {
			cif = &fixedCif
		}

		var rvalue unsafe.Pointer
		var retVal reflect.Value
		var retBuf [32]byte // Large enough for struct returns

		hasActualReturn := (numOut == 2) || (numOut == 1 && !hasError)
		if hasActualReturn {
			outType := ty.Out(0)
			retVal = reflect.New(outType)
			switch outType.Kind() {
			case reflect.String, reflect.Bool, reflect.Pointer, reflect.UnsafePointer, reflect.Func, reflect.Slice:
				rvalue = unsafe.Pointer(&retBuf[0])
			default:
				// Primitive types or structures get populated directly into the new allocation
				rvalue = unsafe.Pointer(retVal.Pointer())
			}
		}

		err := ffi.CallFunction(cif, unsafe.Pointer(cfn), rvalue, ffiArgs)
		if err != nil {
			panic(err)
		}

		var errVal error
		if hasError {
			errVal = getErr()
		}

		if numOut == 2 {
			unpackRet(ty.Out(0), rvalue, retVal)
			var errReflectVal reflect.Value
			if errVal != nil {
				errReflectVal = reflect.ValueOf(errVal).Convert(errorType)
			} else {
				errReflectVal = reflect.Zero(errorType)
			}
			return []reflect.Value{retVal.Elem(), errReflectVal}
		} else if numOut == 1 {
			if hasError {
				var errReflectVal reflect.Value
				if errVal != nil {
					errReflectVal = reflect.ValueOf(errVal).Convert(errorType)
				} else {
					errReflectVal = reflect.Zero(errorType)
				}
				return []reflect.Value{errReflectVal}
			} else {
				unpackRet(ty.Out(0), rvalue, retVal)
				return []reflect.Value{retVal.Elem()}
			}
		}
		return nil
	})
	fn.Set(v)
}

func packArg(v reflect.Value) (unsafe.Pointer, any) {
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if len(s) > 0 && s[len(s)-1] == '\x00' {
			addr := uintptr(unsafe.Pointer(unsafe.StringData(s)))
			return unsafe.Pointer(&addr), s
		}
		b := append([]byte(s), 0)
		addr := uintptr(unsafe.Pointer(&b[0]))
		return unsafe.Pointer(&addr), b
	case reflect.Bool:
		var b uint8
		if v.Bool() {
			b = 1
		}
		ptr := new(uint8)
		*ptr = b
		return unsafe.Pointer(ptr), ptr
	case reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64, reflect.Int, reflect.Uint, reflect.Uintptr, reflect.Float32, reflect.Float64:
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		return unsafe.Pointer(ptr.Pointer()), ptr.Interface()
	case reflect.Pointer, reflect.UnsafePointer, reflect.Slice:
		addr := v.Pointer()
		ptr := new(uintptr)
		*ptr = addr
		return unsafe.Pointer(ptr), ptr
	case reflect.Func:
		addr := ffi.NewCallback(v.Interface())
		ptr := new(uintptr)
		*ptr = addr
		return unsafe.Pointer(ptr), ptr
	case reflect.Array:
		if !v.CanAddr() {
			ptr := reflect.New(v.Type())
			ptr.Elem().Set(v)
			v = ptr.Elem()
		}
		addr := v.UnsafeAddr()
		ptr := new(uintptr)
		*ptr = addr
		return unsafe.Pointer(ptr), v.Interface()
	case reflect.Struct:
		if !v.CanAddr() {
			ptr := reflect.New(v.Type())
			ptr.Elem().Set(v)
			v = ptr.Elem()
		}
		return unsafe.Pointer(v.UnsafeAddr()), v.Interface()
	default:
		panic("purego: unsupported kind " + v.Kind().String())
	}
}

func unpackRet(t reflect.Type, rvalue unsafe.Pointer, retVal reflect.Value) {
	switch t.Kind() {
	case reflect.String:
		addr := *(*uintptr)(rvalue)
		if addr == 0 {
			retVal.Elem().SetString("")
		} else {
			retVal.Elem().SetString(cStringToGoString(addr))
		}
	case reflect.Bool:
		b := *(*uint8)(rvalue)
		retVal.Elem().SetBool(b != 0)
	case reflect.Pointer, reflect.UnsafePointer, reflect.Slice, reflect.Func:
		addr := *(*uintptr)(rvalue)
		*(*uintptr)(unsafe.Pointer(retVal.Pointer())) = addr
	}
}

func cStringToGoString(addr uintptr) string {
	if addr == 0 {
		return ""
	}
	ptr := unsafe.Pointer(addr)
	var length int
	for {
		if *(*byte)(unsafe.Add(ptr, uintptr(length))) == '\x00' {
			break
		}
		length++
	}
	return string(unsafe.Slice((*byte)(ptr), length))
}

func goArgTypeToFfiType(t reflect.Type) *types.TypeDescriptor {
	if t.Kind() == reflect.Array {
		return types.PointerTypeDescriptor
	}
	return goTypeToFfiType(t)
}

func goTypeToFfiType(t reflect.Type) *types.TypeDescriptor {
	switch t.Kind() {
	case reflect.Int8:
		return types.SInt8TypeDescriptor
	case reflect.Uint8, reflect.Bool:
		return types.UInt8TypeDescriptor
	case reflect.Int16:
		return types.SInt16TypeDescriptor
	case reflect.Uint16:
		return types.UInt16TypeDescriptor
	case reflect.Int32:
		return types.SInt32TypeDescriptor
	case reflect.Uint32:
		return types.UInt32TypeDescriptor
	case reflect.Int64, reflect.Int:
		return types.SInt64TypeDescriptor
	case reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return types.UInt64TypeDescriptor
	case reflect.Float32:
		return types.FloatTypeDescriptor
	case reflect.Float64:
		return types.DoubleTypeDescriptor
	case reflect.Pointer, reflect.UnsafePointer, reflect.Func, reflect.Slice, reflect.String:
		return types.PointerTypeDescriptor
	case reflect.Array:
		desc := &types.TypeDescriptor{
			Kind:      types.StructType,
			Size:      t.Size(),
			Alignment: uintptr(t.Align()),
		}
		elemDesc := goTypeToFfiType(t.Elem())
		for i := 0; i < t.Len(); i++ {
			desc.Members = append(desc.Members, elemDesc)
		}
		return desc
	case reflect.Struct:
		desc := &types.TypeDescriptor{
			Kind:      types.StructType,
			Size:      t.Size(),
			Alignment: uintptr(t.Align()),
		}
		for i := 0; i < t.NumField(); i++ {
			desc.Members = append(desc.Members, goTypeToFfiType(t.Field(i).Type))
		}
		return desc
	default:
		panic("purego: unsupported kind " + t.Kind().String())
	}
}

// NewCallback converts a Go function to a function pointer conforming to the C calling convention.
func NewCallback(fn any) uintptr {
	ty := reflect.TypeOf(fn)
	if ty.Kind() != reflect.Func {
		panic("purego: fn must be a function")
	}

	hasMarker := false
	if ty.NumIn() > 0 && (ty.In(0) == reflect.TypeOf(CDecl{}) || ty.In(0) == reflect.TypeOf(Variadic{})) {
		hasMarker = true
	}

	if !hasMarker {
		return ffi.NewCallback(fn)
	}

	// If the callback has a marker (CDecl or Variadic), we must wrap it so that
	// goffi doesn't see the marker and doesn't try to map it from C registers.
	var inTypes []reflect.Type
	for i := 1; i < ty.NumIn(); i++ {
		inTypes = append(inTypes, ty.In(i))
	}

	var outTypes []reflect.Type
	for i := 0; i < ty.NumOut(); i++ {
		outTypes = append(outTypes, ty.Out(i))
	}

	newTy := reflect.FuncOf(inTypes, outTypes, ty.IsVariadic())
	origFn := reflect.ValueOf(fn)

	wrapper := reflect.MakeFunc(newTy, func(args []reflect.Value) []reflect.Value {
		realArgs := make([]reflect.Value, len(args)+1)
		realArgs[0] = reflect.Zero(ty.In(0)) // Pass the empty marker struct
		copy(realArgs[1:], args)
		return origFn.Call(realArgs)
	})

	return ffi.NewCallback(wrapper.Interface())
}
