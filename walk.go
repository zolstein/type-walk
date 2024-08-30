package type_walk

import (
	"fmt"
	"reflect"
	"unsafe"

	g_reflect "github.com/goccy/go-reflect"
)

const (
	NUM_KIND = 27
)

type arg struct {
	p unsafe.Pointer
}

// WalkFn defines the function that will be called when a value of type In is encountered while walking.
type WalkFn[Ctx any, In any] func(Ctx, *In) error

type walkFn[Ctx any] func(Ctx, arg) error

// Must return walkFn[Ctx] for provided type.
type compileFn[Ctx any] func(reflect.Type) walkFn[Ctx]

// TypeWalker wraps a Walker to walk values of a specific type.
type TypeWalker[Ctx any, In any] struct {
	fn WalkFn[Ctx, In]
}

// NewTypeWalker creates a new TypeWalker from the provided Walker.
func NewTypeWalker[Ctx any, In any](w *Walker[Ctx]) (*TypeWalker[Ctx, In], error) {
	var zero In
	fnPtr, err := w.getFn(g_reflect.TypeOf(zero))
	if err != nil {
		return nil, err
	}
	castFn := castTo[WalkFn[Ctx, In]](*(*unsafe.Pointer)(unsafe.Pointer(fnPtr)))
	return &TypeWalker[Ctx, In]{fn: castFn}, nil
}

// Walk walks a value of type In.
func (w *TypeWalker[Ctx, In]) Walk(ctx Ctx, in *In) error {
	return w.fn(ctx, in)
}

type Walker[Ctx any] struct {
	typeFns    map[g_reflect.Type]*walkFn[Ctx]
	compileFns [NUM_KIND]unsafe.Pointer
}

// NewWalker creates a new Walker from the registered functions in register.
// Any new functions that are registered in the register after calling NewWalker will not be used by the returned Walker.
func NewWalker[Ctx any](register *Register[Ctx]) *Walker[Ctx] {
	typeFns := make(map[g_reflect.Type]*walkFn[Ctx], len(register.typeFns))
	for t, fn := range register.typeFns {
		typeFns[g_reflect.ToType(t)] = &fn
	}
	return &Walker[Ctx]{
		typeFns:    typeFns,
		compileFns: register.compileFns,
	}
}

// Walk walks in, calling the registered for each value it encounters.
func (w *Walker[Ctx]) Walk(ctx Ctx, in any) error {
	t, p := g_reflect.TypeAndPtrOf(in)
	fn, err := w.getFn(t)
	if err != nil {
		return err
	}
	// XXX: This almost definitely induces an allocation.
	if ptrTypes[t.Kind()] {
		// It's not clear why this needs to copy p to in new variable before taking the reference, but it doesn't work
		// without this.
		var copyP = p
		p = unsafe.Pointer(&copyP)
	}
	return (*fn)(ctx, arg{p})
}

var ptrTypes = [NUM_KIND]bool{
	g_reflect.Ptr:           true,
	g_reflect.UnsafePointer: true,
	g_reflect.Map:           true,
	g_reflect.Chan:          true,
	g_reflect.Func:          true,
}

func (w *Walker[Ctx]) getFn(t g_reflect.Type) (fn *walkFn[Ctx], err error) {
	fn, ok := w.typeFns[t]
	if !ok {
		fn = new(walkFn[Ctx])
		w.typeFns[t] = fn
		*fn, err = w.compileFn(t)
	}
	return fn, err
}

func (w *Walker[Ctx]) compileFn(t g_reflect.Type) (walkFn[Ctx], error) {
	k := t.Kind()
	fnPtr := w.compileFns[k]
	if fnPtr == nil {
		return nil, fmt.Errorf("no registered handler for type kind %v", k)
	}
	switch k {
	case g_reflect.Array:
		return w.compileArray(t, castTo[CompileArrayFn[Ctx]](fnPtr))
	case g_reflect.Ptr:
		return w.compilePtr(t, castTo[CompilePtrFn[Ctx]](fnPtr))
	case g_reflect.Slice:
		return w.compileSlice(t, castTo[CompileSliceFn[Ctx]](fnPtr))
	case g_reflect.Struct:
		return w.compileStruct(t, castTo[CompileStructFn[Ctx]](fnPtr))
	default:
		compileFn := castTo[compileFn[Ctx]](fnPtr)
		return compileFn(g_reflect.ToReflectType(t)), nil
	}
}

func (w *Walker[Ctx]) compileArray(t g_reflect.Type, fn CompileArrayFn[Ctx]) (walkFn[Ctx], error) {
	elemFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	arrayMeta := arrayMetadata[Ctx]{
		elemSize: t.Elem().Size(),
		length:   t.Len(),
		elemFn:   elemFn,
	}
	arrayWalkFn, err := fn(g_reflect.ToReflectType(t))
	if err != nil {
		return nil, err
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := ArrayWalker[Ctx]{meta: &arrayMeta, arg: arg}
		return arrayWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compilePtr(t g_reflect.Type, fn CompilePtrFn[Ctx]) (walkFn[Ctx], error) {
	elemFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	ptrMeta := ptrMetadata[Ctx]{elemFn: elemFn}
	ptrWalkFn, err := fn(g_reflect.ToReflectType(t))
	if err != nil {
		return nil, err
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := PtrWalker[Ctx]{meta: &ptrMeta, arg: arg}
		return ptrWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compileSlice(t g_reflect.Type, fn CompileSliceFn[Ctx]) (walkFn[Ctx], error) {
	elemFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	sliceMeta := sliceMetadata[Ctx]{
		elemSize: t.Elem().Size(),
		elemFn:   elemFn,
	}
	sliceWalkFn, err := fn(g_reflect.ToReflectType(t))
	if err != nil {
		return nil, err
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := SliceWalker[Ctx]{meta: &sliceMeta, arg: arg}
		return sliceWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compileStruct(t g_reflect.Type, fn CompileStructFn[Ctx]) (walkFn[Ctx], error) {
	reg := structFieldRegister[Ctx]{
		walker: w, typ: t,
	}
	structWalkFn, err := fn(g_reflect.ToReflectType(t), StructFieldRegister[Ctx]{&reg})
	if err != nil {
		return nil, err
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := StructFieldWalker[Ctx]{register: &reg, arg: arg}
		return structWalkFn(ctx, structWalker)
	}, nil
}

// StructFieldRegister stores information about which fields to walk within a struct.
type StructFieldRegister[Ctx any] struct {
	*structFieldRegister[Ctx]
}

type structFieldRegister[Ctx any] struct {
	walker       *Walker[Ctx]
	typ          g_reflect.Type
	fieldOffsets []uintptr
	fieldFns     []*walkFn[Ctx]
}

// RegisterField registers a field by field number, to be available while walking the struct.
// When walking the struct, StructFieldWalker.Walk(n) will walk the nth field registered.
func (w *structFieldRegister[Ctx]) RegisterField(fieldNum int) (int, error) {
	f := w.typ.Field(fieldNum)
	ft := f.Type
	idx := len(w.fieldOffsets)
	w.fieldOffsets = append(w.fieldOffsets, f.Offset)
	fn, err := w.walker.getFn(ft)
	if err != nil {
		return 0, err
	}
	w.fieldFns = append(w.fieldFns, fn)

	return idx, nil
}

// StructFieldWalker is used to walk a struct value.
type StructFieldWalker[Ctx any] struct {
	register *structFieldRegister[Ctx]
	arg      arg
}

// NumFields returns the number of registered fields that can be walked.
func (w *StructFieldWalker[Ctx]) NumFields() int {
	return len(w.register.fieldFns)
}

// Walk walks a registered field of the struct value.
// idx must be in the range [0..NumFields())
func (w *StructFieldWalker[Ctx]) Walk(ctx Ctx, idx int) error {
	fieldArg := arg{unsafe.Add(w.arg.p, w.register.fieldOffsets[idx])}
	fieldFn := w.register.fieldFns[idx]
	return (*fieldFn)(ctx, fieldArg)
}

type arrayMetadata[Ctx any] struct {
	elemSize uintptr
	length   int
	elemFn   *walkFn[Ctx]
}

// ArrayWalker is used to walk the elements of an array value.
type ArrayWalker[Ctx any] struct {
	meta *arrayMetadata[Ctx]
	arg  arg
}

// Len returns the length of the array value.
func (w ArrayWalker[Ctx]) Len() int {
	return w.meta.length
}

// Walk walks an element of the array value.
// idx must be in the range [0..Len())
func (w *ArrayWalker[Ctx]) Walk(ctx Ctx, idx int) error {
	if idx < 0 || idx >= w.meta.length {
		panic("Index out of bounds")
	}
	elemArg := arg{unsafe.Add(w.arg.p, w.meta.elemSize*uintptr(idx))}
	return (*w.meta.elemFn)(ctx, elemArg)
}

type sliceMetadata[Ctx any] struct {
	elemSize uintptr
	elemFn   *walkFn[Ctx]
}

// SliceWalker is used to walk elements of a slice value.
type SliceWalker[Ctx any] struct {
	meta *sliceMetadata[Ctx]
	arg  arg
}

// Len returns the length of the slice value.
func (w SliceWalker[Ctx]) Len() int {
	return len(w.argSlice())
}

// Cap returns the capacity of the slice value.
func (w SliceWalker[Ctx]) Cap() int {
	return cap(w.argSlice())
}

// IsNil returns if the slice value is nil.
func (w SliceWalker[Ctx]) IsNil() bool {
	return w.argSlice() == nil
}

// Walk walks an element of the slice value.
// idx must be in the range [0..Len()]
func (w *SliceWalker[Ctx]) Walk(ctx Ctx, idx int) error {
	s := w.argSlice()
	if idx < 0 || idx >= len(s) {
		panic("Index out of bounds")
	}
	p := unsafe.Pointer(unsafe.SliceData(s))
	elemArg := arg{unsafe.Add(p, w.meta.elemSize*uintptr(idx))}
	return (*w.meta.elemFn)(ctx, elemArg)
}

func (w *SliceWalker[Ctx]) argSlice() []struct{} {
	return *(*[]struct{})(w.arg.p)
}

type ptrMetadata[Ctx any] struct {
	elemFn *walkFn[Ctx]
}

// PtrWalker is used to walk the a pointer value.
type PtrWalker[Ctx any] struct {
	meta *ptrMetadata[Ctx]
	arg  arg
}

// IsNil returns if the pointer value is nil.
func (w *PtrWalker[Ctx]) IsNil() bool {
	return *castTo[*unsafe.Pointer](w.arg.p) == nil
}

// Walk walks the value pointed at by the pointer value.
// The pointer value must not be nil.
func (w *PtrWalker[Ctx]) Walk(ctx Ctx) error {
	elemArg := arg{*castTo[*unsafe.Pointer](w.arg.p)}
	return (*w.meta.elemFn)(ctx, elemArg)
}
