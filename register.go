package type_walk

import (
	g_reflect "github.com/goccy/go-reflect"
	"reflect"
	"unsafe"
)

type arg struct {
	p       unsafe.Pointer
	canAddr bool
}

func (a arg) CanSet() bool {
	return a.canAddr
}

type Arg[T any] struct {
	arg
}

func (a Arg[T]) Get() T {
	return *(*T)(a.arg.p)
}

func (a Arg[T]) Set(value T) {
	*(*T)(a.arg.p) = value
}

type Bool = Arg[bool]
type Int = Arg[int]
type Int8 = Arg[int8]
type Int16 = Arg[int16]
type Int32 = Arg[int32]
type Int64 = Arg[int64]
type Uint = Arg[uint]
type Uint8 = Arg[uint8]
type Uint16 = Arg[uint16]
type Uint32 = Arg[uint32]
type Uint64 = Arg[uint64]
type Uintptr = Arg[uintptr]
type Float32 = Arg[float32]
type Float64 = Arg[float64]
type Complex64 = Arg[complex64]
type Complex128 = Arg[complex128]
type String = Arg[string]
type UnsafePointer = Arg[unsafe.Pointer]

// WalkFn defines the function that will be called when a value of type In is encountered while walking.
type WalkFn[Ctx any, In any] func(Ctx, Arg[In]) error

// CompileFn defines the function type that will be called to generate a WalkFn when a value is encountered while
// walking, if a WalkFn has not already been registered.
type CompileFn[Ctx any, In any] func(reflect.Type) WalkFn[Ctx, In]

type typeFnEntry[Ctx any] struct {
	t  g_reflect.Type
	fn walkFn[Ctx]
}

// Register stores a set of WalkFns used to walk specific types, and functions to compile WalkFns for kinds of types.
type Register[Ctx any] struct {
	typeFns    []typeFnEntry[Ctx]
	compileFns [NUM_KIND]unsafe.Pointer
}

// NewRegister creates a new register.
func NewRegister[Ctx any]() *Register[Ctx] {
	return &Register[Ctx]{}
}

func RegisterTypeFn[Ctx any, In any](register *Register[Ctx], fn WalkFn[Ctx, In]) {
	inType := reflectType[In]()
	_, fp := g_reflect.TypeAndPtrOf(fn)
	castFn := *(*walkFn[Ctx])(unsafe.Pointer(&fp))
	register.typeFns = append(register.typeFns, typeFnEntry[Ctx]{t: inType, fn: castFn})
}

// RegisterCompileBoolFn registers a compile function for types of kind Bool.
func (r *Register[Ctx]) RegisterCompileBoolFn(fn CompileFn[Ctx, bool]) {
	r.compileFns[reflect.Bool] = eraseTypedCompileFn(fn)
}

// RegisterCompileIntFn registers a compile function for types of kind Int.
func (r *Register[Ctx]) RegisterCompileIntFn(fn CompileFn[Ctx, int]) {
	r.compileFns[reflect.Int] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt8Fn registers a compile function for types of kind Int8.
func (r *Register[Ctx]) RegisterCompileInt8Fn(fn CompileFn[Ctx, int8]) {
	r.compileFns[reflect.Int8] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt16Fn registers a compile function for types of kind Int16.
func (r *Register[Ctx]) RegisterCompileInt16Fn(fn CompileFn[Ctx, int16]) {
	r.compileFns[reflect.Int16] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt32Fn registers a compile function for types of kind Int32.
func (r *Register[Ctx]) RegisterCompileInt32Fn(fn CompileFn[Ctx, int32]) {
	r.compileFns[reflect.Int32] = eraseTypedCompileFn(fn)
}

// RegisterCompileInt64Fn registers a compile function for types of kind Int64.
func (r *Register[Ctx]) RegisterCompileInt64Fn(fn CompileFn[Ctx, int64]) {
	r.compileFns[reflect.Int64] = eraseTypedCompileFn(fn)
}

// RegisterCompileUintFn registers a compile function for types of kind Uint.
func (r *Register[Ctx]) RegisterCompileUintFn(fn CompileFn[Ctx, uint]) {
	r.compileFns[reflect.Uint] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint8Fn registers a compile function for types of kind Uint8.
func (r *Register[Ctx]) RegisterCompileUint8Fn(fn CompileFn[Ctx, uint8]) {
	r.compileFns[reflect.Uint8] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint16Fn registers a compile function for types of kind Uint16.
func (r *Register[Ctx]) RegisterCompileUint16Fn(fn CompileFn[Ctx, uint16]) {
	r.compileFns[reflect.Uint16] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint32Fn registers a compile function for types of kind Uint32.
func (r *Register[Ctx]) RegisterCompileUint32Fn(fn CompileFn[Ctx, uint32]) {
	r.compileFns[reflect.Uint32] = eraseTypedCompileFn(fn)
}

// RegisterCompileUint64Fn registers a compile function for types of kind Uint64.
func (r *Register[Ctx]) RegisterCompileUint64Fn(fn CompileFn[Ctx, uint64]) {
	r.compileFns[reflect.Uint64] = eraseTypedCompileFn(fn)
}

// RegisterCompileUintptrFn registers a compile function for types of kind Uintptr.
func (r *Register[Ctx]) RegisterCompileUintptrFn(fn CompileFn[Ctx, uintptr]) {
	r.compileFns[reflect.Uintptr] = eraseTypedCompileFn(fn)
}

// RegisterCompileFloat32Fn registers a compile function for types of kind Float32.
func (r *Register[Ctx]) RegisterCompileFloat32Fn(fn CompileFn[Ctx, float32]) {
	r.compileFns[reflect.Float32] = eraseTypedCompileFn(fn)
}

// RegisterCompileFloat64Fn registers a compile function for types of kind Float64.
func (r *Register[Ctx]) RegisterCompileFloat64Fn(fn CompileFn[Ctx, float64]) {
	r.compileFns[reflect.Float64] = eraseTypedCompileFn(fn)
}

// RegisterCompileComplex64Fn registers a compile function for types of kind Complex64.
func (r *Register[Ctx]) RegisterCompileComplex64Fn(fn CompileFn[Ctx, complex64]) {
	r.compileFns[reflect.Complex64] = eraseTypedCompileFn(fn)
}

// RegisterCompileComplex128Fn registers a compile function for types of kind Complex128.
func (r *Register[Ctx]) RegisterCompileComplex128Fn(fn CompileFn[Ctx, complex128]) {
	r.compileFns[reflect.Complex128] = eraseTypedCompileFn(fn)
}

// RegisterCompileStringFn registers a compile function for types of kind String.
func (r *Register[Ctx]) RegisterCompileStringFn(fn CompileFn[Ctx, string]) {
	r.compileFns[reflect.String] = eraseTypedCompileFn(fn)
}

// RegisterCompileUnsafePointerFn registers a compile function for types of kind UnsafePointer.
func (r *Register[Ctx]) RegisterCompileUnsafePointerFn(fn CompileFn[Ctx, unsafe.Pointer]) {
	r.compileFns[reflect.UnsafePointer] = eraseTypedCompileFn(fn)
}

type CompileStructFn[Ctx any] func(reflect.Type, StructFieldRegister) (StructWalkFn[Ctx], error)
type StructWalkFn[Ctx any] func(Ctx, Struct[Ctx]) error

func (r *Register[Ctx]) RegisterCompileStructFn(fn CompileStructFn[Ctx]) {
	r.compileFns[reflect.Struct] = eraseCompileStructFn(fn)
}

type CompileArrayFn[Ctx any] func(reflect.Type) (ArrayWalkFn[Ctx], error)
type ArrayWalkFn[Ctx any] func(Ctx, Array[Ctx]) error

func (r *Register[Ctx]) RegisterCompileArrayFn(fn CompileArrayFn[Ctx]) {
	r.compileFns[reflect.Array] = eraseCompileArrayFn(fn)
}

type CompilePtrFn[Ctx any] func(reflect.Type) (PtrWalkFn[Ctx], error)
type PtrWalkFn[Ctx any] func(Ctx, Ptr[Ctx]) error

func (r *Register[Ctx]) RegisterCompilePtrFn(fn CompilePtrFn[Ctx]) {
	r.compileFns[reflect.Ptr] = eraseCompilePtrFn(fn)
}

type CompileSliceFn[Ctx any] func(reflect.Type) (SliceWalkFn[Ctx], error)
type SliceWalkFn[Ctx any] func(Ctx, Slice[Ctx]) error

func (r *Register[Ctx]) RegisterCompileSliceFn(fn CompileSliceFn[Ctx]) {
	r.compileFns[reflect.Slice] = eraseCompileSliceFn(fn)
}

type CompileMapFn[Ctx any] func(reflect.Type) (MapWalkFn[Ctx], error)
type MapWalkFn[Ctx any] func(Ctx, Map[Ctx]) error

func (r *Register[Ctx]) RegisterCompileMapFn(fn CompileMapFn[Ctx]) {
	r.compileFns[reflect.Map] = eraseCompileMapFn(fn)
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

func eraseCompileMapFn[Ctx any](fn CompileMapFn[Ctx]) unsafe.Pointer {
	_, fp := g_reflect.TypeAndPtrOf(fn)
	return fp
}

func castTo[Out any](p unsafe.Pointer) Out {
	return *(*Out)(unsafe.Pointer(&p))
}

func reflectType[T any]() g_reflect.Type {
	return g_reflect.TypeOf((*T)(nil)).Elem()
}
