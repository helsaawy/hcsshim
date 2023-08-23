// This package provides an optional data type, similar to Rust's `Option<T>` (and Haskell's `Maybe a`).
package option

// ideally Option would be defined as:
//
//	type Option[T any] struct {
//	  v *T
//	}
//
// but "encoding/json".Marshal matches the type with reflect.Pointer to determine if a type is "empty"
// for the `omitempty` option
//
// However, this means that we cannot define methods on [Option], since its underlying type is *T.

// Option carries either a value of type T, or nothing.
type Option[T any] *T

func Some[T any](v T) Option[T] { return Option[T](&v) }
func None[T any]() Option[T]    { return Option[T](nil) }

func IsNone[T any](o Option[T]) bool { return o == nil }
func IsSome[T any](o Option[T]) bool { return !IsNone(o) }

// helpers

// Unwrap returns the Option's value, or panics if the Option [IsNone].
//
// Use [UnwrapOr] or [UnwrapOrDefault] for functions that do not panic.
func Unwrap[T any](o Option[T]) T {
	if IsNone(o) {
		panic("(Option).Unwrap called on a None value")
	}
	return unwrapUnsafe(o)
}

// UnwrapOrDefault returns the Option's value if it [IsSome], or the type's default value otherwise.
func UnwrapOrDefault[T any](o Option[T]) T {
	return UnwrapOr(o, *new(T))
}

// UnwrapOr returns the Option's value if it [IsSome], or v otherwise.
func UnwrapOr[T any](o Option[T], v T) T {
	if IsNone(o) {
		return v
	}
	return unwrapUnsafe(o)
}

// UnwrapOrElse returns the Option's value if it [IsSome], or the result of f() otherwise.
func UnwrapOrElse[T any](o Option[T], f func() T) T {
	if IsNone(o) {
		return f()
	}
	return unwrapUnsafe(o)
}

//the Map*() functions can't be a method cause methods cannot have type parameters ....

// Map applies f to the Option's value if it [IsSome], or returns a [None[U]] otherwise.
func Map[T, U any](o Option[T], f func(T) U) Option[U] {
	if IsNone(o) {
		return None[U]()
	}
	return Some(f(unwrapUnsafe(o)))
}

// MapOr applies f to the Option's value if it [IsSome], or returns v otherwise.
func MapOr[T, U any](o Option[T], v U, f func(T) U) U {
	if IsNone(o) {
		return v
	}
	return f(unwrapUnsafe(o))
}

// MapOrElse applies f to the Option's value if it [IsSome], or returns g() otherwise.
func MapOrElse[T, U any](o Option[T], g func() U, f func(T) U) U {
	if IsNone(o) {
		return g()
	}
	return f(unwrapUnsafe(o))
}

func unwrapUnsafe[T any](o Option[T]) T { return *o }
