package type_walk

import (
	"fmt"
	"reflect"
	"slices"
	"unsafe"

	g_reflect "github.com/goccy/go-reflect"
)

const (
	NUM_KIND = 27
)

type walkFn[Ctx any] func(Ctx, arg) error

// Must return walkFn[Ctx] for provided type.
type compileFn[Ctx any] func(reflect.Type) walkFn[Ctx]

type TypeFn[Ctx any, In any] func(ctx Ctx, in *In) error

// TypeFnFor returns a TypeFn to walk a value of a particular type.
func TypeFnFor[In any, Ctx any](w *Walker[Ctx]) (TypeFn[Ctx, In], error) {
	fnPtr, err := w.getFn(reflectType[In]())
	if err != nil {
		return nil, err
	}

	fn := *(*unsafe.Pointer)(unsafe.Pointer(fnPtr))
	castFn := castTo[WalkFn[Ctx, In]](fn)

	return func(ctx Ctx, in *In) error {
		return castFn(ctx, argFor(in))
	}, nil
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

// Walker represents a collection of functions that can be used to walk a value using the Walk method.
type Walker[Ctx any] struct {
	typeFns    map[g_reflect.Type]*walkFn[Ctx]
	compileFns [NUM_KIND]unsafe.Pointer
}

// NewWalker creates a new Walker from the registered functions in register.
// Any new functions that are added to the register after calling NewWalker will not be used by the returned Walker.
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
	arrayWalkFn := fn(g_reflect.ToReflectType(t))
	elemFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	arrayMeta := arrayMetadata[Ctx]{
		typ:      t,
		elemSize: t.Elem().Size(),
		length:   t.Len(),
		elemFn:   elemFn,
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := Array[Ctx]{meta: &arrayMeta, arg: arg}
		return arrayWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compilePtr(t g_reflect.Type, fn CompilePtrFn[Ctx]) (walkFn[Ctx], error) {
	ptrWalkFn := fn(g_reflect.ToReflectType(t))
	elemFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	ptrMeta := ptrMetadata[Ctx]{
		typ:    t,
		elemFn: elemFn,
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := Ptr[Ctx]{meta: &ptrMeta, arg: arg}
		return ptrWalkFn(ctx, structWalker)
	}, nil
}

func (w *Walker[Ctx]) compileSlice(t g_reflect.Type, fn CompileSliceFn[Ctx]) (walkFn[Ctx], error) {
	sliceWalkFn := fn(g_reflect.ToReflectType(t))
	elemFn, err := w.getFn(t.Elem())
	if err != nil {
		return nil, err
	}
	sliceMeta := sliceMetadata[Ctx]{
		typ:      t,
		elemSize: t.Elem().Size(),
		elemFn:   elemFn,
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
	structWalkFn := fn(g_reflect.ToReflectType(t), StructFieldRegister{&reg})
	meta := &structMetadata[Ctx]{
		typ:       t,
		fieldInfo: make([]structFieldMetadata[Ctx], len(reg.indexes)),
	}
	for i, idx := range reg.indexes {
		ft := t
		offsets := []uintptr{0}
		for i, x := range idx {
			if i > 0 && ft.Kind() == reflect.Ptr && ft.Elem().Kind() == reflect.Struct {
				ft = ft.Elem()
				offsets = append(offsets, 0)
			}
			f := ft.Field(x)
			ft = f.Type
			offsets[len(offsets)-1] += f.Offset
		}

		fn, err := w.getFn(ft)
		if err != nil {
			return nil, err
		}
		meta.fieldInfo[i] = structFieldMetadata[Ctx]{
			typ:    ft,
			lookup: lookupFieldFn(offsets),
			fn:     fn,
		}
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := Struct[Ctx]{meta: meta, arg: arg}
		return structWalkFn(ctx, structWalker)
	}, nil
}

type lookupFn func(arg) arg

func lookupFieldFn(offsets []uintptr) lookupFn {
	return func(a arg) arg {
		for i, offset := range offsets {
			if i >= 1 {
				// If len(offsets) >= 1, the lookup goes through at least one pointer. In this case, it's necessarily
				// behind a pointer, and therefore addressable.
				a.canAddr = true
				a.p = *(*unsafe.Pointer)(a.p)
				if a.p == nil {
					return arg{}
				}
			}
			a.p = unsafe.Add(a.p, offset)
		}
		return a
	}
}

func (w *Walker[Ctx]) compileMap(t g_reflect.Type, fn CompileMapFn[Ctx]) (walkFn[Ctx], error) {
	mapWalkFn := fn(g_reflect.ToReflectType(t))
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
	typ     g_reflect.Type
	indexes [][]int
	buffer  []int
}

// RegisterField registers a field to be available while walking the struct, by its field number.
// When walking the struct, Struct.Field(n) will return nth field registered.
func (r *structFieldRegister) RegisterField(fieldNum int) int {
	idx := len(r.indexes)
	if r.buffer == nil {
		r.buffer = slices.Grow(r.buffer, max(r.typ.NumField(), 1))
	} else {
		r.buffer = slices.Grow(r.buffer, 1)
	}
	bufLen := len(r.buffer)
	r.buffer = append(r.buffer, fieldNum)
	r.indexes = append(r.indexes, r.buffer[bufLen:bufLen+1])
	return idx
}

// RegisterFieldByIndex registers a field to be available while walking the struct, according to the Index field of
// the reflect.StructField representing the field.
func (r *structFieldRegister) RegisterFieldByIndex(index []int) int {
	if len(index) == 0 {
		panic("index must be non-empty")
	}
	idx := len(r.indexes)
	r.indexes = append(r.indexes, index)
	return idx
}

type structMetadata[Ctx any] struct {
	typ       g_reflect.Type
	fieldInfo []structFieldMetadata[Ctx]
}

type structFieldMetadata[Ctx any] struct {
	typ    g_reflect.Type
	lookup lookupFn
	fn     *walkFn[Ctx]
}

// Struct represents a struct value.
type Struct[Ctx any] struct {
	meta *structMetadata[Ctx]
	arg  arg
}

// NumFields returns the number of registered fields that can be walked.
func (s Struct[Ctx]) NumFields() int {
	return len(s.meta.fieldInfo)
}

// Field returns the StructField value for a registered field, by index in the order the fields were registered.
// idx must be in the range [0..NumFields())
func (s Struct[Ctx]) Field(idx int) StructField[Ctx] {
	meta := &s.meta.fieldInfo[idx]
	return StructField[Ctx]{
		meta: meta,
		arg:  meta.lookup(s.arg),
	}
}

// Walk walks a registered field of the struct value.
// idx must be in the range [0..NumFields())
func (s Struct[Ctx]) Walk(ctx Ctx, idx int) error {
	return s.Field(idx).Walk(ctx)
}

func (s Struct[Ctx]) Interface() any {
	return g_reflect.NewAt(s.meta.typ, s.arg.p).Elem().Interface()
}

// StructField represents the field of a struct.
type StructField[Ctx any] struct {
	meta *structFieldMetadata[Ctx]
	arg  arg
}

// IsValid returns true if the StructField is valid, otherwise false. Calling Walk on an invalid struct field panics.
//
// IsValid returns false only if the field is defined on the parent struct by a multipart Index, one or more of the
// intermediate fields is a pointer, and one or more of the pointers used to look up the field on this value is nil.
// For example, in the following type, Example.F1.F2 will be invalid if Example.F1 is nil, but not if F2 is nil.
// Example.F3.F4 can never be invalid, because it is not referenced through a pointer (even though it is itself a
// pointer.)
//
//	type Example struct {
//	    F1 *struct {
//	        F2 *int
//	    }
//	    F3 struct {
//	        F4 *int
//	    }
//	}
func (f StructField[Ctx]) IsValid() bool {
	return f.arg.p != nil
}

// Walk walks the StructField. The StructField must be valid.
func (f StructField[Ctx]) Walk(ctx Ctx) error {
	return (*f.meta.fn)(ctx, f.arg)
}

func (f StructField[Ctx]) Interface() any {
	return g_reflect.NewAt(f.meta.typ, f.arg.p).Elem().Interface()
}

type arrayMetadata[Ctx any] struct {
	typ      g_reflect.Type
	elemSize uintptr
	length   int
	elemFn   *walkFn[Ctx]
}

// Array represents an array value.
type Array[Ctx any] struct {
	meta *arrayMetadata[Ctx]
	arg  arg
}

// Len returns the length of the array value.
func (a Array[Ctx]) Len() int {
	return a.meta.length
}

// Elem returns an ArrayElem representing an element of the array by idx.
// idx must be in the range [0..Len()).
func (a Array[Ctx]) Elem(idx int) ArrayElem[Ctx] {
	if idx < 0 || idx >= a.meta.length {
		panic("Index out of bounds")
	}

	elemArg := arg{
		p: unsafe.Add(a.arg.p, a.meta.elemSize*uintptr(idx)),
		// An element of an array is addressable iff the array is addressable.
		canAddr: a.arg.canAddr,
	}
	return ArrayElem[Ctx]{
		meta: a.meta,
		arg:  elemArg,
	}
}

// Walk walks an element of the array value.
// idx must be in the range [0..Len())
func (a Array[Ctx]) Walk(ctx Ctx, idx int) error {
	return a.Elem(idx).Walk(ctx)
}

func (a Array[Ctx]) Interface() any {
	return g_reflect.NewAt(a.meta.typ, a.arg.p).Elem().Interface()
}

// ArrayElem represents an element of an array.
type ArrayElem[Ctx any] struct {
	meta *arrayMetadata[Ctx]
	arg  arg
}

// Walk walks the ArrayElem.
func (e ArrayElem[Ctx]) Walk(ctx Ctx) error {
	return (*e.meta.elemFn)(ctx, e.arg)
}

func (e ArrayElem[Ctx]) Interface() any {
	return g_reflect.NewAt(e.meta.typ.Elem(), e.arg.p).Elem().Interface()
}

type sliceMetadata[Ctx any] struct {
	typ      g_reflect.Type
	elemSize uintptr
	elemFn   *walkFn[Ctx]
}

// Slice represents a slice value.
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

// Elem returns a SliceElem representing an element of the slice by idx.
// idx must be in the range [0..Len()).
func (s Slice[Ctx]) Elem(idx int) SliceElem[Ctx] {
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
	return SliceElem[Ctx]{
		meta: s.meta,
		arg:  elemArg,
	}
}

// Walk walks an element of the slice value.
// idx must be in the range [0..Len())
func (s Slice[Ctx]) Walk(ctx Ctx, idx int) error {
	return s.Elem(idx).Walk(ctx)
}

func (a Slice[Ctx]) Interface() any {
	return g_reflect.NewAt(a.meta.typ, a.arg.p).Elem().Interface()
}

func (s Slice[Ctx]) argSlice() []struct{} {
	return *(*[]struct{})(s.arg.p)
}

// SliceElem represents an element of an array.
type SliceElem[Ctx any] struct {
	meta *sliceMetadata[Ctx]
	arg  arg
}

// Walk walks the SliceElem.
// idx must be in the range [0..Len())
func (e SliceElem[Ctx]) Walk(ctx Ctx) error {
	return (*e.meta.elemFn)(ctx, e.arg)
}

func (e SliceElem[Ctx]) Interface() any {
	return g_reflect.NewAt(e.meta.typ.Elem(), e.arg.p).Elem().Interface()
}

type ptrMetadata[Ctx any] struct {
	typ    g_reflect.Type
	elemFn *walkFn[Ctx]
}

// Ptr represents a pointer value.
type Ptr[Ctx any] struct {
	meta *ptrMetadata[Ctx]
	arg  arg
}

// IsNil returns if the pointer value is nil.
func (p Ptr[Ctx]) IsNil() bool {
	return *castTo[*unsafe.Pointer](p.arg.p) == nil
}

// Walk walks the value pointed at by the pointer value.
// The pointer value must not be nil.
func (p Ptr[Ctx]) Walk(ctx Ctx) error {
	elemArg := arg{
		p: *castTo[*unsafe.Pointer](p.arg.p),
		// The value behind a pointer is always addressable - we have the pointer!
		canAddr: true,
	}
	return (*p.meta.elemFn)(ctx, elemArg)
}

func (p Ptr[Ctx]) Interface() any {
	return g_reflect.NewAt(p.meta.typ, p.arg.p).Elem().Interface()
}

type mapMetadata[Ctx any] struct {
	typ   g_reflect.Type
	keyFn *walkFn[Ctx]
	valFn *walkFn[Ctx]
}

// Map represents a Map value.
type Map[Ctx any] struct {
	meta *mapMetadata[Ctx]
	arg  arg
}

// IsNil returns whether the map value is nil.
func (m Map[Ctx]) IsNil() bool {
	return *castTo[*unsafe.Pointer](m.arg.p) == nil
}

// Iter returns an iterator over the elements of the map.
func (m Map[Ctx]) Iter() MapIter[Ctx] {
	ptr := m.arg.p
	rMap := g_reflect.NewAt(m.meta.typ, ptr).Elem()
	return MapIter[Ctx]{
		meta: m.meta,
		iter: rMap.MapRange(),
	}
}

func (m Map[Ctx]) Interface() any {
	return g_reflect.NewAt(m.meta.typ, m.arg.p).Elem().Interface()
}

// MapIter represents an iterator over the entries of the map.
type MapIter[Ctx any] struct {
	meta *mapMetadata[Ctx]
	iter *g_reflect.MapIter
}

// Next advances the MapIter to the next entry in the map.
func (m MapIter[Ctx]) Next() bool {
	return m.iter.Next()
}

// Entry returns a MapEntry representing a key and value in the map.
func (m MapIter[Ctx]) Entry() MapEntry[Ctx] {
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

// MapEntry represents a key and value in the map.
type MapEntry[Ctx any] struct {
	meta   *mapMetadata[Ctx]
	keyPtr unsafe.Pointer
	valPtr unsafe.Pointer
}

// Key returns a MapKey representing a key in the map.
func (m MapEntry[Ctx]) Key() MapKey[Ctx] {
	return MapKey[Ctx]{
		meta: m.meta,
		arg: arg{
			p: m.keyPtr,
			// Map element isn't indexable.
			canAddr: false,
		},
	}
}

// Value returns a MapValue representing a value in the map.
func (m MapEntry[Ctx]) Value() MapValue[Ctx] {
	return MapValue[Ctx]{
		meta: m.meta,
		arg: arg{
			p: m.valPtr,
			// Map element isn't indexable.
			canAddr: false,
		},
	}
}

// WalkKey walks the key associated with this MapEntry.
func (m MapEntry[Ctx]) WalkKey(ctx Ctx) error {
	return m.Key().Walk(ctx)
}

// WalkValue walks the value associated with this MapEntry.
func (m MapEntry[Ctx]) WalkValue(ctx Ctx) error {
	return m.Value().Walk(ctx)
}

// MapKey represents a key in the map.
type MapKey[Ctx any] struct {
	meta *mapMetadata[Ctx]
	arg  arg
}

// Walk walks the MapKey.
func (m MapKey[Ctx]) Walk(ctx Ctx) error {
	return (*m.meta.keyFn)(ctx, m.arg)
}

func (m MapKey[Ctx]) Interface() any {
	return g_reflect.NewAt(m.meta.typ.Key(), m.arg.p).Elem().Interface()
}

// MapValue represents a value in the map.
type MapValue[Ctx any] struct {
	meta *mapMetadata[Ctx]
	arg  arg
}

// Walk walks the MapValue.
func (m MapValue[Ctx]) Walk(ctx Ctx) error {
	return (*m.meta.valFn)(ctx, m.arg)
}

func (m MapValue[Ctx]) Interface() any {
	return g_reflect.NewAt(m.meta.typ.Elem(), m.arg.p).Elem().Interface()
}
