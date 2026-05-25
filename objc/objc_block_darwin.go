// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

package objc

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/internal/xreflect"
)

const (
	blockBaseClass = "__NSMallocBlock__"
	blockFlags = blockHasCopyDispose | blockHasSignature
	blockHasCopyDispose = 1 << 25
	blockHasSignature = 1 << 30
)

type blockDescriptor struct {
	_         uintptr
	size      uintptr
	_         uintptr
	dispose   uintptr
	signature *uint8
}

type blockLayout struct {
	isa        Class
	flags      uint32
	_          uint32
	invoke     uintptr
	descriptor *blockDescriptor
}

type blockFunctionCache struct {
	mutex     sync.RWMutex
	functions map[Block]reflect.Value
}

func (b *blockFunctionCache) Load(key Block) reflect.Value {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.functions[key]
}

func (b *blockFunctionCache) Store(key Block, value reflect.Value) Block {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.functions[key] = value
	return key
}

func (b *blockFunctionCache) Delete(key Block) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	delete(b.functions, key)
}

func newBlockFunctionCache() *blockFunctionCache {
	return &blockFunctionCache{functions: map[Block]reflect.Value{}}
}

type blockCache struct {
	sync.Mutex
	descriptorTemplate blockDescriptor
	layoutTemplate     blockLayout
	layouts            map[reflect.Type]blockLayout
	Functions          *blockFunctionCache
}

func (*blockCache) encode(typ reflect.Type) *uint8 {
	if typ == nil || typ.Kind() != reflect.Func {
		panic("objc: not a function")
	}

	var encoding string
	switch typ.NumOut() {
	case 0:
		encoding = encVoid
	default:
		returnType, err := encodeType(typ.Out(0), false)
		if err != nil {
			panic(fmt.Sprintf("objc: %v", err))
		}
		encoding = returnType
	}

	if typ.NumIn() == 0 || typ.In(0) != reflect.TypeOf(Block(0)) {
		panic(fmt.Sprintf("objc: A Block implementation must take a Block as its first argument; got %v", typ.String()))
	}

	encoding += encId
	for i := 1; i < typ.NumIn(); i++ {
		argType, err := encodeType(typ.In(i), false)
		if err != nil {
			panic(fmt.Sprintf("objc: %v", err))
		}
		encoding = fmt.Sprint(encoding, argType)
	}

	return &append([]uint8(encoding), 0)[0]
}

func (b *blockCache) getLayout(typ reflect.Type) blockLayout {
	b.Lock()
	defer b.Unlock()

	if layout, ok := b.layouts[typ]; ok {
		return layout
	}

	layout := b.layoutTemplate
	layout.descriptor = &blockDescriptor{}
	*layout.descriptor = b.descriptorTemplate

	layout.descriptor.signature = b.encode(typ)

	layout.invoke = purego.NewCallback(
		reflect.MakeFunc(
			typ,
			func(args []reflect.Value) (results []reflect.Value) {
				block, ok := xreflect.TypeAssert[Block](args[0])
				if !ok {
					panic(fmt.Sprintf("objc: block argument is not a block but %s", args[0].Type().String()))
				}
				return b.Functions.Load(block).Call(args)
			},
		).Interface(),
	)

	b.layouts[typ] = layout
	return layout
}

func newBlockCache() *blockCache {
	cache := &blockCache{
		descriptorTemplate: blockDescriptor{
			size: unsafe.Sizeof(blockLayout{}),
		},
		layoutTemplate: blockLayout{
			isa:   GetClass(blockBaseClass),
			flags: blockFlags,
		},
		layouts:   map[reflect.Type]blockLayout{},
		Functions: newBlockFunctionCache(),
	}
	cache.descriptorTemplate.dispose = purego.NewCallback(cache.Functions.Delete)
	return cache
}

var theBlocksCache *blockCache

type Block ID

func (b Block) Copy() Block {
	return _Block_copy(b)
}

func (b Block) Invoke(args ...any) {
	fn := theBlocksCache.Functions.Load(b)

	reflectedArgs := make([]reflect.Value, len(args)+1)
	reflectedArgs[0] = reflect.ValueOf(b)
	for i := range args {
		reflectedArgs[i+1] = reflect.ValueOf(args[i])
	}

	fn.Call(reflectedArgs)
}

func (b Block) Release() {
	_Block_release(b)
}

func NewBlock(fn any) Block {
	layout := theBlocksCache.getLayout(reflect.TypeOf(fn))
	block := Block(unsafe.Pointer(&layout)).Copy()
	return theBlocksCache.Functions.Store(block, reflect.ValueOf(fn))
}

func InvokeBlock[T any](block Block, args ...any) (result T, err error) {
	block = block.Copy()
	defer block.Release()

	fn := theBlocksCache.Functions.Load(block)
	if fn.Type().NumIn() != len(args)+1 {
		return result, fmt.Errorf("objc: block callback expects %d arguments, got %d", fn.Type().NumIn()-1, len(args))
	}

	reflectedArgs := make([]reflect.Value, len(args)+1)
	reflectedArgs[0] = reflect.ValueOf(block)
	for i := range args {
		reflectedArgs[i+1] = reflect.ValueOf(args[i])
	}

	callResult := fn.Call(reflectedArgs)

	var ok bool
	result, ok = xreflect.TypeAssert[T](callResult[0])
	if !ok {
		return result, fmt.Errorf("objc: the returned value type %s was not %T", callResult[0].Type().String(), result)
	}

	return result, nil
}