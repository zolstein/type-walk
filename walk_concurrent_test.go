package type_walk_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	tw "github.com/zolstein/type-walk"
)

func TestConcurrent(t *testing.T) {
	r := tw.NewRegister[struct{}]()
	var global atomic.Int64
	r.RegisterCompileInt64Fn(func(r reflect.Type) tw.WalkFn[struct{}, int64] {
		return func(s struct{}, a tw.Arg[int64]) error {
			global.Add(a.Get())
			return nil
		}
	})

	type A int64
	type B int64
	type fn func(walker *tw.Walker[struct{}], i int64)

	var start sync.WaitGroup
	var end sync.WaitGroup

	walkA := func(walker *tw.Walker[struct{}], i int64) {
		start.Wait()
		err := walker.Walk(struct{}{}, A(i))
		require.NoError(t, err)
		end.Done()
	}

	walkB := func(walker *tw.Walker[struct{}], i int64) {
		start.Wait()
		err := walker.Walk(struct{}{}, B(i))
		require.NoError(t, err)
		end.Done()
	}

	typeForA := func(walker *tw.Walker[struct{}], i int64) {
		start.Wait()
		fn, err := tw.TypeFnFor[A](walker)
		require.NoError(t, err)
		a := A(i)
		err = fn(struct{}{}, &a)
		require.NoError(t, err)
		end.Done()
	}

	typeForB := func(walker *tw.Walker[struct{}], i int64) {
		start.Wait()
		fn, err := tw.TypeFnFor[B](walker)
		require.NoError(t, err)
		b := B(i)
		err = fn(struct{}{}, &b)
		require.NoError(t, err)
		end.Done()
	}

	helper := func(fns []fn) {
		for i := 0; i < 1000; i++ {
			global.Store(0)
			walker := tw.NewWalker[struct{}](r, tw.WithThreadSafe)
			start.Add(1)
			end.Add(100)
			for j := 0; j < 100; j++ {
				go fns[j%len(fns)](walker, int64(j))
			}
			start.Done()
			end.Wait()
			assert.Equal(t, 4950, int(global.Load()))
		}
	}

	type testCase struct {
		name string
		fns  []fn
	}

	cases := []testCase{
		{
			name: "walk-one-type",
			fns:  []fn{walkA},
		},
		{
			name: "walk-two-types",
			fns:  []fn{walkA, walkB},
		},
		{
			name: "type-for-one-type",
			fns:  []fn{typeForA},
		},
		{
			name: "type-for-two-types",
			fns:  []fn{typeForA, typeForB},
		},
		{
			name: "mix-one-type",
			fns:  []fn{typeForA, walkA},
		},
		{
			name: "mix-all",
			fns:  []fn{walkA, walkB, typeForA, typeForB},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper(tc.fns)
		})
	}

}
