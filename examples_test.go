package type_walk_test

import (
	"fmt"
	tw "github.com/zolstein/type-walk"
	"reflect"
	"strings"
)

func Example_prettyPrint() {

	// Ctx tracks the buffer that we're writing into, as well as the current indentation level.
	// Writing errors can mostly be ignored because writing to strings.Builder cannot fail.
	type Ctx struct {
		Buffer *strings.Builder
		Indent int
	}

	// Write N spaces to the buffer to indent correctly.
	indent := func(ctx Ctx) {
		for i := 0; i < ctx.Indent; i++ {
			ctx.Buffer.WriteRune(' ')
		}
	}

	register := tw.NewRegister[Ctx]()

	// For simple types, just serialize into the buffer.
	tw.RegisterCompileBoolFn(register, func(typ reflect.Type) tw.WalkFn[Ctx, bool] {
		return func(ctx Ctx, v tw.Bool) error {
			_, err := fmt.Fprintf(ctx.Buffer, "%t", v.Get())
			return err
		}
	})
	tw.RegisterCompileIntFn(register, func(typ reflect.Type) tw.WalkFn[Ctx, int] {
		return func(ctx Ctx, v tw.Int) error {
			_, err := fmt.Fprintf(ctx.Buffer, "%d", v.Get())
			return err
		}
	})
	tw.RegisterCompileStringFn(register, func(typ reflect.Type) tw.WalkFn[Ctx, string] {
		return func(ctx Ctx, v tw.String) error {
			_, err := fmt.Fprintf(ctx.Buffer, `"%s"`, v.Get())
			return err
		}
	})

	// For pointers, handle nil, otherwise handle recursively.
	tw.RegisterCompilePtrFn(register, func(typ reflect.Type) tw.WalkPtrFn[Ctx] {
		return func(ctx Ctx, p tw.Ptr[Ctx]) error {
			if p.IsNil() {
				_, _ = ctx.Buffer.WriteString("nil")
				return nil
			}
			return p.Walk(ctx)
		}
	})

	// For slices, handle each element, setting up the correct indentation.
	tw.RegisterCompileSliceFn(register, func(typ reflect.Type) tw.WalkSliceFn[Ctx] {
		return func(ctx Ctx, p tw.Slice[Ctx]) error {
			if p.IsNil() {
				_, err := fmt.Fprintf(ctx.Buffer, "nil")
				return err
			}
			ctx.Buffer.WriteString("[\n")
			ctx.Indent += 2
			for i := 0; i < p.Len(); i++ {
				indent(ctx)
				err := p.Elem(i).Walk(ctx)
				if err != nil {
					return err
				}
				ctx.Buffer.WriteString("\n")
			}
			ctx.Indent -= 2
			indent(ctx)
			ctx.Buffer.WriteString("]")
			return nil
		}
	})

	// For structs, when compiling, register the fields and track the field names.
	// When walking, handle each element, printing the correct name and setting up the correct indentation.
	tw.RegisterCompileStructFn(register, func(typ reflect.Type, r tw.StructFieldRegister) tw.WalkStructFn[Ctx] {
		fieldNames := make([]string, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			idx := r.RegisterField(i)
			fieldNames[idx] = typ.Field(i).Name
		}
		return func(ctx Ctx, s tw.Struct[Ctx]) error {
			ctx.Buffer.WriteString("{\n")
			ctx.Indent += 2
			for i := 0; i < s.NumFields(); i++ {
				indent(ctx)
				ctx.Buffer.WriteString(fieldNames[i])
				ctx.Buffer.WriteString(": ")
				err := s.Field(i).Walk(ctx)
				if err != nil {
					return err
				}
				ctx.Buffer.WriteString("\n")
			}
			ctx.Indent -= 2
			indent(ctx)
			ctx.Buffer.WriteString("}")
			return nil
		}
	})

	walker := tw.NewWalker[Ctx](register)

	type Inner struct {
		A *int
		B bool
		C []string
	}
	type Outer struct {
		S []*Inner
	}

	ctx := Ctx{Buffer: &strings.Builder{}}
	err := walker.Walk(ctx, Outer{
		S: []*Inner{
			{A: ptr(1), B: true, C: []string{"foo", "bar", "baz"}},
			nil,
			{A: nil, B: false, C: nil},
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(ctx.Buffer.String())

	// Output:
	// {
	//   S: [
	//     {
	//       A: 1
	//       B: true
	//       C: [
	//         "foo"
	//         "bar"
	//         "baz"
	//       ]
	//     }
	//     nil
	//     {
	//       A: nil
	//       B: false
	//       C: nil
	//     }
	//   ]
	// }
}
