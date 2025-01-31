package type_walk_test

import (
	"bytes"
	"github.com/stretchr/testify/require"
	tw "github.com/zolstein/type-walk"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

func BenchmarkSimpleJsonSerialize(b *testing.B) {

	type Inner struct {
		D int
		E string
	}

	type Outer struct {
		A string
		B int
		C []*Inner
	}

	toSerialize := make([]*Outer, 100)
	for i := range toSerialize {
		inners := make([]*Inner, rand.Intn(10))
		for j := range inners {
			inners[j] = &Inner{
				D: rand.Int(),
				E: randString(10),
			}
		}
		toSerialize[i] = &Outer{
			A: randString(20),
			B: rand.Int(),
			C: inners,
		}
	}

	var bb bytes.Buffer
	var intBuf []byte

	var serializeReflect func(v reflect.Value)
	serializeReflect = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Int:
			intBuf = intBuf[:0]
			intBuf = strconv.AppendInt(intBuf, v.Int(), 10)
			bb.Write(intBuf)
		case reflect.String:
			bb.WriteRune('"')
			bb.WriteString(v.String())
			bb.WriteRune('"')
		case reflect.Ptr:
			if v.IsNil() {
				bb.WriteString("null")
			} else {
				serializeReflect(v.Elem())
			}
		case reflect.Slice:
			bb.WriteRune('[')
			for i := 0; i < v.Len(); i++ {
				if i > 0 {
					bb.WriteRune(',')
				}
				serializeReflect(v.Index(i))
			}
			bb.WriteRune(']')
		case reflect.Struct:
			bb.WriteRune('{')
			t := v.Type()
			for i := 0; i < t.NumField(); i++ {
				if i > 0 {
					bb.WriteRune(',')
				}
				sf := t.Field(i)
				serializeReflect(reflect.ValueOf(sf.Name))
				bb.WriteRune(':')
				serializeReflect(v.Field(i))
			}
			bb.WriteRune('}')
		default:
			panic("unhandled default case")
		}
	}

	serializeReflect(reflect.ValueOf(toSerialize))

	b.Run("reflect", func(b *testing.B) {
		runtime.GC()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bb.Reset()
			serializeReflect(reflect.ValueOf(toSerialize))
			bs := bb.Bytes()
			_ = bs
		}
	})

	register := tw.NewRegister[*bytes.Buffer]()
	tw.RegisterCompileIntFn(register, func(r reflect.Type) tw.WalkFn[*bytes.Buffer, int] {
		return func(ctx *bytes.Buffer, i tw.Int) error {
			intBuf = intBuf[:0]
			intBuf = strconv.AppendInt(intBuf, int64(i.Get()), 10)
			bb.Write(intBuf)
			return nil
		}
	})
	tw.RegisterCompileStringFn(register, func(r reflect.Type) tw.WalkFn[*bytes.Buffer, string] {
		return func(ctx *bytes.Buffer, s tw.String) error {
			bb.WriteRune('"')
			bb.WriteString(s.Get())
			bb.WriteRune('"')
			return nil
		}
	})
	tw.RegisterCompileSliceFn(register, func(r reflect.Type) tw.WalkSliceFn[*bytes.Buffer] {
		return func(ctx *bytes.Buffer, s tw.Slice[*bytes.Buffer]) error {
			bb.WriteRune('[')
			for i := 0; i < s.Len(); i++ {
				if i > 0 {
					bb.WriteRune(',')
				}
				err := s.Elem(i).Walk(ctx)
				if err != nil {
					return err
				}
			}
			bb.WriteRune(']')
			return nil
		}
	})
	tw.RegisterCompileStructFn(register, func(r reflect.Type, sfr tw.StructFieldRegister) tw.WalkStructFn[*bytes.Buffer] {
		fields := make([]reflect.StructField, r.NumField())
		for i := range fields {
			idx := sfr.RegisterField(i)
			fields[idx] = r.Field(i)
		}
		return func(ctx *bytes.Buffer, s tw.Struct[*bytes.Buffer]) error {
			bb.WriteRune('{')
			for i := 0; i < s.NumFields(); i++ {
				if i > 0 {
					bb.WriteRune(',')
				}
				bb.WriteRune('"')
				bb.WriteString(fields[i].Name)
				bb.WriteRune('"')
				bb.WriteRune(':')
				err := s.Field(i).Walk(ctx)
				if err != nil {
					return err
				}
			}
			bb.WriteRune('}')
			return nil
		}
	})
	tw.RegisterCompilePtrFn(register, func(r reflect.Type) tw.WalkPtrFn[*bytes.Buffer] {
		return func(ctx *bytes.Buffer, p tw.Ptr[*bytes.Buffer]) error {
			if p.IsNil() {
				bb.WriteString("null")
			}
			return p.Walk(ctx)
		}
	})

	{
		walker := tw.NewWalker(register)
		typeFn, err := tw.TypeFnFor[[]*Outer](walker)
		require.NoError(b, err)

		serializeTypeWalk := func(toSerialize *[]*Outer) {
			bb.Reset()
			err := typeFn(&bb, toSerialize)
			require.NoError(b, err)
		}

		b.Run("type-walk", func(b *testing.B) {
			toSerializePtr := &toSerialize
			runtime.GC()
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				serializeTypeWalk(toSerializePtr)
				bs := bb.Bytes()
				_ = bs
			}
		})

	}

	{
		walker := tw.NewWalker(register, tw.WithThreadSafe)
		typeFn, err := tw.TypeFnFor[[]*Outer](walker)
		require.NoError(b, err)

		serializeTypeWalk := func(toSerialize *[]*Outer) {
			bb.Reset()
			err := typeFn(&bb, toSerialize)
			require.NoError(b, err)
		}

		b.Run("type-walk-thread-safe", func(b *testing.B) {
			toSerializePtr := &toSerialize
			runtime.GC()
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				serializeTypeWalk(toSerializePtr)
				bs := bb.Bytes()
				_ = bs
			}
		})
	}

}

func randString(length int) string {
	var letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bs := make([]byte, rand.Intn(length))
	for i := range bs {
		bs[i] = letters[rand.Intn(len(letters))]
	}
	return string(bs)
}
