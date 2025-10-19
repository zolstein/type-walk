[![Go Reference](https://pkg.go.dev/badge/github.com/zolstein/type-walk.svg)](https://pkg.go.dev/github.com/zolstein/type-walk)

# type-walk - Fast reflection for mere mortals

## Warning: Experimental

type-walk is an experimental library. It probably has enough functionality to use in some real projects, but it
still lacks features, and adding them over time may change the API. I've tried to test all possible edge cases and round
off the sharp corners, but it's a big ball of unsafe code and it's possible I've missed some.

If you use this in production, do so at your own risk. Know that the benefits are critical to your project. Audit my
code yourself, and test your own code rigorously. Be prepared to update your code in response to API changes or freeze
the library to a specific version.

## Quick Start

### Installation

```bash
go get github.com/zolstein/type-walk
```

### Basic Example

```go
package main

import (
    "fmt"
    "strings"
    tw "github.com/zolstein/type-walk"
)

func main() {
    // Create a register and add a handler for strings
    register := tw.NewRegister[*strings.Builder]()

    tw.RegisterTypeFn(register, func(ctx *strings.Builder, v tw.String) error {
      ctx.WriteString(v.Get())
      return nil
    })

    // Create walker and use it
    walker := tw.NewWalker(register)
    var buf strings.Builder

    err := walker.Walk(&buf, "Hello, world!")
	if err != nil {
		panic(err)
    }
    fmt.Println(buf.String()) // Output: Hello, world!
}
```

## Table of Contents

- [Warning: Experimental](#warning-experimental)
- [Quick Start](#quick-start)
- [Why does this even exist?](#why-does-this-even-exist)
- [Examples](#examples)
- [Performance](#performance)
- [How does it work](#how-does-it-work)
  - [Walking](#walking)
  - [Compiling](#compiling)
  - [Complex Kinds](#complex-kinds)
  - [Structs](#structs)
- [Values](#values)
- [API Reference](#api-reference)

## Why does this even exist?

The `reflect` package in Go is useful for writing general libraries that can process data of any type.
However, it has a large drawback - it's SLOW. Many common patterns cause the runtime to allocate lots of memory.
Allocating memory and collecting garbage are frequently large parts of Go programs' CPU time - in programs that use
reflection, it is often a main contributor.

Some patterns can allow programmers to get the benefits of reflection while avoiding most of the runtime cost.
One common pattern is to use reflection to analyze a type, "compile" a function that stores information about the type,
and use this function to process many values of that type. However, this code needed to accomplish this can be **gnarly**.
It can require converting every value to an unsafe.Pointer and using pointer-arithmetic to walk the type. Writing this
is tedious, error-prone, and **wildly** unsafe.

type-walk attempts to abstract the unsafe code and provide a safe interface to build fast reflective code.

## Examples

For complete, runnable examples see the [package documentation](https://pkg.go.dev/github.com/zolstein/type-walk) or check out `examples_test.go` in this repository.

**Basic pattern:**
```go
package main
import (
	"time"
    tw "github.com/zolstein/type-walk"
)
type YourContext struct {
    // Any data you want to pass into your walk functions
}
func main() {
	// 1. Create register and add handlers
	register := tw.NewRegister[YourContext]()
	// Directly register WalkFn handlers for types you want to handle specially.
	var yourTimeHandler tw.WalkFn[YourContext, time.Time]
	tw.RegisterTypeFn[YourContext, time.Time](register, yourTimeHandler)
	// Register CompileFn handlers to handle unknown types by kind.
	var yourStringHandler tw.CompileFn[YourContext, string]
	tw.RegisterCompileStringFn(register, yourStringHandler)
	var yourStructHandler tw.CompileStructFn[YourContext]
	tw.RegisterCompileStructFn(register, yourStructHandler)
	// 2. Create walker
	walker := tw.NewWalker(register)
	// 3. Walk your data
	var yourData any
	err := walker.Walk(YourContext{}, yourData)
}
```

## Performance

Benchmark results comparing type-walk to standard reflection implementing a simplified JSON serializer:

| Implementation | Iterations | ns/op   | B/op   | allocs/op |
|---------------|-----------|---------|--------|-----------|
| reflect       | 10,000    | 111,963 | 28,920 | 2,409     |
| type-walk     | 24,776    | 49,587  | 0      | 0         |

Run `go test -bench=.` to see benchmarks on your system.

## How does it work

type-walk uses a two-stage approach similar to Go's `regexp` package: compile once, use many times.
All values of the same type have identical structure, so we can analyze each type once and reuse that analysis.

**The Two Stages:**
1. **Compile** - Analyze a type and generate a fast walk function for it (like `regexp.Compile`)
2. **Walk** - Use that pre-compiled walk function on actual values

Just as `regexp` does expensive analysis once to compile a fast Regexp, then uses it to search many strings,
`type-walk` does expensive analysis once to compile a fast walk function, then uses it to process many values.

### Walking

Walking means calling a function to recursively process a value. The walker:

1. Uses reflection once to determine the value's type
2. Finds the appropriate walk function (either pre-registered or compiled on-demand).
    1. If a registered walk function exists for the value's type, use it.
    2. If a compile function exists for the value's kind - int, struct, slice, etc. - compile a new walk function and register it for future use.
    3. Otherwise, return an error.
3. Calls that function with your value

**Walk Function Signature:**
```go
type WalkFn[Ctx any, In any] func(Ctx, Arg[In]) error
```

- `In` - The type being processed (e.g., `string`, `Person`)
- `Arg[In]` - Wrapper providing `Get()` and `Set()` methods for the value
- `Ctx` - An arbitrary type to pass data into the WalkFn, or to store and return results.
  - `Ctx` should not be confused with `context.Context`. However, you might include a `context.Context` inside your `Ctx` type if you need it.


### Compiling

Compiling creates walk functions for types that haven't been seen before. You register compile functions by **kind** - not specific type.

**Compile Function Signature:**
```go
type CompileFn[Ctx any, In any] func(reflect.Type) WalkFn[Ctx, In]
```

A compile function takes a `reflect.Type` and returns a walk function for that specific type. For example, if you
register a `CompileFn[Ctx, int]`, it will be used to generate functions for `int`, `type UserID int`, `type Count int`,
etc.

**Example:**
```go
RegisterCompileIntFn(register, func(typ reflect.Type) WalkFn[Ctx, int] {
    return func(ctx Ctx, i Arg[int]) error {
        fmt.Printf("Processing %s: %d\n", typ.Name(), i.Get())
        return nil
    }
})
```

### Complex Kinds

For complex types like slices, arrays, structs, and pointers, type-walk provides specialized types that enable recursive walking.

**Slice Function signatures:**
```go
type WalkSliceFn[Ctx any] func(Ctx, Slice[Ctx]) error
type CompileSliceFn[Ctx any] func(reflect.Type) WalkSliceFn[Ctx]
```

`Slice[Ctx]` represents a slice and provides methods to access its length, capacity, and nil status.
Most importantly, you can get an element with `Elem(i)` and recursively walk it with the element's `Walk` method.

**Example:**
```go
RegisterCompileSliceFn(register, func(typ reflect.Type) WalkSliceFn[Ctx] {
    return func(ctx Ctx, s Slice[Ctx]) error {
        for i := 0; i < s.Len(); i++ {
            s.Elem(i).Walk(ctx) // Walk each element
        }
        return nil
    }
})
```

All other complex types have similar patterns - they provide specialized helper types that let you examine some information about them, and recursively walk their contents.

### Structs

Structs are more complex than slices because they can have fields of multiple different types. Additionally, you may not want to process all fields of every struct.

**Function signature:**
```go
type CompileStructFn[Ctx any] func(reflect.Type, StructFieldRegister) WalkStructFn[Ctx]
```

You must use the `StructFieldRegister` in the `CompileStructFn` to explicitly register which fields you want to be available in the `WalkStructFn`.

**Field registration methods:**
- `RegisterField(fieldNum)` - Register a direct field by number
- `RegisterFieldByIndex([]int{...})` - Register nested fields (like `person.Address.Street`)

**Example:**
```go
RegisterCompileStructFn(register, func(typ reflect.Type, reg StructFieldRegister) WalkStructFn[Ctx] {
    // Register all fields we want to process
    for i := 0; i < typ.NumField(); i++ {
        reg.RegisterField(i)
    }

    return func(ctx Ctx, s Struct[Ctx]) error {
        for i := 0; i < s.NumFields(); i++ {
            field := s.Field(i)
            if field.IsValid() {
                field.Walk(ctx) // Walk each registered field
            }
        }
        return nil
    }
})
```

## Values

This section outlines the values that guide type-walk's design decisions and trade-offs.

**Primary Goals:**

* **Performance** - Fast execution with minimal allocations after initial compilation.
    * This should not be compromised for anything other than safety.

* **Safety** - All unsafe operations stay internal to the library. Users should not be able to cause undefined behavior through the safe API or need to import the `unsafe` package.
    * There may be optional features that can be used unsafely. These should be clearly marked and require users to explicitly opt in.

* **Ease-of-use** - Simple API for common use cases, with a reasonable learning curve. Code using type-walk should be readable and maintainable.
    * Convenience features should be added, even if they're not strictly necessary.

**Secondary Goals:**

* **Flexibility** - Support varied use cases through plain Go code rather than DSLs. Enable replacement of most reflection-based value walking.

**Non-priorities:**

* **Simplicity** - Internal code complexity is acceptable if it keeps the external API simple.

* **Backward compatibility** - Breaking changes are expected during development. Pin to specific versions if stability is required.

## API Reference

For complete API documentation, see [pkg.go.dev](https://pkg.go.dev/github.com/zolstein/type-walk).

### Core Types

- `Register[Ctx]` - Stores registered walk and compile functions
- `Walker[Ctx]` - Compiles and executes registered functions on values
- `TypeFn[Ctx, T]` - Pre-compiled function for walking values of type T
- `Arg[T]` - Wrapper for values being walked (provides Get/Set methods)

### Registration Functions

- `RegisterTypeFn[Ctx, T]` - Register handler for specific type T
- `RegisterCompileStringFn[Ctx]` - Register compile handler for string types
- `RegisterCompileStructFn[Ctx]` - Register compile handler for struct types
- Similar functions exist for Bool, Int, Slice, Array, Ptr, Map, Interface, etc.

### Walk Functions

- `WalkFn[Ctx, T]` - Function type for handling values of simple type T
- `WalkStructFn[Ctx]` - Function type for handling struct values
- `WalkSliceFn[Ctx]` - Function type for handling slice values
- Similar types exist for Array, Ptr, Map, Interface, etc.

### Abstract Walking Types

- `Struct[Ctx]` - Represents a struct during walking
- `Slice[Ctx]` - Represents a slice during walking
- `Ptr[Ctx]` - Represents a pointer during walking
- Similar types exist for other complex kinds
