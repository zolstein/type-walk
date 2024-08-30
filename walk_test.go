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
}

func registerTypeFnHelper[V any](t *testing.T, v V) {
	t.Run(fmt.Sprintf("%T", v), func(t *testing.T) {
		var ctx []V
		register := tw.NewRegister[*[]V]()
		err := register.RegisterTypeFn(func(ctx *[]V, val *V) error {
			*ctx = append(*ctx, *val)
			return nil
		})
		require.NoError(t, err)
		walker := tw.NewWalker[*[]V](register)

		err = walker.Walk(&ctx, v)
		require.NoError(t, err)
		assert.Equal(t, []V{v}, ctx)

		typeWalker, err := tw.NewTypeWalker[*[]V, V](walker)
		require.NoError(t, err)
		err = typeWalker.Walk(&ctx, &v)
		require.NoError(t, err)
		assert.Equal(t, []V{v, v}, ctx)
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
			return func(ctx *[]V, i *V) error {
				*ctx = append(*ctx, *i)
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
	var err error
	err = register.RegisterTypeFn(func(ctx *strings.Builder, i *int) error {
		_, err := fmt.Fprint(ctx, *i)
		return err
	})
	require.NoError(t, err)
	err = register.RegisterTypeFn(func(ctx *strings.Builder, s *string) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, *s)
		return err
	})
	require.NoError(t, err)

	register.RegisterCompileStructFn(func(typ reflect.Type, sfw tw.StructFieldRegister[*strings.Builder]) (tw.StructWalkFn[*strings.Builder], error) {
		// fields := reflect.VisibleFields(typ)
		fields := make([]reflect.StructField, typ.NumField())
		for i := range fields {
			fields[i] = typ.Field(i)
			_, err := sfw.RegisterField(i)
			if err != nil {
				return nil, err
			}
		}
		return func(ctx *strings.Builder, sw tw.StructFieldWalker[*strings.Builder]) error {
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
	var err error
	err = register.RegisterTypeFn(func(ctx *strings.Builder, s *string) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, *s)
		return err
	})
	require.NoError(t, err)

	register.RegisterCompileArrayFn(func(typ reflect.Type) (tw.ArrayWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, aw tw.ArrayWalker[*strings.Builder]) error {
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
	var err error
	err = register.RegisterTypeFn(func(ctx *strings.Builder, s *string) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, *s)
		return err
	})
	require.NoError(t, err)

	register.RegisterCompileSliceFn(func(typ reflect.Type) (tw.SliceWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, sw tw.SliceWalker[*strings.Builder]) error {
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
	var err error
	err = register.RegisterTypeFn(func(ctx *strings.Builder, s *string) error {
		_, err := fmt.Fprintf(ctx, `"%s"`, *s)
		return err
	})
	require.NoError(t, err)

	register.RegisterCompilePtrFn(func(typ reflect.Type) (tw.PtrWalkFn[*strings.Builder], error) {
		return func(ctx *strings.Builder, pw tw.PtrWalker[*strings.Builder]) error {
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

func TestCompileRecursive(t *testing.T) {
	type Node struct {
		val  int
		next *Node
	}
	register := tw.NewRegister[*strings.Builder]()
	var err error
	err = register.RegisterTypeFn(func(ctx *strings.Builder, s *int) error {
		_, err := fmt.Fprintf(ctx, `%d`, *s)
		return err
	})
	require.NoError(t, err)

	register.RegisterCompileStructFn(func(typ reflect.Type, s tw.StructFieldRegister[*strings.Builder]) (tw.StructWalkFn[*strings.Builder], error) {
		fields := make([]reflect.StructField, typ.NumField())
		for i := range fields {
			fields[i] = typ.Field(i)
			_, err := s.RegisterField(i)
			if err != nil {
				return nil, err
			}
		}
		return func(ctx *strings.Builder, s tw.StructFieldWalker[*strings.Builder]) error {
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
		return func(ctx *strings.Builder, p tw.PtrWalker[*strings.Builder]) error {
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
