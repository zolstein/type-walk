package type_walk_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tw "github.com/zolstein/type-walk"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func ptr[T any](t T) *T {
	return &t
}

func TestRegisterTypeFn(t *testing.T) {

	// Simple types
	registerTypeFnHelper(t, true)
	registerTypeFnHelper(t, int(123))
	registerTypeFnHelper(t, int8(123))
	registerTypeFnHelper(t, int16(123))
	registerTypeFnHelper(t, int32(123))
	registerTypeFnHelper(t, int64(123))
	registerTypeFnHelper(t, uint(123))
	registerTypeFnHelper(t, uint8(123))
	registerTypeFnHelper(t, uint16(123))
	registerTypeFnHelper(t, uint32(123))
	registerTypeFnHelper(t, uint64(123))
	registerTypeFnHelper(t, uintptr(123))
	registerTypeFnHelper(t, float32(123.456))
	registerTypeFnHelper(t, float64(123.456))
	registerTypeFnHelper(t, complex64(123.45+678.9i))
	registerTypeFnHelper(t, complex128(123.45+678.9i))
	registerTypeFnHelper(t, "abc")
	registerTypeFnHelper(t, unsafe.Pointer(ptr(123)))

	// Variadic types
	registerTypeFnHelper(t, ptr(123))
	registerTypeFnHelper(t, map[string]int{"abc": 123})
	registerTypeFnHelper(t, []int{1, 2, 3})
	registerTypeFnHelper(t, [...]int{1, 2, 3})
	registerTypeFnHelper(t, make(chan int))
	registerTypeFnHelper(t, (func(int) error)(nil))
	registerTypeFnHelper(t, struct{ a int }{a: 123})

	// Context types
	registerCtxTypeFnHelper[struct{}](t)
	registerCtxTypeFnHelper[any](t)
	registerCtxTypeFnHelper[error](t)
	registerCtxTypeFnHelper[interface{ Func() }](t)
}

func registerTypeFnHelper[V any](t *testing.T, v V) {
	t.Run(fmt.Sprintf("%T", v), func(t *testing.T) {
		var ctx []V
		register := tw.NewRegister[*[]V]()
		tw.RegisterTypeFn(register, func(ctx *[]V, val tw.Arg[V]) error {
			*ctx = append(*ctx, val.Get())
			return nil
		})
		walker := tw.NewWalker[*[]V](register)

		err := walker.Walk(&ctx, v)
		require.NoError(t, err)
		assert.Equal(t, []V{v}, ctx)

		typeWalker, err := tw.NewTypeWalker[*[]V, V](walker)
		require.NoError(t, err)
		err = typeWalker.Walk(&ctx, &v)
		require.NoError(t, err)
		assert.Equal(t, []V{v, v}, ctx)
	})
}

func registerCtxTypeFnHelper[Ctx any](t *testing.T) {
	t.Run(fmt.Sprintf("Context(%s)", reflect.TypeOf((*Ctx)(nil)).Elem().String()), func(t *testing.T) {
		register := tw.NewRegister[Ctx]()
		tw.RegisterTypeFn(register, func(ctx Ctx, val tw.Arg[struct{}]) error { return nil })
	})
}

func TestRegisterCompileTypeFn(t *testing.T) {
	registerCompileTypeFnHelper(t, (*tw.Register[*[]bool]).RegisterCompileBoolFn, true)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]int]).RegisterCompileIntFn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]int8]).RegisterCompileInt8Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]int16]).RegisterCompileInt16Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]int32]).RegisterCompileInt32Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]int64]).RegisterCompileInt64Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]uint]).RegisterCompileUintFn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]uint8]).RegisterCompileUint8Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]uint16]).RegisterCompileUint16Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]uint32]).RegisterCompileUint32Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]uint64]).RegisterCompileUint64Fn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]uintptr]).RegisterCompileUintptrFn, 123)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]float32]).RegisterCompileFloat32Fn, 123.456)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]float64]).RegisterCompileFloat64Fn, 123.456)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]complex64]).RegisterCompileComplex64Fn, 123.45+678.9i)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]complex128]).RegisterCompileComplex128Fn, 123.45+678.9i)
	registerCompileTypeFnHelper(t, (*tw.Register[*[]string]).RegisterCompileStringFn, "abc")
	registerCompileTypeFnHelper(t, (*tw.Register[*[]unsafe.Pointer]).RegisterCompileUnsafePointerFn, unsafe.Pointer(ptr(123)))
}

type registerCompileTypeFn[T any] func(*tw.Register[*[]T], tw.CompileFn[*[]T, T])

func registerCompileTypeFnHelper[V any](t *testing.T, registerFn registerCompileTypeFn[V], v V) {
	t.Run(fmt.Sprintf("%T", v), func(t *testing.T) {
		var ctx []V
		register := tw.NewRegister[*[]V]()
		registerFn(register, func(t reflect.Type) tw.WalkFn[*[]V, V] {
			return func(ctx *[]V, val tw.Arg[V]) error {
				*ctx = append(*ctx, val.Get())
				return nil
			}
		})

		walker := tw.NewWalker[*[]V](register)
		err := walker.Walk(&ctx, v)
		require.NoError(t, err)
		assert.Equal(t, []V{v}, ctx)

		typeWalker, err := tw.NewTypeWalker[*[]V, V](walker)
		require.NoError(t, err)
		err = typeWalker.Walk(&ctx, &v)
		require.NoError(t, err)
		assert.Equal(t, []V{v, v}, ctx)
	})
}

func TestRegisterCompileStructFn(t *testing.T) {

	type B struct {
		C string
	}

	type D struct {
		E int
		F string
	}

	type S struct {
		A int
		B B
		D
	}

	register := tw.NewRegister[*strings.Builder]()
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, i tw.Int) error {
		_, err := fmt.Fprint(ctx, i.Get())
		return err
	})
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, s tw.String) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, s.Get())
		return err
	})

	register.RegisterCompileStructFn(func(typ reflect.Type, sfw tw.StructFieldRegister) (tw.StructWalkFn[*strings.Builder], error) {
		// fields := reflect.VisibleFields(typ)
		fields := make([]reflect.StructField, typ.NumField())
		for i := range fields {
			fields[i] = typ.Field(i)
			sfw.RegisterField(i)
		}
		return func(ctx *strings.Builder, sw tw.Struct[*strings.Builder]) error {
			ctx.WriteRune('{')
			for i, field := range fields {
				if i > 0 {
					ctx.WriteRune(',')
				}
				_, err := fmt.Fprintf(ctx, `"%s":`, field.Name)
				if err != nil {
					return err
				}

				err = sw.Walk(ctx, i)
				if err != nil {
					return err
				}
			}
			ctx.WriteRune('}')
			return nil
		}, nil
	})

	walker := tw.NewWalker[*strings.Builder](register)
	typeWalker, err := tw.NewTypeWalker[*strings.Builder, S](walker)
	require.NoError(t, err)

	{
		var sb strings.Builder
		err := walker.Walk(&sb, S{A: 123, B: B{C: "abc"}, D: D{E: 456, F: "def"}})
		require.NoError(t, err)
		assert.Equal(t, `{"A":123,"B":{"C":"abc"},"D":{"E":456,"F":"def"}}`, sb.String())
	}
	{
		var sb strings.Builder
		err := typeWalker.Walk(&sb, &S{A: 123, B: B{C: "abc"}, D: D{E: 456, F: "def"}})
		require.NoError(t, err)
		assert.Equal(t, `{"A":123,"B":{"C":"abc"},"D":{"E":456,"F":"def"}}`, sb.String())
	}
}

func TestRegisterCompileArrayFn(t *testing.T) {

	register := tw.NewRegister[*strings.Builder]()
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, s tw.String) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, s.Get())
		return err
	})

	register.RegisterCompileArrayFn(func(typ reflect.Type) (tw.ArrayWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, aw tw.Array[*strings.Builder]) error {
			ctx.WriteRune('[')
			for i := range aw.Len() {
				if i > 0 {
					ctx.WriteRune(',')
				}
				err := aw.Walk(ctx, i)
				if err != nil {
					return err
				}
			}
			ctx.WriteRune(']')
			return nil
		}, nil
	})

	walker := tw.NewWalker[*strings.Builder](register)
	typeWalker, err := tw.NewTypeWalker[*strings.Builder, [3]string](walker)
	require.NoError(t, err)

	{
		var sb strings.Builder
		err = walker.Walk(&sb, [3]string{"abc", "def", "ghi"})
		require.NoError(t, err)
		assert.Equal(t, `["abc","def","ghi"]`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, &[3]string{"abc", "def", "ghi"})
		require.NoError(t, err)
		assert.Equal(t, `["abc","def","ghi"]`, sb.String())
	}
}

func TestRegisterCompileSliceFn(t *testing.T) {

	register := tw.NewRegister[*strings.Builder]()
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, s tw.String) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, s.Get())
		return err
	})

	register.RegisterCompileSliceFn(func(typ reflect.Type) (tw.SliceWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, sw tw.Slice[*strings.Builder]) error {
			if sw.IsNil() {
				ctx.WriteString("null")
				return nil
			}
			ctx.WriteRune('[')
			for i := range sw.Len() {
				if i > 0 {
					ctx.WriteRune(',')
				}
				err := sw.Walk(ctx, i)
				if err != nil {
					return err
				}
			}
			ctx.WriteRune(']')
			return nil
		}, nil
	})

	walker := tw.NewWalker[*strings.Builder](register)
	typeWalker, err := tw.NewTypeWalker[*strings.Builder, []string](walker)
	require.NoError(t, err)

	{
		var sb strings.Builder
		err = walker.Walk(&sb, []string{"abc", "def", "ghi"})
		require.NoError(t, err)
		assert.Equal(t, `["abc","def","ghi"]`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, &[]string{"abc", "def", "ghi"})
		require.NoError(t, err)
		assert.Equal(t, `["abc","def","ghi"]`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, []string(nil))
		require.NoError(t, err)
		assert.Equal(t, `null`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, ptr([]string(nil)))
		require.NoError(t, err)
		assert.Equal(t, `null`, sb.String())
	}
}
func TestRegisterCompilePtrFn(t *testing.T) {

	register := tw.NewRegister[*strings.Builder]()
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, s tw.String) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, s.Get())
		return err
	})

	register.RegisterCompilePtrFn(func(typ reflect.Type) (tw.PtrWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, pw tw.Ptr[*strings.Builder]) error {
			if pw.IsNil() {
				ctx.WriteString("null")
				return nil
			}
			ctx.WriteString("ptr(")
			err := pw.Walk(ctx)
			if err != nil {
				return err
			}
			ctx.WriteRune(')')
			return nil
		}, nil
	})

	var err error
	walker := tw.NewWalker[*strings.Builder](register)
	{
		var sb strings.Builder
		err = walker.Walk(&sb, (*string)(nil))
		require.NoError(t, err)
		assert.Equal(t, `null`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, (**string)(nil))
		require.NoError(t, err)
		assert.Equal(t, `null`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, ptr("abc"))
		require.NoError(t, err)
		assert.Equal(t, `ptr("abc")`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, ptr[*string](nil))
		require.NoError(t, err)
		assert.Equal(t, `ptr(null)`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, ptr(ptr("abc")))
		require.NoError(t, err)
		require.Equal(t, `ptr(ptr("abc"))`, sb.String())
	}

	{
		typeWalker, err := tw.NewTypeWalker[*strings.Builder, *string](walker)
		require.NoError(t, err)
		{
			var sb strings.Builder
			err = typeWalker.Walk(&sb, ptr((*string)(nil)))
			require.NoError(t, err)
			assert.Equal(t, `null`, sb.String())
		}
		{
			var sb strings.Builder
			err = typeWalker.Walk(&sb, ptr(ptr("abc")))
			require.NoError(t, err)
			assert.Equal(t, `ptr("abc")`, sb.String())
		}
	}
	{
		typeWalker, err := tw.NewTypeWalker[*strings.Builder, **string](walker)
		require.NoError(t, err)
		{
			var sb strings.Builder
			err = typeWalker.Walk(&sb, ptr((**string)(nil)))
			require.NoError(t, err)
			assert.Equal(t, `null`, sb.String())
		}
		{
			var sb strings.Builder
			err = typeWalker.Walk(&sb, ptr(ptr[*string](nil)))
			require.NoError(t, err)
			assert.Equal(t, `ptr(null)`, sb.String())
		}
		{
			var sb strings.Builder
			err = typeWalker.Walk(&sb, ptr(ptr(ptr("abc"))))
			require.NoError(t, err)
			require.Equal(t, `ptr(ptr("abc"))`, sb.String())
		}
	}
}

func TestRegisterCompileMapFn(t *testing.T) {

	register := tw.NewRegister[*strings.Builder]()
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, s tw.String) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, s.Get())
		return err
	})

	tw.RegisterTypeFn(register, func(ctx *strings.Builder, i tw.Int) error {
		_, err := fmt.Fprintf(ctx, `%d`, i.Get())
		return err
	})

	register.RegisterCompileMapFn(func(typ reflect.Type) (tw.MapWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, m tw.Map[*strings.Builder]) error {
			if m.IsNil() {
				ctx.WriteString("null")
				return nil
			}
			ctx.WriteRune('{')
			iter := m.Iter()
			i := 0
			for iter.Next() {
				if i > 0 {
					ctx.WriteRune(',')
				}
				e := iter.Entry()
				err := e.WalkKey(ctx)
				if err != nil {
					return err
				}
				ctx.WriteRune(':')
				err = e.WalkValue(ctx)
				if err != nil {
					return err
				}
				i++
			}
			ctx.WriteRune('}')
			return nil
		}, nil
	})

	walker := tw.NewWalker[*strings.Builder](register)
	typeWalker, err := tw.NewTypeWalker[*strings.Builder, map[string]int](walker)
	require.NoError(t, err)

	{
		var sb strings.Builder
		err = walker.Walk(&sb, (map[string]int)(nil))
		require.NoError(t, err)
		assert.Equal(t, `null`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, map[string]int{})
		require.NoError(t, err)
		assert.Equal(t, `{}`, sb.String())
	}
	{
		var sb strings.Builder
		err = walker.Walk(&sb, map[string]int{"abc": 123, "def": 456})
		require.NoError(t, err)
		// Use JSONEq because map iteration order is not guaranteed.
		assert.JSONEq(t, `{"abc":123,"def":456}`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, ptr[map[string]int](nil))
		require.NoError(t, err)
		assert.Equal(t, `null`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, &map[string]int{})
		require.NoError(t, err)
		assert.Equal(t, `{}`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, &map[string]int{"abc": 123, "def": 456})
		require.NoError(t, err)
		// Use JSONEq because map iteration order is not guaranteed.
		assert.JSONEq(t, `{"abc":123,"def":456}`, sb.String())
	}
}

func TestCompileRecursive(t *testing.T) {
	type Node struct {
		val  int
		next *Node
	}
	register := tw.NewRegister[*strings.Builder]()
	tw.RegisterTypeFn(register, func(ctx *strings.Builder, i tw.Int) error {
		_, err := fmt.Fprintf(ctx, `%d`, i.Get())
		return err
	})

	register.RegisterCompileStructFn(func(typ reflect.Type, s tw.StructFieldRegister) (tw.StructWalkFn[*strings.Builder], error) {
		fields := make([]reflect.StructField, typ.NumField())
		for i := range fields {
			fields[i] = typ.Field(i)
			s.RegisterField(i)
		}
		return func(ctx *strings.Builder, s tw.Struct[*strings.Builder]) error {
			ctx.WriteRune('{')
			for i, f := range fields {
				if i > 0 {
					ctx.WriteRune(',')
				}
				ctx.WriteString(f.Name)
				ctx.WriteRune(':')
				if err := s.Walk(ctx, i); err != nil {
					return err
				}
			}
			ctx.WriteRune('}')
			return nil
		}, nil
	})

	register.RegisterCompilePtrFn(func(typ reflect.Type) (tw.PtrWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, p tw.Ptr[*strings.Builder]) error {
			if p.IsNil() {
				ctx.WriteString("nil")
				return nil
			}
			return p.Walk(ctx)
		}, nil
	})

	walker := tw.NewWalker[*strings.Builder](register)
	typeWalker, err := tw.NewTypeWalker[*strings.Builder, Node](walker)
	require.NoError(t, err)

	{
		var sb strings.Builder
		err = walker.Walk(&sb, &Node{val: 1, next: &Node{val: 2}})
		require.NoError(t, err)
		assert.Equal(t, `{val:1,next:{val:2,next:nil}}`, sb.String())
	}
	{
		var sb strings.Builder
		err = typeWalker.Walk(&sb, &Node{val: 1, next: &Node{val: 2}})
		require.NoError(t, err)
		assert.Equal(t, `{val:1,next:{val:2,next:nil}}`, sb.String())
	}
}

func TestSettable(t *testing.T) {
	settableHelper(t, true, false)
	settableHelper(t, int(123), int(-123))
	settableHelper(t, int8(123), int8(-123))
	settableHelper(t, int16(123), int16(-123))
	settableHelper(t, int32(123), int32(-123))
	settableHelper(t, int64(123), int64(-123))
	settableHelper(t, uint(123), uint(234))
	settableHelper(t, uint8(123), uint8(234))
	settableHelper(t, uint16(123), uint16(234))
	settableHelper(t, uint32(123), uint32(234))
	settableHelper(t, uint64(123), uint64(234))
	settableHelper(t, uintptr(123), uintptr(234))
	settableHelper(t, float32(123.456), float32(-123.456))
	settableHelper(t, float64(123.456), float64(-123.456))
	settableHelper(t, complex64(123.45+678.9i), complex64(-123.45-678.9i))
	settableHelper(t, complex128(123.45+678.9i), complex128(-123.45-678.9i))
	settableHelper(t, "abc", "def")
	settableHelper(t, unsafe.Pointer(ptr(123)), unsafe.Pointer(ptr(234)))

	// Variadic types
	settableHelper(t, ptr(123), ptr(-123))
	settableHelper(t, map[string]int{"abc": 123}, map[string]int{"def": -123})
	settableHelper(t, []int{1, 2, 3}, []int{2, 3, 4})
	settableHelper(t, [...]int{1, 2, 3}, [...]int{2, 3, 4})
	settableHelper(t, make(chan int), make(chan int))
	settableHelper(t, struct{ a int }{a: 123}, struct{ a int }{a: -123})
	// N.b. cannot do functions, because only nil functions are comparable.

	settableSliceHelper(t)
	settableArrayHelper(t)
	settableStructHelper(t)
	settablePtrHelper(t)
	settableMapHelper(t)
}

func settableHelper[V any](t *testing.T, v V, newV V) {
	t.Run(fmt.Sprintf("%T", v), func(t *testing.T) {
		t.Run("fromInterface", func(t *testing.T) {
			register := tw.NewRegister[struct{}]()
			var savedVal tw.Arg[V]
			tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[V]) error {
				savedVal = val
				return nil
			})
			walker := tw.NewWalker[struct{}](register)
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			assert.False(t, savedVal.CanSet())
		})
		t.Run("fromPointer", func(t *testing.T) {
			oldV := v
			register := tw.NewRegister[struct{}]()
			var savedVal tw.Arg[V]
			tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[V]) error {
				savedVal = val
				if val.CanSet() {
					val.Set(newV)
				}
				return nil
			})
			walker := tw.NewWalker[struct{}](register)
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			assert.False(t, savedVal.CanSet())
			typeWalker, err := tw.NewTypeWalker[struct{}, V](walker)
			require.NoError(t, err)
			err = typeWalker.Walk(struct{}{}, &v)
			require.NoError(t, err)
			assert.True(t, savedVal.CanSet())
			assert.Equal(t, newV, v)

			savedVal.Set(oldV)
			assert.Equal(t, oldV, v)
		})
	})
}

func settableSliceHelper(t *testing.T) {
	t.Run("slice", func(t *testing.T) {
		register := tw.NewRegister[struct{}]()
		var savedChildren []tw.Arg[int]
		tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[int]) error {
			savedChildren = append(savedChildren, val)
			return nil
		})
		register.RegisterCompileSliceFn(func(typ reflect.Type) (tw.SliceWalkFn[struct{}], error) {
			return func(ctx struct{}, s tw.Slice[struct{}]) error {
				for i := range s.Len() {
					err := s.Walk(ctx, i)
					if err != nil {
						return err
					}
				}
				return nil
			}, nil
		})
		walker := tw.NewWalker[struct{}](register)
		typeWalker, err := tw.NewTypeWalker[struct{}, []int](walker)
		require.NoError(t, err)
		t.Run("fromInterface", func(t *testing.T) {
			savedChildren = nil
			v := []int{1, 2, 3}
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			var oldV int
			for i, sv := range savedChildren {
				assert.True(t, sv.CanSet())
				oldV = sv.Get()
				sv.Set(oldV + 1)
				assert.Equal(t, oldV+1, v[i])
			}
		})
		t.Run("fromPointer", func(t *testing.T) {
			savedChildren = nil
			v := []int{1, 2, 3}
			err = typeWalker.Walk(struct{}{}, &v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 3)
			var oldV int
			for i, sv := range savedChildren {
				assert.True(t, sv.CanSet())
				oldV = sv.Get()
				sv.Set(oldV + 1)
				assert.Equal(t, oldV+1, v[i])
			}
		})
	})
}

func settableArrayHelper(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		register := tw.NewRegister[struct{}]()
		var savedChildren []tw.Arg[int]
		tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[int]) error {
			savedChildren = append(savedChildren, val)
			return nil
		})
		register.RegisterCompileArrayFn(func(typ reflect.Type) (tw.ArrayWalkFn[struct{}], error) {
			return func(ctx struct{}, a tw.Array[struct{}]) error {
				for i := range a.Len() {
					err := a.Walk(ctx, i)
					if err != nil {
						return err
					}
				}
				return nil
			}, nil
		})
		walker := tw.NewWalker[struct{}](register)
		typeWalker, err := tw.NewTypeWalker[struct{}, [3]int](walker)
		require.NoError(t, err)
		t.Run("fromInterface", func(t *testing.T) {
			savedChildren = nil
			v := [...]int{1, 2, 3}
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			for _, sv := range savedChildren {
				assert.False(t, sv.CanSet())
			}
		})
		t.Run("fromPointer", func(t *testing.T) {
			savedChildren = nil
			v := [...]int{1, 2, 3}
			err = typeWalker.Walk(struct{}{}, &v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 3)
			var oldV int
			for i, sv := range savedChildren {
				assert.True(t, sv.CanSet())
				oldV = sv.Get()
				sv.Set(oldV + 1)
				assert.Equal(t, oldV+1, v[i])
			}
		})
	})
}

func settableStructHelper(t *testing.T) {
	type ABC struct {
		A int
		B int
		C int
	}
	t.Run("struct", func(t *testing.T) {
		register := tw.NewRegister[struct{}]()
		var savedChildren []tw.Arg[int]
		tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[int]) error {
			savedChildren = append(savedChildren, val)
			return nil
		})
		register.RegisterCompileStructFn(func(typ reflect.Type, register tw.StructFieldRegister) (tw.StructWalkFn[struct{}], error) {
			for i := range typ.NumField() {
				register.RegisterField(i)
			}
			return func(ctx struct{}, s tw.Struct[struct{}]) error {
				for i := range s.NumFields() {
					err := s.Walk(ctx, i)
					if err != nil {
						return err
					}
				}
				return nil
			}, nil
		})
		walker := tw.NewWalker[struct{}](register)
		typeWalker, err := tw.NewTypeWalker[struct{}, ABC](walker)
		require.NoError(t, err)
		t.Run("fromInterface", func(t *testing.T) {
			savedChildren = nil
			v := ABC{A: 1, B: 2, C: 3}
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 3)
			for _, sv := range savedChildren {
				assert.False(t, sv.CanSet())
			}
		})
		t.Run("fromPointer", func(t *testing.T) {
			savedChildren = nil
			v := ABC{A: 1, B: 2, C: 3}
			err = typeWalker.Walk(struct{}{}, &v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 3)
			for _, sv := range savedChildren {
				assert.True(t, sv.CanSet())
			}
			var oldV int
			for i, sv := range savedChildren {
				assert.True(t, sv.CanSet())
				oldV = sv.Get()
				sv.Set(oldV + 1)
				var newV int
				switch i {
				case 0:
					newV = v.A
				case 1:
					newV = v.B
				case 2:
					newV = v.C
				}
				assert.Equal(t, oldV+1, newV)
			}
		})
	})
}

func settablePtrHelper(t *testing.T) {
	t.Run("ptr", func(t *testing.T) {
		register := tw.NewRegister[struct{}]()
		var savedChildren []tw.Arg[int]
		tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[int]) error {
			savedChildren = append(savedChildren, val)
			return nil
		})
		register.RegisterCompilePtrFn(func(typ reflect.Type) (tw.PtrWalkFn[struct{}], error) {
			return func(ctx struct{}, p tw.Ptr[struct{}]) error {
				return p.Walk(ctx)
			}, nil
		})
		walker := tw.NewWalker[struct{}](register)
		typeWalker, err := tw.NewTypeWalker[struct{}, *int](walker)
		require.NoError(t, err)
		t.Run("fromInterface", func(t *testing.T) {
			savedChildren = nil
			v := ptr(1)
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			var oldV int
			for _, sv := range savedChildren {
				assert.True(t, sv.CanSet())
				oldV = sv.Get()
				sv.Set(oldV + 1)
				assert.Equal(t, oldV+1, *v)
			}
		})
		t.Run("fromPointer", func(t *testing.T) {
			savedChildren = nil
			v := ptr(1)
			err = typeWalker.Walk(struct{}{}, &v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 1)
			var oldV int
			for _, sv := range savedChildren {
				assert.True(t, sv.CanSet())
				oldV = sv.Get()
				sv.Set(oldV + 1)
				assert.Equal(t, oldV+1, *v)
			}
		})
	})
}

func settableMapHelper(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		register := tw.NewRegister[struct{}]()
		var savedChildren []tw.Arg[int]
		tw.RegisterTypeFn(register, func(ctx struct{}, val tw.Arg[int]) error {
			savedChildren = append(savedChildren, val)
			return nil
		})
		register.RegisterCompileMapFn(func(typ reflect.Type) (tw.MapWalkFn[struct{}], error) {
			return func(ctx struct{}, m tw.Map[struct{}]) error {
				iter := m.Iter()
				for iter.Next() {
					entry := iter.Entry()
					err := entry.WalkKey(ctx)
					if err != nil {
						return err
					}
					err = entry.WalkValue(ctx)
					if err != nil {
						return err
					}
				}
				return nil
			}, nil
		})
		walker := tw.NewWalker[struct{}](register)
		typeWalker, err := tw.NewTypeWalker[struct{}, map[int]int](walker)
		require.NoError(t, err)
		t.Run("fromInterface", func(t *testing.T) {
			savedChildren = nil
			v := map[int]int{1: 2, 3: 4}
			err := walker.Walk(struct{}{}, v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 4)
			for _, sv := range savedChildren {
				assert.False(t, sv.CanSet())
			}
		})
		t.Run("fromPointer", func(t *testing.T) {
			savedChildren = nil
			v := map[int]int{1: 2, 3: 4}
			err = typeWalker.Walk(struct{}{}, &v)
			require.NoError(t, err)
			require.Len(t, savedChildren, 4)
			for _, sv := range savedChildren {
				assert.False(t, sv.CanSet())
			}
		})
	})
}
