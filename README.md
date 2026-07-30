form
====

A Form Encoding & Decoding Package for Go, written by [Alvaro J. Genial](http://alva.ro).

[![Build Status](https://github.com/ajg/form/actions/workflows/ci.yml/badge.svg)](https://github.com/ajg/form/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ajg/form.svg)](https://pkg.go.dev/github.com/ajg/form)

Synopsis
--------

This library is designed to allow seamless, high-fidelity encoding and decoding of arbitrary data in `application/x-www-form-urlencoded` format and as [`url.Values`](http://golang.org/pkg/net/url/#Values). It is intended to be useful primarily in dealing with web forms and URI query strings, both of which natively employ said format.

Unsurprisingly, `form` is modeled after other Go [`encoding`](http://golang.org/pkg/encoding/) packages, in particular [`encoding/json`](http://golang.org/pkg/encoding/json/), and follows the same conventions (see below for more.) It aims to automatically handle any kind of concrete Go [data value](#values) (i.e., not functions, channels, etc.) while providing mechanisms for custom behavior.

Status
------

`form` is stable and in production use. The public API and the wire format are settled: within the v1 series they will not change incompatibly, and anything that would require a breaking change is deferred to a future v2 (see [Future Work](#future-work)).

The package is thoroughly tested — an extensive unit suite, end-to-end HTTP tests, and a native fuzz target seeded from past bugs — and has been hardened against the resource-exhaustion and malformed-input classes: bounded slice growth, nesting depth, and raw input size, with typed, classified errors on every decode path. It has not undergone a formal third-party security audit, so give untrusted input the usual care and set the available limits. Please file an issue or open a pull request for anything you find (see [CONTRIBUTING](CONTRIBUTING.md) and [SECURITY](SECURITY.md)).

Dependencies
------------

The only requirement is [Go 1.17](http://golang.org/doc/go1.17) or later.

Usage
-----

```go
import "github.com/ajg/form"
// or: "gopkg.in/ajg/form.v1"
```

Given a type like the following...

```go
type User struct {
	Name         string            `form:"name"`
	Email        string            `form:"email"`
	Joined       time.Time         `form:"joined,omitempty"`
	Posts        []int             `form:"posts"`
	Preferences  map[string]string `form:"prefs"`
	Avatar       []byte            `form:"avatar"`
	PasswordHash int64             `form:"-"`
}
```

...it is easy to encode data of that type...


```go
func PostUser(url string, u User) error {
	var c http.Client
	_, err := c.PostForm(url, form.EncodeToValues(u))
	return err
}
```

...as well as decode it...


```go
func Handler(w http.ResponseWriter, r *http.Request) {
	var u User

	d := form.NewDecoder(r.Body)
	if err := d.Decode(&u); err != nil {
		http.Error(w, "Form could not be decoded", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Decoded: %#v", u)
}
```

...without having to do any grunt work.

Field Tags
----------

Like other encoding packages, `form` supports the following options for fields:

 - `` `form:"-"` ``: Causes the field to be ignored during encoding and decoding.
 - `` `form:"<name>"` ``: Overrides the field's name; useful especially when dealing with external identifiers in camelCase, as are commonly found on the web.
 - `` `form:",omitempty"` ``: Elides the field during encoding if it is empty (typically meaning equal to the type's zero value.)
 - `` `form:"<name>,omitempty"` ``: The way to combine the two options above.

When a field carries no `form` tag, a `json` tag, if present, is used in its
place — convenient for structs already annotated for JSON APIs. (The
`form/multipart` subpackage follows the same rule.)

Values
------

### Simple Values

Values of the following types are all considered simple:

 - `bool`
 - `int`, `int8`, `int16`, `int32`, `int64`, `rune`
 - `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `byte`
 - `float32`, `float64`
 - `complex64`, `complex128`
 - `string`
 - `[]byte` (see note)
 - [`time.Time`](http://golang.org/pkg/time/#Time)
 - [`url.URL`](http://golang.org/pkg/net/url/#URL)
 - The fixed-channel [`image/color`](http://golang.org/pkg/image/color/) types
   when represented as hex strings (see [Colors](#colors); unlike `time.Time`
   and `url.URL` these are dual-representation — their composite form below
   also remains supported, and is the default when encoding)
 - An alias of any of the above
 - A pointer to any of the above

### Composite Values

A composite value is one that can contain other values. Values of the following kinds...

 - Maps
 - Slices; except `[]byte` (see note)
 - Structs; except [`time.Time`](http://golang.org/pkg/time/#Time),
   [`url.URL`](http://golang.org/pkg/net/url/#URL) and — when written as hex
   strings — the fixed-channel color types (see [Colors](#colors))
 - Arrays
 - An alias of any of the above
 - A pointer to any of the above

...are considered composites in general, unless they implement custom marshaling/unmarshaling. Composite values are encoded as a flat mapping of paths to values, where the paths are constructed by joining the parent and child paths with a period (`.`).

(Note: a byte slice is treated as a `string` by default because it's more efficient, but can also be decoded as a slice—i.e., with indexes.)

### Untyped Values

While encouraged, it is not necessary to define a type (e.g. a `struct`) in order to use `form`, since it is able to encode and decode untyped data generically using the following rules:

 - Simple values will be treated as a `string`.
 - Because `application/x-www-form-urlencoded` carries no type information, no
   numeric (or other) interpretation is attempted: simple values are preserved
   as strings exactly as sent, so numerals of any magnitude — including those
   exceeding Go's primitive types — survive losslessly, and the caller decides
   how, and at what precision, to parse them.
 - Composite values will be treated as a `map[string]interface{}`, itself able to contain nested values (both simple and composite) ad infinitum.
 - However, if there is a value (of any supported type) already present in a map for a given key, then it will be used when possible, rather than being replaced with a generic value as specified above; this makes it possible to handle partially typed, dynamic or schema-less values.

### Zero Values

By default, and without custom marshaling, zero values (also known as empty/default values) are encoded as the empty string. To disable this behavior, meaning to keep zero values in their literal form (e.g. `0` for integral types), `Encoder` offers a `KeepZeros` setter method, which will do just that when set to `true`.

For instance, given...

```go
foo := map[string]interface{}{"b": false, "i": 0}
```

...the following...

```go
form.EncodeToString(foo) // i.e. keepZeros == false
```

...will result in `"b=&i="`, whereas...

```go
keepZeros := true
delimiter := '.'
escape := '\\'
form.EncodeToStringWith(foo, delimiter, escape, keepZeros)
```

...will result in `"b=false&i=0"`.

### Unsupported Values

Values of the following kinds aren't supported and, if present, must be ignored.

 - Channel
 - Function
 - Unsafe pointer
 - An alias of any of the above
 - A pointer to any of the above

### Colors

The fixed-channel [`image/color`](http://golang.org/pkg/image/color/) types
(`NRGBA`, `RGBA`, `NRGBA64`, `RGBA64`, `Gray`, `Gray16`, `Alpha`, `Alpha16`
and `CMYK`) decode from hex strings such as those submitted by HTML
`<input type=color>` elements — bare or `#`-prefixed, case-insensitive, with
CSS-style shorthand (`f00`, `f00a`) accepted by the 8-bit RGBA types and an
omitted alpha defaulting to opaque. The encoding is byte-faithful per type,
so every value round-trips exactly; accordingly, the premultiplied types
(`RGBA`, `RGBA64`) reject values whose color channels exceed their alpha —
that shape almost always means straight-alpha (CSS) semantics, for which
`NRGBA`/`NRGBA64` are the faithful destinations.

The pre-existing composite representation (`c.R=255&c.G=0&…`) continues to
decode for every color type, and remains the default encoding; opt into hex
output with `Encoder.HexColors(true)`. Named types based on the color types
keep the composite representation entirely.

Custom Marshaling
-----------------

There is a default (generally lossless) marshaling & unmarshaling scheme for any concrete data value in Go, which is good enough in most cases. However, it is possible to override it and use a custom scheme. For instance, a "binary" field could be marshaled more efficiently using [base64](http://golang.org/pkg/encoding/base64/) to prevent it from being percent-escaped during serialization to `application/x-www-form-urlencoded` format.

Because `form` provides support for [`encoding.TextMarshaler`](http://golang.org/pkg/encoding/#TextMarshaler) and [`encoding.TextUnmarshaler`](http://golang.org/pkg/encoding/#TextUnmarshaler) it is easy to do that; for instance, like this:

```go
import "encoding"

type Binary []byte

var (
	_ encoding.TextMarshaler   = &Binary{}
	_ encoding.TextUnmarshaler = &Binary{}
)

func (b Binary) MarshalText() ([]byte, error) {
	return []byte(base64.URLEncoding.EncodeToString([]byte(b))), nil
}

func (b *Binary) UnmarshalText(text []byte) error {
	bs, err := base64.URLEncoding.DecodeString(string(text))
	if err == nil {
		*b = Binary(bs)
	}
	return err
}
```

Now any value with type `Binary` will automatically be encoded using the [URL](http://golang.org/pkg/encoding/base64/#URLEncoding) variant of base64. It is left as an exercise to the reader to improve upon this scheme by eliminating the need for padding (which, besides being superfluous, uses `=`, a character that will end up percent-escaped.)

Standard-library types that implement these interfaces work out of the box.
Notably, that includes [`math/big`](http://golang.org/pkg/math/big/): `Int`
and `Rat` encode and decode losslessly at arbitrary precision, and `Float` at
its destination's precision — a 64-bit mantissa by default (already wider
than `float64`), or any precision pre-set on the destination value with
`SetPrec` before decoding. These are the natural destinations for numeric
values too large for Go's primitive types; decoding an out-of-range value
into a primitive fails loudly rather than truncating silently.

Keys
----

In theory any value can be a key as long as it has a string representation. However, by default, periods have special meaning to `form`, and thus, under the hood (i.e. in encoded form) they are transparently escaped using a preceding backslash (`\`). Backslashes within keys, themselves, are also escaped in this manner (e.g. as `\\`) in order to permit representing `\.` itself (as `\\\.`).

(Note: it is normally unnecessary to deal with this issue unless keys are being constructed manually—e.g. literally embedded in HTML or in a URI.)

The default delimiter and escape characters used for encoding and decoding composite keys can be changed using the `DelimitWith` and `EscapeWith` setter methods of `Encoder` and `Decoder`, respectively. For example...

```go
package main

import (
	"os"

	"github.com/ajg/form"
)

func main() {
	type B struct {
		Qux string `form:"qux"`
	}
	type A struct {
		FooBar B `form:"foo.bar"`
	}
	a := A{FooBar: B{"XYZ"}}
	os.Stdout.WriteString("Default: ")
	form.NewEncoder(os.Stdout).Encode(a)
	os.Stdout.WriteString("\nCustom:  ")
	form.NewEncoder(os.Stdout).DelimitWith('/').Encode(a)
	os.Stdout.WriteString("\n")
}

```

...will produce...

```
Default: foo%5C.bar.qux=XYZ
Custom:  foo.bar%2Fqux=XYZ
```

(`%5C` and `%2F` represent `\` and `/`, respectively.)

Key Mapping
-----------

When a wire naming convention differs from Go's — snake_case being the usual
case — `KeysWith` on both `Decoder` and `Encoder` sets a transformation
applied to struct field names, at every nesting level, to obtain their form
keys:

```go
form.NewDecoder(r).KeysWith(strcase.ToSnake) // or any func(string) string
```

Fields with an explicit tag are exempt — tags always name keys verbatim — and
the same function should be set on both directions for values to round-trip;
passing nil clears the mapping. The function receives struct field names
only, never input data; it should be deterministic and injective — distinct
fields must map to distinct keys, including any explicitly tagged names —
should avoid emitting the reserved implicit-index key `_` or the delimiter
rune, and — since decoding may invoke it once
per candidate field per incoming key — memoize it if it is expensive.
form deliberately ships no built-in casing heuristic: word-boundary rules
(acronyms, digits) are project-specific, and baking one in would freeze its
inevitable edge cases into the wire format. Bring your own mapper and it is
applied mechanically.

Unknown Keys and Case
---------------------

Decoding is strict by default: a key that matches no field in the destination
is an error. `Decoder.IgnoreUnknownKeys(true)` makes the decoder skip such
values instead — the pragmatic choice for raw browser submissions, which
routinely carry fields the destination doesn't model (CSRF tokens,
submit-button names). Separately, `Decoder.IgnoreCase(true)` lets keys match
fields even when their cases differ.

Limits
------

When decoding untrusted input, the `Decoder` bounds two dimensions to prevent
resource-exhaustion denial of service (added in v1.7.2):

 - **Slice size.** An explicit index such as `Foo.1000000=x` will not pre-allocate an
   unbounded slice. By default the allowed length is proportional to the number of
   elements actually supplied, so ordinary data is unaffected while a tiny payload
   with a huge index is rejected. Override with `Decoder.MaxSize(n)`: `n > 0` sets a
   fixed cap, `n < 0` disables the bound (trusted input only), and the zero value
   keeps the proportional default.

 - **Key-path depth.** A key with many delimiters such as `a.a.a.…=x` is rejected
   past a maximum nesting depth (default 10000, far beyond any real form). Override
   with `Decoder.MaxDepth(n)`: `n > 0` sets the limit, `n < 0` disables it (trusted
   input only), and the zero value keeps the default.

 - **Input size.** `Decode` reads its entire input into memory before parsing.
   For untrusted streams, bound it with `Decoder.MaxBytes(n)` (a larger input
   is rejected) or wrap the reader (e.g. with `http.MaxBytesReader`). Unlike
   the bounds above, this one is off by default, because a built-in cap would
   break legitimately large trusted inputs; set it explicitly when decoding
   untrusted data.

The package-level `DecodeString` and `DecodeValues` helpers use these safe defaults.
If you decode large but trusted input and hit a limit, raise or disable it via the
methods above.

Errors
------

Errors returned by the decoding and encoding functions are of type `*form.Error`,
which records the operation that failed via `Op` (`form.OpDecode` or
`form.OpEncode`), the class of failure via `Kind` (`form.KindParse`,
`form.KindUnknownKey`, `form.KindLimit`, and so on; the zero value means
unclassified), and, through `errors.Unwrap`, any underlying cause — such as an
error from a `TextMarshaler`/`TextUnmarshaler` or a malformed-query error from the
standard library. Detect and inspect them with `errors.As` and `errors.Is`. The
message text returned by `Error` is unchanged from earlier versions, so code that
matches on error strings continues to work.

File Uploads (`multipart/form-data`)
------------------------------------

HTML forms that contain file inputs submit `multipart/form-data` rather than
`application/x-www-form-urlencoded`; the subpackage
[form/multipart](./multipart) decodes such submissions into structs. Value
fields follow the same rules as this package (field tags, nesting, custom
delimiters, the limits above), while file parts are matched by name onto
fields declared as any of `*multipart.FileHeader`, `[]*multipart.FileHeader`,
`[]byte`, or `[][]byte`:

```go
import (
	"mime/multipart"

	fmp "github.com/ajg/form/multipart"
)

type Upload struct {
	Name   string                  `form:"name"`
	Avatar *multipart.FileHeader   `form:"avatar"` // Lazy: open/stream/close manually.
	Docs   [][]byte                `form:"docs"`   // Eager: decoding reads into memory.
}

func handle(w http.ResponseWriter, r *http.Request) {
	var u Upload
	if err := fmp.DecodeRequest(&u, r, 32<<20); err != nil { // -> r.ParseMultipartForm.
		// ...
	}
	// ...
}
```

 - The `*multipart.FileHeader` forms hand over the standard library's own handle:
   the caller `Open`s (and closes) the file and may stream it, so no size bound
   is imposed by this package.
 - The `[]byte` forms are read into memory during decoding, bounded by
   `Decoder.MaxFileSize` (10 MiB per file by default) and `Decoder.MaxFiles`
   (1000 per key by default), which follow the same zero/positive/negative
   convention as the limits above.

Like this package, decoding is strict by default: a value or file key that
matches no destination field is an error. Browser submissions routinely carry
extra fields (CSRF tokens, submit-button names); either model them or skip
them with `fmp.NewDecoder().IgnoreUnknownKeys(true)`.

Future Work
-----------

Remaining aspirations are deliberately deferred to a future v2, where some
defaults may change: decoding is expected to become lenient about unknown
keys by default (with strictness opt-in), hex is expected to become the
default encoding for the fixed-channel `image/color` types, and slice
wire-format conventions (such as PHP/Rails-style `services[]=…` or bare
repeated keys) are candidates for an implicit-indexing rework. The
longest-standing item — streaming encoding and decoding directly via the
`io.Reader`/`io.Writer` instead of materializing the input, its `url.Values`
and an intermediate tree in memory — is likewise v2-shaped: implicit indexing
and the deterministic handling of conflicting keys are whole-input properties
that a streaming design must either restrict or redefine.

Related Work
------------

 - Package [gorilla/schema](https://github.com/gorilla/schema), which only implements decoding.
 - Package [google/go-querystring](https://github.com/google/go-querystring), which only implements encoding.

License
-------

This library is distributed under a BSD-style [LICENSE](./LICENSE).
