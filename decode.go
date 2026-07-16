// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

// NewDecoder returns a new form Decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r, d: defaultDelimiter, e: defaultEscape}
}

// Decoder decodes data from a form (application/x-www-form-urlencoded).
type Decoder struct {
	r             io.Reader
	d             rune
	e             rune
	ignoreUnknown bool
	ignoreCase    bool
	maxSize       int
	maxDepth      int
	keyFn         func(string) string
}

// DelimitWith sets r as the delimiter used for composite keys by Decoder d and returns the latter; it is '.' by default.
func (d *Decoder) DelimitWith(r rune) *Decoder {
	d.d = r
	return d
}

// EscapeWith sets r as the escape used for delimiters (and to escape itself) by Decoder d and returns the latter; it is '\\' by default.
func (d *Decoder) EscapeWith(r rune) *Decoder {
	d.e = r
	return d
}

// Reset switches the Decoder to read from r, retaining all other
// configuration, and returns the Decoder. It allows a configured Decoder to
// be reused (e.g. via sync.Pool) without reallocation.
func (d *Decoder) Reset(r io.Reader) *Decoder {
	d.r = r
	return d
}

// Decode reads in and decodes form-encoded data into dst.
func (d Decoder) Decode(dst interface{}) error {
	if d.r == nil {
		return NewError(OpDecode, KindIO, nil, "form: cannot decode from a nil reader")
	}
	bs, err := io.ReadAll(d.r)
	if err != nil {
		return wrapKind(OpDecode, KindIO, err)
	}
	vs, err := url.ParseQuery(string(bs))
	if err != nil {
		return wrapKind(OpDecode, KindSyntax, err)
	}
	return d.decode(reflect.ValueOf(dst), vs)
}

// IgnoreUnknownKeys if set to true it will make the Decoder ignore values
// that are not found in the destination object instead of returning an error.
func (d *Decoder) IgnoreUnknownKeys(ignoreUnknown bool) {
	d.ignoreUnknown = ignoreUnknown
}

// IgnoreCase if set to true it will make the Decoder try to set values in the
// destination object even if the case does not match.
func (d *Decoder) IgnoreCase(ignoreCase bool) {
	d.ignoreCase = ignoreCase
}

// MaxSize overrides how large a slice the Decoder will grow in response to an
// explicit index in the input, guarding against memory exhaustion from a small
// payload with a very large index (e.g. "Foo.900000000=x").
//
// By default (the zero value) the Decoder uses a bound proportional to the
// number of elements actually supplied, so legitimately large slices decode
// while amplification is prevented; most callers never need this method. A
// value > 0 sets a fixed absolute cap instead (use a large value to permit
// large sparse slices in trusted input, or a small one to tighten the limit).
// A value < 0 disables the bound entirely; only do this for fully trusted
// input.
func (d *Decoder) MaxSize(maxSize int) *Decoder {
	d.maxSize = maxSize
	return d
}

// KeysWith sets f as a transformation applied to struct field names — at
// every nesting level — to obtain their form keys, and returns the Decoder.
// Fields with an explicit tag are exempt: tags always name keys verbatim.
// Use it to map Go naming to a wire convention, e.g. snake_case:
//
//	d.KeysWith(strcase.ToSnake) // or any func(string) string
//
// The same transformation should be set on the Encoder (see
// Encoder.KeysWith) for values to round-trip. Configure IgnoreCase
// separately if case-insensitive matching of the transformed keys is also
// desired.
func (d *Decoder) KeysWith(f func(string) string) *Decoder {
	d.keyFn = f
	return d
}

// MaxDepth overrides the maximum key-path nesting depth the Decoder will parse,
// guarding against stack exhaustion and nested-map blow-up from a single key
// with many delimiters (e.g. "a.a.a.…=x"). Legitimate nesting is bounded by the
// destination type, so the default sits far above any real form. A value > 0
// sets the limit; a value < 0 disables it (trusted input only); the zero value
// uses the built-in default.
func (d *Decoder) MaxDepth(maxDepth int) *Decoder {
	d.maxDepth = maxDepth
	return d
}

// sliceLimit reports the largest slice length decodeSlice may allocate given
// count elements supplied for the slice. A negative result means unbounded.
func (d Decoder) sliceLimit(count int) int {
	switch {
	case d.maxSize < 0:
		return -1 // Explicitly unbounded (trusted input).
	case d.maxSize > 0:
		return d.maxSize // Explicit absolute cap.
	default:
		if limit := count * sliceGrowthFactor; limit > sliceGrowthFloor {
			return limit
		}
		return sliceGrowthFloor
	}
}

// depthLimit reports the maximum key-path nesting depth the Decoder will parse.
// A negative result means unbounded.
func (d Decoder) depthLimit() int {
	switch {
	case d.maxDepth < 0:
		return -1 // Explicitly unbounded (trusted input).
	case d.maxDepth > 0:
		return d.maxDepth // Explicit limit.
	default:
		return builtinMaxDepth
	}
}

// DecodeString decodes src into dst.
func (d Decoder) DecodeString(dst interface{}, src string) error {
	vs, err := url.ParseQuery(src)
	if err != nil {
		return wrapKind(OpDecode, KindSyntax, err)
	}
	return d.decode(reflect.ValueOf(dst), vs)
}

// DecodeValues decodes vs into dst.
func (d Decoder) DecodeValues(dst interface{}, vs url.Values) error {
	return d.decode(reflect.ValueOf(dst), vs)
}

// DecodeString decodes src into dst.
func DecodeString(dst interface{}, src string) error {
	return NewDecoder(nil).DecodeString(dst, src)
}

// DecodeValues decodes vs into dst.
func DecodeValues(dst interface{}, vs url.Values) error {
	return NewDecoder(nil).DecodeValues(dst, vs)
}

// decode parses vs into a node tree and decodes it into v. Parsing runs inside
// the same deferred recover as decoding on purpose: malformed input can panic
// during parseValues (a colliding key path, or a key nested past the maximum
// depth), and that panic must be turned into an error rather than escaping to
// the caller and crashing the process.
func (d Decoder) decode(v reflect.Value, vs url.Values) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = asError(OpDecode, e)
		}
	}()

	if v.Kind() == reflect.Slice {
		return &Error{Op: OpDecode, Kind: KindUnsupported, msg: "could not decode directly into slice; use pointer to slice"}
	}
	d.decodeValue(v, parseValues(d.d, d.e, vs, canIndexOrdinally(v), d.depthLimit()))
	return nil
}

func (d Decoder) decodeValue(v reflect.Value, x interface{}) {
	t := v.Type()
	k := v.Kind()

	if k == reflect.Ptr && v.IsNil() {
		v.Set(reflect.New(t.Elem()))
	}

	if unmarshalValue(v, x) {
		return
	}

	empty := isEmpty(x)

	switch k {
	case reflect.Ptr:
		d.decodeValue(v.Elem(), x)
		return
	case reflect.Interface:
		if !v.IsNil() {
			d.decodeValue(v.Elem(), x)
			return

		} else if empty {
			return // Allow nil interfaces only if empty.
		} else {
			panic(errKind(KindUnsupported, "form: cannot decode non-empty value into nil interface"))
		}
	}

	if empty {
		v.Set(reflect.Zero(t)) // Treat the empty string as the zero value.
		return
	}

	switch k {
	case reflect.Struct:
		if s, ok := x.(string); ok && isColorType(t) {
			decodeColor(v, s)
		} else if t.ConvertibleTo(timeType) {
			d.decodeTime(v, x)
		} else if t.ConvertibleTo(urlType) {
			d.decodeURL(v, x)
		} else {
			d.decodeStruct(v, x)
		}
	case reflect.Slice:
		d.decodeSlice(v, x)
	case reflect.Array:
		d.decodeArray(v, x)
	case reflect.Map:
		d.decodeMap(v, x)
	case reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer, reflect.Chan, reflect.Func:
		panic(errKind(KindUnsupported, t.String()+" has unsupported kind "+k.String()))
	default:
		d.decodeBasic(v, x)
	}
}

func (d Decoder) decodeStruct(v reflect.Value, x interface{}) {
	t := v.Type()
	for k, c := range getNode(x) {
		if f, ok := findField(v, k, d.ignoreCase, d.keyFn); !ok && k == "" {
			panic(errKind(KindParse, getString(x)+" cannot be decoded as "+t.String()))
		} else if !ok {
			if !d.ignoreUnknown {
				panic(errKind(KindUnknownKey, k+" doesn't exist in "+t.String()))
			}
		} else if !f.CanSet() {
			panic(errKind(KindUnsupported, k+" cannot be set in "+t.String()))
		} else {
			d.decodeValue(f, c)
		}
	}
}

func (d Decoder) decodeMap(v reflect.Value, x interface{}) {
	t := v.Type()
	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}
	for k, c := range getNode(x) {
		i := reflect.New(t.Key()).Elem()
		d.decodeValue(i, k)

		w := v.MapIndex(i)
		if w.IsValid() { // We have an actual element value to decode into.
			if w.Kind() == reflect.Interface {
				w = w.Elem()
			}
			w = reflect.New(w.Type()).Elem()
		} else if t.Elem().Kind() != reflect.Interface { // The map's element type is concrete.
			w = reflect.New(t.Elem()).Elem()
		} else {
			// The best we can do here is to decode as either a string (for scalars) or a map[string]interface {} (for the rest).
			// We could try to guess the type based on the string (e.g. true/false => bool) but that'll get ugly fast,
			// especially if we have to guess the kind (slice vs. array vs. map) and index type (e.g. string, int, etc.)
			switch c.(type) {
			case node:
				w = reflect.MakeMap(stringMapType)
			case string:
				w = reflect.New(stringType).Elem()
			default:
				panic("value is neither node nor string")
			}
		}

		d.decodeValue(w, c)
		v.SetMapIndex(i, w)
	}
}

func (d Decoder) decodeArray(v reflect.Value, x interface{}) {
	t := v.Type()
	for k, c := range getNode(x) {
		i, err := strconv.Atoi(k)
		if err != nil || i < 0 {
			panic(errKind(KindIndex, k+" is not a valid index for type "+t.String()))
		}
		if l := v.Len(); i >= l {
			panic(errKind(KindIndex, "index is above array size"))
		}
		d.decodeValue(v.Index(i), c)
	}
}

func (d Decoder) decodeSlice(v reflect.Value, x interface{}) {
	t := v.Type()
	if t.Elem().Kind() == reflect.Uint8 {
		// Allow, but don't require, byte slices to be encoded as a single string.
		if s, ok := x.(string); ok {
			v.SetBytes([]byte(s))
			return
		}
	}

	n := getNode(x)
	limit := d.sliceLimit(len(n))

	// NOTE: Implicit indexing is currently done at the parseValues level,
	//       so if if an implicitKey reaches here it will always replace the last.
	implicit := 0
	for k, c := range n {
		var i int
		if k == implicitKey {
			i = implicit
			implicit++
		} else {
			explicit, err := strconv.Atoi(k)
			if err != nil {
				panic(errKind(KindIndex, k+" is not a valid index for type "+t.String()))
			}
			i = explicit
			implicit = explicit + 1
		}
		if i < 0 {
			panic(errKind(KindIndex, k+" is not a valid index for type "+t.String()))
		}
		// Guard against a small payload forcing a huge allocation via a large
		// explicit index. The limit tracks the number of supplied elements, so
		// legitimately large slices are unaffected; see Decoder.MaxSize.
		if limit >= 0 && i >= limit {
			panic(errKind(KindLimit, "index "+strconv.Itoa(i)+" exceeds the allowed size ("+
				strconv.Itoa(limit)+") for type "+t.String()+
				"; supply more elements or set Decoder.MaxSize for trusted input"))
		}
		// "Extend" the slice if it's too short.
		if l := v.Len(); i >= l {
			delta := i - l + 1
			v.Set(reflect.AppendSlice(v, reflect.MakeSlice(t, delta, delta)))
		}
		d.decodeValue(v.Index(i), c)
	}
}

func (d Decoder) decodeBasic(v reflect.Value, x interface{}) {
	t := v.Type()
	switch k, s := t.Kind(), getString(x); k {
	case reflect.Bool:
		if b, e := strconv.ParseBool(s); e == nil {
			v.SetBool(b)
		} else {
			panic(errKind(KindParse, "could not parse bool from "+strconv.Quote(s)))
		}
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		if i, e := strconv.ParseInt(s, 10, 64); e == nil {
			v.SetInt(i)
		} else {
			panic(errKind(KindParse, "could not parse int from "+strconv.Quote(s)))
		}
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		if u, e := strconv.ParseUint(s, 10, 64); e == nil {
			v.SetUint(u)
		} else {
			panic(errKind(KindParse, "could not parse uint from "+strconv.Quote(s)))
		}
	case reflect.Float32,
		reflect.Float64:
		if f, e := strconv.ParseFloat(s, 64); e == nil {
			v.SetFloat(f)
		} else {
			panic(errKind(KindParse, "could not parse float from "+strconv.Quote(s)))
		}
	case reflect.Complex64,
		reflect.Complex128:
		var c complex128
		if n, err := fmt.Sscanf(s, "%g", &c); n == 1 && err == nil {
			v.SetComplex(c)
		} else {
			panic(errKind(KindParse, "could not parse complex from "+strconv.Quote(s)))
		}
	case reflect.String:
		v.SetString(s)
	default:
		panic(errKind(KindUnsupported, t.String()+" has unsupported kind "+k.String()))
	}
}

func (d Decoder) decodeTime(v reflect.Value, x interface{}) {
	t := v.Type()
	s := getString(x)
	// TODO: Find a more efficient way to do this.
	for _, f := range allowedTimeFormats {
		if p, err := time.Parse(f, s); err == nil {
			v.Set(reflect.ValueOf(p).Convert(v.Type()))
			return
		}
	}
	panic(errKind(KindParse, "cannot decode string `"+s+"` as "+t.String()))
}

func (d Decoder) decodeURL(v reflect.Value, x interface{}) {
	t := v.Type()
	s := getString(x)
	if u, err := url.Parse(s); err == nil {
		v.Set(reflect.ValueOf(*u).Convert(v.Type()))
		return
	}
	panic(errKind(KindParse, "cannot decode string `"+s+"` as "+t.String()))
}

var allowedTimeFormats = []string{
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05.999999999Z07",
	"2006-01-02T15:04:05.999999999Z",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05Z07",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04Z",
	"2006-01-02T15:04",
	"2006-01-02T15Z",
	"2006-01-02T15",
	"2006-01-02",
	"2006-01",
	"2006",
	"15:04:05.999999999Z07:00",
	"15:04:05.999999999Z07",
	"15:04:05.999999999Z",
	"15:04:05.999999999",
	"15:04:05Z07:00",
	"15:04:05Z07",
	"15:04:05Z",
	"15:04:05",
	"15:04Z",
	"15:04",
	"15Z",
	"15",
}
