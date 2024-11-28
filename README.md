[![Go Reference](https://pkg.go.dev/badge/github.com/zolstein/type-walk.svg)](https://pkg.go.dev/github.com/zolstein/type-walk)

# type-walk - Fast reflection for mere mortals

## Warning: Experimental

type-walk is an experimental library. It probably has enough functionality to use in some real projects, but it may
still lack features, and adding them over time may change the API. I've tried to test all possible edge cases and round
off the sharp corners, but it's a big ball of unsafe code and it's possible I've missed some.

I would not run this code in production systems... yet. I plan to improve it over time, and I hope that some people see
value in it and choose to play around with it in non-critical places, file bugs, and offer suggestions for improvement.
In order to get better, it needs to be used.

If you use this in production, do so at your own risk. Know that the benefits are critical to your project. Audit my
code yourself, and test your own code rigorously. Be prepared to update your code in response to API changes or freeze
the library to a specific version.

## Why does this even exist?

The `reflect` package in Go is incredibly useful for writing general libraries that can process data of any type.
However, it comes with a large drawback - it's SLOW. More precisely, many common patterns cause the runtime to
allocate lots of memory and generate lots of garbage. In my experience, allocating memory and collecting garbage are
often the largest parts of what Go programs spend their time doing, and in programs that use reflection, it often
accounts for a not-small portion of that garbage.

There are some patterns that allow programmers to get the benefits of reflection while avoiding the majority of the
runtime cost. The most-common pattern that I have used is to use reflection to walk a type, "compile" a function that
stores information about the type - this function can then be used to process values of that type without needing to
re-analyze the type. However, this pattern is _gnarly_. It requires converting every value to an unsafe.Pointer and using
pointer-arithmetic to walk the type. The code necessary to do this is tedious, hard to get right, hard to understand,
and _wildly_ unsafe.

type-walk attempts to abstract away the unsafe code and provide a safe interface to build fast reflective code.

## How does it work

type-walk leverages one fact, and one assumption, to improve performance over naive reflection:

* Fact: All values of a given type have the same structure.
* Assumption: Code reflectively analyzing objects will generally analyze objects of the same type many times.

Reflection causes unavoidable allocations, but we expect to analyze many values of the same time over the lifetime of
the program. Therefore, to improve performance we can do analysis for each type just once, and generate a much faster
function that can handle the type without using reflection.

To achieve this, we can separate the processing of a value into two stages, which we'll call "compiling" and "walking".
Compiling is the process of analyzing a type and generating a function that can process a value, and walking is the
process of applying that generated function to a particular value.

### Walking

Walking a value just involves taking a value of a given type and passing it into the walk-function that handles values
of that type. How do we know the type of the value? At the start, we use reflection once to get the type. However, if
the value has a structure that requires descending through sub-values, we can know from context what the type of each
sub-value is, so we don't generally need to do more reflection to figure it out.

A walk function (`WalkFn`) has the following type definition:

`type WalkFn[Ctx any, In any] func(Ctx, Arg[In]) error`

`In` is the type being walked. `Arg[In]` is a wrapper type, which represents a value of type `In` in the context of the
walk. The `In` value can be read with`Get`, and (if it represents an addressable value) can be set with `Set`.
`CanSet` can be used to check if the arg is settable.

`Ctx` is a "do whatever you want" parameter. It's a value that's passed along with the values you're walking through the
program. You can use it to modify the way you process a value, pass information between levels of the walk, or expose
results to the rest of your program after the walk finishes. (For example, if you used type-walk to write a JSON
serializer, `Ctx` might contain an io.Writer to serialize to, and an integer to track the indentation level.) However,
note that `Ctx` must be the same type for the entire walk - different In types can't use different Ctx types.

We choose the walk function to apply for a value of a particular type in one of two ways:

1. A function for handling a specific type was registered directly - in this case, we just call that function.
2. No function was registered for that type, so we look for a function to compile one for type based on its kind.
   Assuming we have a compile function for the kind, we compile a new function for the type, save it for future use,
   then call it.

(If we don't have a compile function registered for that kind, we give up and return an error.)

### Compiling

A compiling function (CompileFn) has the following type definition:

`type CompileFn[Ctx any, In any] func(reflect.Type) WalkFn[Ctx, In]`.

Or, spelled out all the way:

`type CompileFn[Ctx any, In any] func(reflect.Type) func(Ctx, Arg[In]) error`.

So it's a function which takes a `reflect.Type`, and returns a function that walks values of that type.

CompileFns are designed to handle all types of a particular kind. For example, a `CompileFn[T, int]` would be called for
any types of kind Int, not _just_ the exact type `int`. For instance, if you defined `type ID int`, the
CompileFn for `int` would be used to compile a WalkFn for `ID`.

If you want a bit more reflective magic in your WalkFn, and having access to just the builtin value type isn't enough,
consider that your CompileFn can return a closure that contains the `reflect.Type` value. Ex:

```
var _ tw.CompileFn[any, int] = func(typ reflect.Type) WalkFn[Ctx, int] {
   return func(ctx any, i Arg[Int]) error {
      fmt.Printf("Walking value of type %s with value %d", typ.Name(), i.Get())
   }
}
```

### Complex Kinds

Handling ints (and bools and strings, and other simple types) is all well and good, but what about more complex types?
What about structs, and slices, and arrays? type-walk supports this too, and this is where the concept of "walking"
really comes into play. For each of these, it provides specialized kinds of functions.

```
type WalkSliceFn[Ctx any] func(Ctx, Slice[Ctx]) error
type CompileSliceFn[Ctx any] func(reflect.Type) WalkSliceFn[Ctx]
```

`Slice` represents a slice of any type, but in the context of the `WalkSliceFn` returned by a `CompileSliceFn`, it will
always contain elements of the `reflect.Type`. `Slice` lets you get the length and capacity of the slice, as well as
whether it's nil. Most importantly, though, it lets you get an element at one of its indexes, with `Elem`, and
you can call the registered `WalkFn` on that element with `Walk`.

All the other more complicated types have similar Walk and Compile functions, as well as similar types representing
their values. Importantly, rather than giving you direct access to the internal values, they provide stub values that
let you walk the inner values recursively. This is key to the model of type-walk, and is part of what allows it to walk
values efficiently - inside the `SliceElem`, it knows what the type of the internal values is and what function it
should call on them.

### Structs

Structs work much like other complex kinds, but they are more complicated in that they may have any number of sub-values
(fields) of any number of distinct types. (By comparison, a slice may have any number of sub-values, but they will all
have the same type.) Furthermore, Go's `reflect` package allows access to fields of nested structs, and it is useful for
type-walk to provide the same functionality. For these reasons, it is sometimes useful to compile a `WalkStructFn` that
cannot walk _all_ fields of the struct.

To accommodate this, `CompileStructFn` has a different type signature:

```
type CompileStructFn[Ctx any] func(reflect.Type, StructFieldRegister) WalkStructFn[Ctx]
```

`StructFieldRegister` is a type that tracks which fields of a struct should be made available to the `WalkStructFn`.
It has two relevant methods: `RegisterField(fieldNum int) int` to register a field by its field number, and
`RegisterFieldByIndex(index []int) int` to register a potentially nested field by its index path. Only fields that are
registered with one of these methods will be available in the `StructWalkFn`. Both methods return an int, which is the
index to use to look up the `StructField` from the `Struct` of this type.

## Virtues

There are many virtues that a piece of software can aspire to uphold. However, some virtues are inherently at odds with
one another. When writing software, it can be helpful to explicitly lay out the virtues that you care about, to ensure
that it stays focused and achieves it's goals. When consuming software, it can be helpful to know the virtues that
underpin a codebase, to ensure that it aligns with your own values. To that end, I've chosen to include a section
describing the virtues that I aim to uphold in type-walk, as well as those on which I'm willing to compromise.

* Performance - If type-walk cannot be fast, it has no reason to exist. At minimum, it should support simple, common
use-cases with very few allocations after the first use for each type. Preferably, it should require no allocations to
walk a value of a pre-processed type, and it should otherwise have as little overhead as possible.

* Safety - type-walk inherently uses a large amount of unsafe code. However, that unsafe code should remain internal
to the library, without leaking unsafe abstractions that the user needs to care about. Ideally, it should be impossible
to write code using this library that is less safe than any other Go code that doesn't use the unsafe package. I may
compromise on this in service of performance or ease-of-use, but at minimum it should be easier to use this safely than
unsafely, and any unsafe behavior that leaks out should require going out of one's way and doing clearly incorrect
things. If it's necessary to import the "unsafe" package to use type-walk, I have failed.

* Ease-of-use - It's important that code using type-walk is easy to understand, write, and modify. It's therefore
important that it's not too complicated to understand. I consider it acceptable for there to be a learning-curve to grok
the library, but once a user has gotten over that hump, it should be easy to understand what the library does and how to
implement common use-cases. Furthermore, type-walk should provide convenience features that make common use-cases more
concise and easier to understand, even if one could implement the same use-cases without them.

* Flexibility - The more use-cases that type-walk can support, the more places it can be used. Ideally, it should be
possible to use type-walk to replace any code that uses reflection to walk a value. In service of this, type-walk tries
to allow users to write their walking procedures as plain code, rather than providing a DSL, which would necessarily
restrict the ways in which it can be used. However, some features have to be provided by the library, because they
cannot be implemented safely in the consumer, which imposes some limitations. Furthermore, there can be trade-offs
between flexibility and ease-of-use. I'm willing to compromise somewhat on flexibility if the alternative would
introduce unavoidable unsafe behaviors or make the API significantly more complicated. I definitely prefer making
complicated use-cases in order to keep simple use-cases simple, compared to adding APIs that make simple use-cases more
complicated.

In particular, below are some virtues that, while not _unimportant_ are not priorities in this code base at this time,
and are the first things I will compromise on if they conflict with a more important one.

* Simplicity - So long as the library is as easy to use and understand as is possible from the outside, I don't
particularly care if the codebase itself is simple. So long as I can understand it, and there are test cases to catch
any potential regressions, simplicity is not a goal for its own sake.

* Backward compatibility - Especially now, with type-walk in an early state and having no users, all APIs are subject to
change for any reason. Likely there will be significant early churn as the best interfaces get hammered out. However,
even if it becomes somewhat more established, given the choice between making the library better and maintaining
backward compatibility, I expect to choose to make it better. If this gains real traction, this consideration may
change, but until then, if you use type-walk, you should be willing to update your code in response to changes or
otherwise pin a version.
