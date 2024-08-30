package type_walk

import (
	"fmt"
	g_reflect "github.com/goccy/go-reflect"
	"reflect"
	"unsafe"
)

var (
	// TODO: Find a better way of constructing the error type.
	// Seemingly you can't just get the type of a variable.
	errType = reflect.TypeOf(func() error { return nil }).Out(0)
)

// CompileFn defines the function type that will be called to generate a WalkFn when a value is encountered while
// walking, if a WalkFn has not already been registered.
type CompileFn[Ctx any, In any] func(reflect.Type) WalkFn[Ctx, In]

// Register stores a set of WalkFns used to walk specific types, and functions to compile WalkFns for kinds of types.
type Register[Ctx any] struct {
	typeFns    map[reflect.Type]walkFn[Ctx]
	compileFns [NUM_KIND]unsafe.Pointer
}

// NewRegister creates a new register.
func NewRegister[Ctx any]() *Register[Ctx] {
	return &Register[Ctx]{
		typeFns: make(map[reflect.Type]walkFn[Ctx]),
	}
}

// RegisterTypeFn registers a function to walk a value of a particular type.
// To register a function for type T, fn must be a WalkFn[Ctx,T].
func (w *Register[Ctx]) RegisterTypeFn(fn any) error {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("%T is not a function", fnType)
	}

	var zeroCtx Ctx
	isWalkFn := fnType.NumOut() == 1 &&
		fnType.Out(0) == errType &&
		fnType.NumIn() == 2 &&
		fnType.In(0) == reflect.TypeOf(zeroCtx) &&
		(fnType.In(1).Kind() == reflect.Ptr || fnType.In(1).Kind() == reflect.UnsafePointer)
	if !isWalkFn {
		return fmt.Errorf("%T is not a valid walkFn - it must be a func(%T, *ArbitraryType) error", fn, zeroCtx)
	}

	_, fp := g_reflect.TypeAndPtrOf(fn)
	castFn := *(*walkFn[Ctx])(unsafe.Pointer(&fp))

	argType := fnType.In(1).Elem()
	w.typeFns[argType] = castFn
	return nil
}

// RegisterCompileBoolFn registers a compile function for types of kind Bool.
func (w *Register[Ctx]) RegisterCompileBoolFn(fn CompileFn[Ctx, bool]) {
	w.compileFns[reflect.Bool] = eraseTypedCompileFn(fn)
}

// RegisterCompileIntFn registers a compile function for types of kind Int.
func (w *Register[Ctx]) RegisterCompileIntFn(fn CompileFn[Ctx, int]) {
	w.compileFns[reflect.Int] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt8Fn registers a compile function for types of kind Int8.
func (w *Register[Ctx]) RegisterCompileInt8Fn(fn CompileFn[Ctx, int8]) {
	w.compileFns[reflect.Int8] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt16Fn registers a compile function for types of kind Int16.
func (w *Register[Ctx]) RegisterCompileInt16Fn(fn CompileFn[Ctx, int16]) {
	w.compileFns[reflect.Int16] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt32Fn registers a compile function for types of kind Int32.
func (w *Register[Ctx]) RegisterCompileInt32Fn(fn CompileFn[Ctx, int32]) {
	w.compileFns[reflect.Int32] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt64Fn registers a compile function for types of kind Int64.
func (w *Register[Ctx]) RegisterCompileInt64Fn(fn CompileFn[Ctx, int64]) {
	w.compileFns[reflect.Int64] = eraseTypedCompileFn(fn)
}

// RegisterCompileUintFn registers a compile function for types of kind Uint.
func (w *Register[Ctx]) RegisterCompileUintFn(fn CompileFn[Ctx, uint]) {
	w.compileFns[reflect.Uint] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint8Fn registers a compile function for types of kind Uint8.
func (w *Register[Ctx]) RegisterCompileUint8Fn(fn CompileFn[Ctx, uint8]) {
	w.compileFns[reflect.Uint8] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint16Fn registers a compile function for types of kind Uint16.
func (w *Register[Ctx]) RegisterCompileUint16Fn(fn CompileFn[Ctx, uint16]) {
	w.compileFns[reflect.Uint16] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint32Fn registers a compile function for types of kind Uint32.
func (w *Register[Ctx]) RegisterCompileUint32Fn(fn CompileFn[Ctx, uint32]) {
	w.compileFns[reflect.Uint32] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint64Fn registers a compile function for types of kind Uint64.
func (w *Register[Ctx]) RegisterCompileUint64Fn(fn CompileFn[Ctx, uint64]) {
	w.compileFns[reflect.Uint64] = eraseTypedCompileFn(fn)
}

// RegisterCompileUintptrFn registers a compile function for types of kind Uintptr.
func (w *Register[Ctx]) RegisterCompileUintptrFn(fn CompileFn[Ctx, uintptr]) {
	w.compileFns[reflect.Uintptr] = eraseTypedCompileFn(fn)
}

// RegisterCompileFloat32Fn registers a compile function for types of kind Float32.
func (w *Register[Ctx]) RegisterCompileFloat32Fn(fn CompileFn[Ctx, float32]) {
	w.compileFns[reflect.Float32] = eraseTypedCompileFn(fn)
}

// RegisterCompileFloat64Fn registers a compile function for types of kind Float64.
func (w *Register[Ctx]) RegisterCompileFloat64Fn(fn CompileFn[Ctx, float64]) {
	w.compileFns[reflect.Float64] = eraseTypedCompileFn(fn)
}

// RegisterCompileComplex64Fn registers a compile function for types of kind Complex64.
func (w *Register[Ctx]) RegisterCompileComplex64Fn(fn CompileFn[Ctx, complex64]) {
	w.compileFns[reflect.Complex64] = eraseTypedCompileFn(fn)
}

// RegisterCompileComplex128Fn registers a compile function for types of kind Complex128.
func (w *Register[Ctx]) RegisterCompileComplex128Fn(fn CompileFn[Ctx, complex128]) {
	w.compileFns[reflect.Complex128] = eraseTypedCompileFn(fn)
}

// RegisterCompileStringFn registers a compile function for types of kind String.
func (w *Register[Ctx]) RegisterCompileStringFn(fn CompileFn[Ctx, string]) {
	w.compileFns[reflect.String] = eraseTypedCompileFn(fn)
}

// RegisterCompileUnsafePointerFn registers a compile function for types of kind UnsafePointer.
func (w *Register[Ctx]) RegisterCompileUnsafePointerFn(fn CompileFn[Ctx, unsafe.Pointer]) {
	w.compileFns[reflect.UnsafePointer] = eraseTypedCompileFn(fn)
}

type CompileStructFn[Ctx any] func(reflect.Type, StructFieldRegister[Ctx]) (StructWalkFn[Ctx], error)
type StructWalkFn[Ctx any] func(Ctx, StructFieldWalker[Ctx]) error

func (w *Register[Ctx]) RegisterCompileStructFn(fn CompileStructFn[Ctx]) {
	w.compileFns[reflect.Struct] = eraseCompileStructFn(fn)
}

type CompileArrayFn[Ctx any] func(reflect.Type) (ArrayWalkFn[Ctx], error)
type ArrayWalkFn[Ctx any] func(Ctx, ArrayWalker[Ctx]) error

func (w *Register[Ctx]) RegisterCompileArrayFn(fn CompileArrayFn[Ctx]) {
	w.compileFns[reflect.Array] = eraseCompileArrayFn(fn)
}

type CompilePtrFn[Ctx any] func(reflect.Type) (PtrWalkFn[Ctx], error)
type PtrWalkFn[Ctx any] func(Ctx, PtrWalker[Ctx]) error

func (w *Register[Ctx]) RegisterCompilePtrFn(fn CompilePtrFn[Ctx]) {
	w.compileFns[reflect.Ptr] = eraseCompilePtrFn(fn)
}

type CompileSliceFn[Ctx any] func(reflect.Type) (SliceWalkFn[Ctx], error)
type SliceWalkFn[Ctx any] func(Ctx, SliceWalker[Ctx]) error

func (w *Register[Ctx]) RegisterCompileSliceFn(fn CompileSliceFn[Ctx]) {
	w.compileFns[reflect.Slice] = eraseCompileSliceFn(fn)
}

func eraseTypedCompileFn[Ctx any, In any](fn CompileFn[Ctx, In]) unsafe.Pointer {
	_, fp := g_reflect.TypeAndPtrOf(fn)
	return fp
}

func eraseCompileArrayFn[Ctx any](fn CompileArrayFn[Ctx]) unsafe.Pointer {
	_, fp := g_reflect.TypeAndPtrOf(fn)
	return fp
}

func eraseCompilePtrFn[Ctx any](fn CompilePtrFn[Ctx]) unsafe.Pointer {
	_, fp := g_reflect.TypeAndPtrOf(fn)
	return fp
}

func eraseCompileSliceFn[Ctx any](fn CompileSliceFn[Ctx]) unsafe.Pointer {
	_, fp := g_reflect.TypeAndPtrOf(fn)
	return fp
}

func eraseCompileStructFn[Ctx any](fn CompileStructFn[Ctx]) unsafe.Pointer {
	_, fp := g_reflect.TypeAndPtrOf(fn)
	return fp
}

func castTo[Out any](p unsafe.Pointer) Out {
	return *(*Out)(unsafe.Pointer(&p))
}
