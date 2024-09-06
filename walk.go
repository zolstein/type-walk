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

type walkFn[Ctx any] func(Ctx, arg) error

// Must return walkFn[Ctx] for provided type.
type compileFn[Ctx any] func(reflect.Type) walkFn[Ctx]

// TypeWalker wraps a Walker to walk values of a specific type.
type TypeWalker[Ctx any, In any] struct {
	fn WalkFn[Ctx, In]
}

// NewTypeWalker creates a new TypeWalker from the provided Walker.
func NewTypeWalker[Ctx any, In any](w *Walker[Ctx]) (*TypeWalker[Ctx, In], error) {
	fnPtr, err := w.getFn(reflectType[In]())
	if err != nil {
		return nil, err
	}
	fn := *(*unsafe.Pointer)(unsafe.Pointer(fnPtr))
	castFn := castTo[WalkFn[Ctx, In]](fn)
	return &TypeWalker[Ctx, In]{fn: castFn}, nil
}

// Walk walks a value of type In.
func (w *TypeWalker[Ctx, In]) Walk(ctx Ctx, in *In) error {
	return w.fn(ctx, argFor(in))
}

func argFor[T any](ptr *T) Arg[T] {
	return Arg[T]{
		arg{
			p: unsafe.Pointer(ptr),
			// canAddr is true because we're coming directly through a pointer.
			canAddr: true,
		},
	}
}

type Walker[Ctx any] struct {
	typeFns    map[g_reflect.Type]*walkFn[Ctx]
	compileFns [NUM_KIND]unsafe.Pointer
}

// NewWalker creates a new Walker from the registered functions in register.
// Any new functions that are registered in the register after calling NewWalker will not be used by the returned Walker.
func NewWalker[Ctx any](register *Register[Ctx]) *Walker[Ctx] {
	typeFns := make(map[g_reflect.Type]*walkFn[Ctx], len(register.typeFns))
	for _, e := range register.typeFns {
		typeFns[e.t] = &e.fn
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
	arg := arg{
		p:       p,
		canAddr: reflect.ValueOf(in).CanAddr(),
	}
	return (*fn)(ctx, arg)
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
	case g_reflect.Map:
		return w.compileMap(t, castTo[CompileMapFn[Ctx]](fnPtr))
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
		structWalker := Array[Ctx]{meta: &arrayMeta, arg: arg}
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
		structWalker := Ptr[Ctx]{meta: &ptrMeta, arg: arg}
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
		structWalker := Slice[Ctx]{meta: &sliceMeta, arg: arg}
		return sliceWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compileStruct(t g_reflect.Type, fn CompileStructFn[Ctx]) (walkFn[Ctx], error) {
	reg := structFieldRegister{
		typ: t,
	}
	structWalkFn, err := fn(g_reflect.ToReflectType(t), StructFieldRegister{&reg})
	if err != nil {
		return nil, err
	}
	meta := &structMetadata[Ctx]{
		typ:          t,
		fieldOffsets: make([]uintptr, len(reg.fields)),
		fieldFns:     make([]*walkFn[Ctx], len(reg.fields)),
	}
	for i := range reg.fields {
		f := &reg.fields[i]
		meta.fieldOffsets[i] = f.Offset
		fn, err := w.getFn(f.Type)
		if err != nil {
			return nil, err
		}
		meta.fieldFns[i] = fn
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := Struct[Ctx]{meta: meta, arg: arg}
		return structWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compileMap(t g_reflect.Type, fn CompileMapFn[Ctx]) (walkFn[Ctx], error) {
	keyFn, err := w.getFn(t.Key())
	if err != nil {
		return nil, err
	}
	valFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	mapMeta := mapMetadata[Ctx]{
		typ:   t,
		keyFn: keyFn,
		valFn: valFn,
	}
	mapWalkFn, err := fn(g_reflect.ToReflectType(t))
	if err != nil {
		return nil, err
	}
	return func(ctx Ctx, arg arg) error {
		mapWalker := Map[Ctx]{meta: &mapMeta, arg: arg}
		return mapWalkFn(ctx, mapWalker)
	}, nil
}

// StructFieldRegister stores information about which fields to walk within a struct.
type StructFieldRegister struct {
	*structFieldRegister
}

type structFieldRegister struct {
	typ    g_reflect.Type
	fields []g_reflect.StructField
}

// RegisterField registers a field by field number, to be available while walking the struct.
// When walking the struct, Struct.Walk(n) will walk the nth field registered.
func (r *structFieldRegister) RegisterField(fieldNum int) int {
	idx := len(r.fields)
	f := r.typ.Field(fieldNum)
	r.fields = append(r.fields, f)
	return idx
}

type structMetadata[Ctx any] struct {
	typ          g_reflect.Type
	fieldOffsets []uintptr
	fieldFns     []*walkFn[Ctx]
}

// Struct is used to walk a struct value.
type Struct[Ctx any] struct {
	meta *structMetadata[Ctx]
	arg  arg
}

// NumFields returns the number of registered fields that can be walked.
func (w *Struct[Ctx]) NumFields() int {
	return len(w.meta.fieldFns)
}

// Walk walks a registered field of the struct value.
// idx must be in the range [0..NumFields())
func (w *Struct[Ctx]) Walk(ctx Ctx, idx int) error {
	fieldArg := arg{
		p: unsafe.Add(w.arg.p, w.meta.fieldOffsets[idx]),
		// A field is addressable iff the struct is addressable.
		canAddr: w.arg.canAddr,
	}
	fieldFn := w.meta.fieldFns[idx]
	return (*fieldFn)(ctx, fieldArg)
}

type arrayMetadata[Ctx any] struct {
	typ      g_reflect.Type
	elemSize uintptr
	length   int
	elemFn   *walkFn[Ctx]
}

// Array is used to walk the elements of an array value.
type Array[Ctx any] struct {
	meta *arrayMetadata[Ctx]
	arg  arg
}

// Len returns the length of the array value.
func (a Array[Ctx]) Len() int {
	return a.meta.length
}

// Walk walks an element of the array value.
// idx must be in the range [0..Len())
func (a *Array[Ctx]) Walk(ctx Ctx, idx int) error {
	if idx < 0 || idx >= a.meta.length {
		panic("Index out of bounds")
	}
	elemArg := arg{
		p: unsafe.Add(a.arg.p, a.meta.elemSize*uintptr(idx)),
		// An element of an array is addressable iff the array is addressable.
		canAddr: a.arg.canAddr,
	}
	return (*a.meta.elemFn)(ctx, elemArg)
}

type sliceMetadata[Ctx any] struct {
	typ      g_reflect.Type
	elemSize uintptr
	elemFn   *walkFn[Ctx]
}

// Slice represents a slice value being walked.
type Slice[Ctx any] struct {
	meta *sliceMetadata[Ctx]
	arg  arg
}

// Len returns the length of the slice value.
func (s Slice[Ctx]) Len() int {
	return len(s.argSlice())
}

// Cap returns the capacity of the slice value.
func (s Slice[Ctx]) Cap() int {
	return cap(s.argSlice())
}

// IsNil returns if the slice value is nil.
func (s Slice[Ctx]) IsNil() bool {
	return s.argSlice() == nil
}

// Walk walks an element of the slice value.
// idx must be in the range [0..Len())
func (s *Slice[Ctx]) Walk(ctx Ctx, idx int) error {
	slice := s.argSlice()
	if idx < 0 || idx >= len(slice) {
		panic("Index out of bounds")
	}
	p := unsafe.Pointer(unsafe.SliceData(slice))
	elemArg := arg{
		p: unsafe.Add(p, s.meta.elemSize*uintptr(idx)),
		// An element of a slice is always addressable because the slice implicitly includes a pointer.
		canAddr: true,
	}
	return (*s.meta.elemFn)(ctx, elemArg)
}

func (s *Slice[Ctx]) argSlice() []struct{} {
	return *(*[]struct{})(s.arg.p)
}

type ptrMetadata[Ctx any] struct {
	typ    g_reflect.Type
	elemFn *walkFn[Ctx]
}

// Ptr represents a pointer value being walked.
type Ptr[Ctx any] struct {
	meta *ptrMetadata[Ctx]
	arg  arg
}

// IsNil returns if the pointer value is nil.
func (p *Ptr[Ctx]) IsNil() bool {
	return *castTo[*unsafe.Pointer](p.arg.p) == nil
}

// Walk walks the value pointed at by the pointer value.
// The pointer value must not be nil.
func (p *Ptr[Ctx]) Walk(ctx Ctx) error {
	elemArg := arg{
		p: *castTo[*unsafe.Pointer](p.arg.p),
		// The value behind a pointer is always addressable - we have the pointer!
		canAddr: true,
	}
	return (*p.meta.elemFn)(ctx, elemArg)
}

type mapMetadata[Ctx any] struct {
	typ   g_reflect.Type
	keyFn *walkFn[Ctx]
	valFn *walkFn[Ctx]
}

type Map[Ctx any] struct {
	meta *mapMetadata[Ctx]
	arg  arg
}

func (m *Map[Ctx]) IsNil() bool {
	return *castTo[*unsafe.Pointer](m.arg.p) == nil
}

func (m *Map[Ctx]) Iter() MapIter[Ctx] {
	ptr := m.arg.p
	rMap := g_reflect.NewAt(m.meta.typ, ptr).Elem()
	return MapIter[Ctx]{
		meta: m.meta,
		iter: rMap.MapRange(),
	}
}

type MapIter[Ctx any] struct {
	meta *mapMetadata[Ctx]
	iter *g_reflect.MapIter
}

func (m *MapIter[Ctx]) Next() bool {
	return m.iter.Next()
}

func (m *MapIter[Ctx]) Entry() MapEntry[Ctx] {
	key := m.iter.Key().Interface()
	val := m.iter.Value().Interface()
	_, keyPtr := g_reflect.TypeAndPtrOf(key)
	_, valPtr := g_reflect.TypeAndPtrOf(val)
	return MapEntry[Ctx]{
		meta:   m.meta,
		keyPtr: keyPtr,
		valPtr: valPtr,
	}
}

type MapEntry[Ctx any] struct {
	meta   *mapMetadata[Ctx]
	keyPtr unsafe.Pointer
	valPtr unsafe.Pointer
}

func (m *MapEntry[Ctx]) WalkKey(ctx Ctx) error {
	keyArg := arg{
		p: m.keyPtr,
		// Map element isn't indexable.
		canAddr: false,
	}
	return (*m.meta.keyFn)(ctx, keyArg)
}

func (m *MapEntry[Ctx]) WalkValue(ctx Ctx) error {
	valArg := arg{
		p: m.valPtr,
		// Map element isn't indexable.
		canAddr: false,
	}
	return (*m.meta.valFn)(ctx, valArg)
}
