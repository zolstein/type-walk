package type_walk

import (
	"fmt"
	g_reflect "github.com/goccy/go-reflect"
	"github.com/zolstein/sync-map"
	"reflect"
	"sync"
	"unsafe"
)

type simpleCompiler[Ctx any] struct {
	typeFns         map[g_reflect.Type]*walkFn[Ctx]
	compileFns      [numKind]unsafe.Pointer
	ifaceConvertFns map[g_reflect.Type]ifaceConvertFn
}

func newSimpleCompiler[Ctx any](register *Register[Ctx]) *simpleCompiler[Ctx] {
	typeFns := make(map[g_reflect.Type]*walkFn[Ctx], len(register.typeFns))
	for i := range register.typeFns {
		e := register.typeFns[i]
		typeFns[e.t] = &e.fn
	}
	ifaceConvertFns := make(map[g_reflect.Type]ifaceConvertFn, len(register.ifaceConvertFns)+1)
	for _, e := range register.ifaceConvertFns {
		ifaceConvertFns[e.t] = e.fn
	}
	ifaceConvertFns[reflectType[any]()] = func(a any) unsafe.Pointer { return unsafe.Pointer(&a) }
	return &simpleCompiler[Ctx]{
		typeFns:         typeFns,
		compileFns:      register.compileFns,
		ifaceConvertFns: ifaceConvertFns,
	}
}

func (c *simpleCompiler[Ctx]) getFn(t g_reflect.Type) (fn *walkFn[Ctx], err error) {
	fn, ok := c.typeFns[t]
	if !ok {
		if t == nil {
			// This panics, rather than returning an error, because it's an easily preventable user error.
			// Check for nil before calling Walk!
			// Maybe we should have a way to specify a handler for a nil interface?
			panic("cannot compile function for nil type")
		}
		fn = new(walkFn[Ctx])
		c.typeFns[t] = fn
		*fn, err = c.compileFn(t)
	}
	return fn, err
}

func (c *simpleCompiler[Ctx]) compileFn(t g_reflect.Type) (walkFn[Ctx], error) {
	k := t.Kind()
	fnPtr := c.compileFns[k]
	if fnPtr == nil {
		return nil, fmt.Errorf("no registered handler for type kind %v", k)
	}
	switch k {
	case g_reflect.Array:
		return c.compileArray(t, castTo[CompileArrayFn[Ctx]](fnPtr))
	case g_reflect.Ptr:
		return c.compilePtr(t, castTo[CompilePtrFn[Ctx]](fnPtr))
	case g_reflect.Slice:
		return c.compileSlice(t, castTo[CompileSliceFn[Ctx]](fnPtr))
	case g_reflect.Struct:
		return c.compileStruct(t, castTo[CompileStructFn[Ctx]](fnPtr))
	case g_reflect.Map:
		return c.compileMap(t, castTo[CompileMapFn[Ctx]](fnPtr))
	case g_reflect.Interface:
		return c.compileInterface(t, castTo[CompileInterfaceFn[Ctx]](fnPtr))
	default:
		compileFn := castTo[compileFn[Ctx]](fnPtr)
		return compileFn(g_reflect.ToReflectType(t)), nil
	}
}

func (c *simpleCompiler[Ctx]) compileArray(t g_reflect.Type, fn CompileArrayFn[Ctx]) (walkFn[Ctx], error) {
	arrayWalkFn := fn(g_reflect.ToReflectType(t))
	elemFn, err := c.getFn(t.Elem())
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

func (c *simpleCompiler[Ctx]) compilePtr(t g_reflect.Type, fn CompilePtrFn[Ctx]) (walkFn[Ctx], error) {
	ptrWalkFn := fn(g_reflect.ToReflectType(t))
	elemFn, err := c.getFn(t.Elem())
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

func (c *simpleCompiler[Ctx]) compileSlice(t g_reflect.Type, fn CompileSliceFn[Ctx]) (walkFn[Ctx], error) {
	sliceWalkFn := fn(g_reflect.ToReflectType(t))
	elemFn, err := c.getFn(t.Elem())
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

func (c *simpleCompiler[Ctx]) compileStruct(t g_reflect.Type, fn CompileStructFn[Ctx]) (walkFn[Ctx], error) {
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

		fn, err := c.getFn(ft)
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

func (c *simpleCompiler[Ctx]) compileMap(t g_reflect.Type, fn CompileMapFn[Ctx]) (walkFn[Ctx], error) {
	mapWalkFn := fn(g_reflect.ToReflectType(t))
	keyType := t.Key()
	keyFn, err := c.getFn(keyType)
	if err != nil {
		return nil, err
	}
	valType := t.Elem()
	valFn, err := c.getFn(valType)
	if err != nil {
		return nil, err
	}
	mapMeta := mapMetadata[Ctx]{
		typ:   t,
		keyFn: keyFn,
		valFn: valFn,
	}
	if keyType.Kind() == reflect.Interface {
		mapMeta.keyConvFn = c.ifaceConvertFns[keyType]
	}
	if valType.Kind() == reflect.Interface {
		mapMeta.valConvFn = c.ifaceConvertFns[valType]
	}
	return func(ctx Ctx, arg arg) error {
		mapWalker := Map[Ctx]{meta: &mapMeta, arg: arg}
		return mapWalkFn(ctx, mapWalker)
	}, nil
}

func (c *simpleCompiler[Ctx]) compileInterface(t g_reflect.Type, fn CompileInterfaceFn[Ctx]) (walkFn[Ctx], error) {
	ifaceWalkFn := fn(g_reflect.ToReflectType(t))
	ifaceMeta := ifaceMetadata[Ctx]{
		typ:   t,
		fnSrc: c.getFn,
	}
	return func(ctx Ctx, arg arg) error {
		structWalker := Interface[Ctx]{meta: &ifaceMeta, arg: arg}
		return ifaceWalkFn(ctx, structWalker)
	}, nil
}

type threadSafeCompiler[Ctx any] struct {
	typeFns sync_map.Map[g_reflect.Type, *walkFn[Ctx]]
	inner   simpleCompiler[Ctx]
	m       sync.Mutex
}

func newThreadSafeCompiler[Ctx any](register *Register[Ctx]) *threadSafeCompiler[Ctx] {
	c := &threadSafeCompiler[Ctx]{
		inner:   *newSimpleCompiler[Ctx](register),
		typeFns: sync_map.Map[g_reflect.Type, *walkFn[Ctx]]{},
	}
	for t, fn := range c.inner.typeFns {
		c.typeFns.Store(t, fn)
	}
	return c
}

func (c *threadSafeCompiler[Ctx]) getFn(t g_reflect.Type) (fn *walkFn[Ctx], err error) {
	// Check typeFns first before grabbing the lock. If it's here, we know it's non-nil, since it only exists in the
	// inner map until we actually return from this function. (There's no parallel vs recursive case.)
	fn, ok := c.typeFns.Load(t)
	if !ok {
		if t == nil {
			// This panics, rather than returning an error, because it's an easily preventable user error.
			// Check for nil before calling Walk!
			// Maybe we should have a way to specify a handler for a nil interface?
			panic("cannot compile function for nil type")
		}

		c.m.Lock()
		defer c.m.Unlock()

		// Check again. If another thread tried to get the same type concurrently, it may have already added it.
		fn, ok = c.typeFns.Load(t)
		if !ok {
			// It's safe to call inner.getFn while holding the lock because no other threads will attempt to read or
			// update its typeFns map concurrently.
			// N.b. It might be slightly better to copy the full simpleCompiler.getFn implementation, make it work
			// directly on typeFns, and explicitly check for nil values while not holding the lock.
			fn, err = c.inner.getFn(t)
			if err != nil {
				c.typeFns.Store(t, fn)
			}
		}
	}
	return fn, err
}
