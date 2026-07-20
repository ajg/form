// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"encoding"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// NewEncoder returns a new form Encoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{writer: w, delimiter: defaultDelimiter, escape: defaultEscape}
}

// Encoder provides a way to encode to a Writer.
type Encoder struct {
	writer    io.Writer
	delimiter rune
	escape    rune
	keepZeros bool
	omitEmpty bool
	hexColors bool
	keyFn     func(string) string
}

// DelimitWith sets r as the delimiter used for composite keys by Encoder e
// and returns the latter; it is '.' by default. The delimiter must differ
// from the escape (validated when encoding).
func (e *Encoder) DelimitWith(r rune) *Encoder {
	e.delimiter = r
	return e
}

// EscapeWith sets r as the escape used for delimiters (and to escape itself)
// by Encoder e and returns the latter; it is '\\' by default. The escape must
// differ from the delimiter (validated when encoding).
func (e *Encoder) EscapeWith(r rune) *Encoder {
	e.escape = r
	return e
}

// KeepZeros sets whether Encoder e should keep zero (default) values in their literal form when encoding, and returns the former; by default zero values are not kept, but are rather encoded as the empty string.
func (e *Encoder) KeepZeros(z bool) *Encoder {
	e.keepZeros = z
	return e
}

// KeysWith sets f as a transformation applied to struct field names — at
// every nesting level — to obtain their form keys, and returns the Encoder.
// Fields with an explicit tag are exempt: tags always name keys verbatim.
// Passing nil clears the transformation. f is only ever called with struct
// field names — never with data being encoded — and must be deterministic,
// return non-empty keys, and be injective over a struct's field names,
// tagged ones included; colliding outputs yield an unspecified (though
// safe) result, and outputs should avoid the reserved implicit-index key
// "_" and the delimiter rune. A panic inside f is recovered
// into a *Error. Set the same transformation on the Decoder (see
// Decoder.KeysWith) for values to round-trip.
func (e *Encoder) KeysWith(f func(string) string) *Encoder {
	e.keyFn = f
	return e
}

// HexColors makes the Encoder render the fixed-channel image/color types as
// bare lowercase hex strings (e.g. color.NRGBA => "ff007f80"; see color.go
// for the per-type formats) instead of as composite structs (C.R=255&C.G=0&…).
// It is false by default, preserving the existing composite wire format for
// current consumers. Decoding accepts both representations regardless.
func (e *Encoder) HexColors(h bool) *Encoder {
	e.hexColors = h
	return e
}

// OmitEmpty sets whether Encoder e should omit empty (zero) struct fields during encoding, and returns the former; this is equivalent to having ",omitempty" on every field. By default, empty fields are included.
func (e *Encoder) OmitEmpty(o bool) *Encoder {
	e.omitEmpty = o
	return e
}

// Reset switches the Encoder to write to w, retaining all other
// configuration, and returns the Encoder. It allows a configured Encoder to
// be reused (e.g. via sync.Pool) without reallocation.
func (e *Encoder) Reset(w io.Writer) *Encoder {
	e.writer = w
	return e
}

// Encode encodes dst as form and writes it out using the Encoder's Writer.
func (e Encoder) Encode(dst interface{}) error {
	if e.writer == nil {
		return NewError(OpEncode, KindIO, nil, "form: cannot encode to a nil writer")
	}
	if e.delimiter == e.escape {
		return NewError(OpEncode, KindUnsupported, nil, "form: delimiter and escape must differ")
	}
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v, encOpts{keepZeros: e.keepZeros, omitEmpty: e.omitEmpty, hexColors: e.hexColors, keyFn: e.keyFn})
	if err != nil {
		return err
	}
	s := n.values(e.delimiter, e.escape).Encode()
	l, err := io.WriteString(e.writer, s)
	switch {
	case err != nil:
		return wrapKind(OpEncode, KindIO, err)
	case l != len(s):
		return NewError(OpEncode, KindIO, nil, "could not write data completely")
	}
	return nil
}

// EncodeToString encodes dst as a form and returns it as a string.
func EncodeToString(dst interface{}, needEmptyValue ...bool) (string, error) {
	z := defaultKeepZeros
	if len(needEmptyValue) != 0 {
		z = needEmptyValue[0]
	}
	return EncodeToStringWith(dst, defaultDelimiter, defaultEscape, z)
}

// EncodeToStringWith encodes dst as a form with delimiter d, escape e, keeping zero values if z, and returns it as a string.
// The delimiter and escape must differ.
func EncodeToStringWith(dst interface{}, d rune, e rune, z bool) (string, error) {
	if d == e {
		return "", NewError(OpEncode, KindUnsupported, nil, "form: delimiter and escape must differ")
	}
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v, encOpts{keepZeros: z})
	if err != nil {
		return "", err
	}
	vs := n.values(d, e)
	return vs.Encode(), nil
}

// EncodeToValues encodes dst as a form and returns it as Values.
func EncodeToValues(dst interface{}, needEmptyValue ...bool) (url.Values, error) {
	z := defaultKeepZeros
	if len(needEmptyValue) != 0 {
		z = needEmptyValue[0]
	}
	return EncodeToValuesWith(dst, defaultDelimiter, defaultEscape, z)
}

// EncodeToValuesWith encodes dst as a form with delimiter d, escape e, keeping zero values if z, and returns it as Values.
// The delimiter and escape must differ.
func EncodeToValuesWith(dst interface{}, d rune, e rune, z bool) (url.Values, error) {
	if d == e {
		return nil, NewError(OpEncode, KindUnsupported, nil, "form: delimiter and escape must differ")
	}
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v, encOpts{keepZeros: z})
	if err != nil {
		return nil, err
	}
	vs := n.values(d, e)
	return vs, nil
}

func encodeToNode(v reflect.Value, opts encOpts) (n node, err error) {
	if !v.IsValid() {
		return nil, NewError(OpEncode, KindUnsupported, nil, "form: cannot encode an untyped nil value")
	}
	defer func() {
		if e := recover(); e != nil {
			err = asError(OpEncode, e)
		}
	}()
	seen := make(map[uintptr]bool)
	return getNode(encodeValue(v, opts, seen)), nil
}

func encodeValue(v reflect.Value, opts encOpts, seen map[uintptr]bool) interface{} {
	t := v.Type()
	k := v.Kind()

	if s, ok := marshalValue(v); ok {
		return s
	} else if !opts.keepZeros && isEmptyValue(v) {
		return "" // Treat the zero value as the empty string.
	}

	switch k {
	case reflect.Ptr:
		ptr := v.Pointer()
		if seen[ptr] {
			panic(errKind(KindCycle, "form: encoding a cycle via "+t.String()))
		}
		seen[ptr] = true
		defer delete(seen, ptr)
		return encodeValue(v.Elem(), opts, seen)
	case reflect.Interface:
		return encodeValue(v.Elem(), opts, seen)
	case reflect.Struct:
		if opts.hexColors && isColorType(t) {
			return encodeColor(v)
		}
		if t.ConvertibleTo(timeType) {
			return encodeTime(v)
		} else if t.ConvertibleTo(urlType) {
			return encodeURL(v)
		}
		return encodeStruct(v, opts, seen)
	case reflect.Slice:
		if v.Len() > 0 {
			ptr := v.Pointer()
			if seen[ptr] {
				panic(errKind(KindCycle, "form: encoding a cycle via "+t.String()))
			}
			seen[ptr] = true
			defer delete(seen, ptr)
		}
		return encodeSlice(v, opts, seen)
	case reflect.Array:
		return encodeArray(v, opts, seen)
	case reflect.Map:
		ptr := v.Pointer()
		if seen[ptr] {
			panic(errKind(KindCycle, "form: encoding a cycle via "+t.String()))
		}
		seen[ptr] = true
		defer delete(seen, ptr)
		return encodeMap(v, opts, seen)
	case reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer, reflect.Chan, reflect.Func:
		panic(errKind(KindUnsupported, t.String()+" has unsupported kind "+t.Kind().String()))
	default:
		return encodeBasic(v)
	}
}

type encoderField struct {
	index     []int
	name      string
	omitempty bool
	tagged    bool
}

func encodeStruct(v reflect.Value, opts encOpts, seen map[uintptr]bool) interface{} {
	fields := collectFields(v.Type())
	n := node{}
	for _, f := range fields {
		fv := fieldByIndex(v, f.index)
		if !fv.IsValid() {
			continue
		}
		if (opts.omitEmpty || f.omitempty) && isEmptyValue(fv) {
			continue
		}
		name := f.name
		if opts.keyFn != nil && !f.tagged {
			name = opts.keyFn(name)
		}
		n[name] = encodeValue(fv, opts, seen)
	}
	return n
}

func hasExplicitTag(f reflect.StructField) bool {
	tag := f.Tag.Get("form")
	if tag == "" {
		tag = f.Tag.Get("json")
	}
	if tag == "" {
		return false
	}
	return strings.SplitN(tag, ",", 2)[0] != ""
}

func shouldPromote(f reflect.StructField) bool {
	return f.Anonymous && !hasExplicitTag(f)
}

func collectFields(t reflect.Type) []encoderField {
	type queueItem struct {
		typ   reflect.Type
		index []int
		depth int
	}
	type fieldCandidate struct {
		field  encoderField
		depth  int
		tagged bool
	}

	current := []queueItem{{typ: t}}
	visited := map[reflect.Type]bool{}
	candidatesByName := map[string][]fieldCandidate{}
	nameOrder := []string{}

	for len(current) > 0 {
		var next []queueItem
		for _, item := range current {
			if visited[item.typ] {
				continue
			}
			visited[item.typ] = true

			for i := 0; i < item.typ.NumField(); i++ {
				f := item.typ.Field(i)
				k, oe := fieldInfo(f)
				if k == omittedKey {
					continue
				}

				idx := make([]int, len(item.index)+1)
				copy(idx, item.index)
				idx[len(item.index)] = i

				if shouldPromote(f) {
					ft := f.Type
					if ft.Kind() == reflect.Ptr {
						ft = ft.Elem()
					}
					if ft.Kind() == reflect.Struct && !isLeafStruct(ft) {
						next = append(next, queueItem{typ: ft, index: idx, depth: item.depth + 1})
						continue
					}
				}

				tagged := hasExplicitTag(f)
				fc := fieldCandidate{
					field: encoderField{
						index:     idx,
						name:      k,
						omitempty: oe,
						tagged:    tagged,
					},
					depth:  item.depth,
					tagged: tagged,
				}

				if _, exists := candidatesByName[k]; !exists {
					nameOrder = append(nameOrder, k)
				}
				candidatesByName[k] = append(candidatesByName[k], fc)
			}
		}

		current = next
	}

	// Resolve conflicts
	var result []encoderField
	for _, name := range nameOrder {
		cands := candidatesByName[name]
		if len(cands) == 1 {
			result = append(result, cands[0].field)
			continue
		}

		// Multiple candidates: keep only those at minimum depth
		minDepth := cands[0].depth
		for _, c := range cands[1:] {
			if c.depth < minDepth {
				minDepth = c.depth
			}
		}
		var filtered []fieldCandidate
		for _, c := range cands {
			if c.depth == minDepth {
				filtered = append(filtered, c)
			}
		}

		if len(filtered) == 1 {
			result = append(result, filtered[0].field)
			continue
		}

		// Still multiple at same depth: keep only tagged ones
		var tagged []fieldCandidate
		for _, c := range filtered {
			if c.tagged {
				tagged = append(tagged, c)
			}
		}

		if len(tagged) == 1 {
			result = append(result, tagged[0].field)
			continue
		}

		// Still multiple or none tagged: ambiguous, omit entirely
	}

	return result
}

func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

func isLeafStruct(ft reflect.Type) bool {
	if ft.ConvertibleTo(timeType) || ft.ConvertibleTo(urlType) {
		return true
	}
	return ft.Implements(textMarshalerType) || reflect.PtrTo(ft).Implements(textMarshalerType)
}

func encodeMap(v reflect.Value, opts encOpts, seen map[uintptr]bool) interface{} {
	n := node{}
	for _, i := range v.MapKeys() {
		k := getString(encodeValue(i, opts, seen))
		n[k] = encodeValue(v.MapIndex(i), opts, seen)
	}
	return n
}

func encodeArray(v reflect.Value, opts encOpts, seen map[uintptr]bool) interface{} {
	n := node{}
	for i := 0; i < v.Len(); i++ {
		n[strconv.Itoa(i)] = encodeValue(v.Index(i), opts, seen)
	}
	return n
}

func encodeSlice(v reflect.Value, opts encOpts, seen map[uintptr]bool) interface{} {
	t := v.Type()
	if t.Elem().Kind() == reflect.Uint8 {
		return string(v.Bytes()) // Encode byte slices as a single string by default.
	}
	n := node{}
	for i := 0; i < v.Len(); i++ {
		n[strconv.Itoa(i)] = encodeValue(v.Index(i), opts, seen)
	}
	return n
}

func encodeTime(v reflect.Value) string {
	t := v.Convert(timeType).Interface().(time.Time)
	if t.Year() == 0 && (t.Month() == 0 || t.Month() == 1) && (t.Day() == 0 || t.Day() == 1) {
		return t.Format("15:04:05.999999999Z07:00")
	} else if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02T15:04:05.999999999Z07:00")
}

func encodeURL(v reflect.Value) string {
	u := v.Convert(urlType).Interface().(url.URL)
	return u.String()
}

func encodeBasic(v reflect.Value) string {
	t := v.Type()
	switch k := t.Kind(); k {
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'g', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64)
	case reflect.Complex64, reflect.Complex128:
		s := fmt.Sprintf("%g", v.Complex())
		return strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
	case reflect.String:
		return v.String()
	}
	panic(errKind(KindUnsupported, t.String()+" has unsupported kind "+t.Kind().String()))
}

func isEmptyValue(v reflect.Value) bool {
	switch t := v.Type(); v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Struct:
		if t.ConvertibleTo(timeType) {
			return v.Convert(timeType).Interface().(time.Time).IsZero()
		}
		return reflect.DeepEqual(v, reflect.Zero(t))
	}
	return false
}

// canIndexOrdinally returns whether a value contains an ordered sequence of elements.
func canIndexOrdinally(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	switch t := v.Type(); t.Kind() {
	case reflect.Ptr, reflect.Interface:
		return canIndexOrdinally(v.Elem())
	case reflect.Slice, reflect.Array:
		return true
	}
	return false
}

func fieldInfo(f reflect.StructField, tagName ...string) (k string, oe bool) {
	_tagName := "form"
	if len(tagName) > 0 {
		_tagName = tagName[0]
	}
	if f.PkgPath != "" { // Skip private fields.
		return omittedKey, oe
	}

	k = f.Name
	tag := f.Tag.Get(_tagName)
	if tag == "" {
		if len(tagName) == 0 && _tagName != "json" {
			return fieldInfo(f, "json") // using json as secondary
		} else {
			return k, oe
		}
	}

	ps := strings.SplitN(tag, ",", 2)
	if ps[0] != "" {
		k = ps[0]
	}
	if len(ps) == 2 {
		oe = ps[1] == "omitempty"
	}
	return k, oe
}

func findField(v reflect.Value, n string, ignoreCase bool, keyFn func(string) string) (reflect.Value, bool) {
	t := v.Type()
	l := v.NumField()

	var lowerN string
	caseInsensitiveMatch := -1
	if ignoreCase {
		lowerN = strings.ToLower(n)
	}

	// First try named fields.
	for i := 0; i < l; i++ {
		f := t.Field(i)
		k, _ := fieldInfo(f)
		if k == omittedKey {
			continue
		}
		if keyFn != nil && !hasExplicitTag(f) {
			k = keyFn(k)
		}
		if n == k {
			return v.Field(i), true
		} else if ignoreCase && lowerN == strings.ToLower(k) {
			caseInsensitiveMatch = i
		}
	}

	// If no exact match was found try case insensitive match.
	if caseInsensitiveMatch != -1 {
		return v.Field(caseInsensitiveMatch), true
	}

	// Then try anonymous (embedded) fields.
	for i := 0; i < l; i++ {
		f := t.Field(i)
		k, _ := fieldInfo(f)
		if k == omittedKey || !f.Anonymous { // || k != "" ?
			continue
		}
		fv := v.Field(i)
		fk := fv.Kind()
		for fk == reflect.Ptr || fk == reflect.Interface {
			fv = fv.Elem()
			fk = fv.Kind()
		}

		if fk != reflect.Struct {
			continue
		}
		if ev, ok := findField(fv, n, ignoreCase, keyFn); ok {
			return ev, true
		}
	}

	return reflect.Value{}, false
}

var (
	stringType        = reflect.TypeOf(string(""))
	stringMapType     = reflect.TypeOf(map[string]interface{}{})
	timeType          = reflect.TypeOf(time.Time{})
	timePtrType       = reflect.TypeOf(&time.Time{})
	urlType           = reflect.TypeOf(url.URL{})
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

func skipTextMarshalling(t reflect.Type) bool {
	/*// Skip time.Time because its text unmarshaling is overly rigid:
	return t == timeType || t == timePtrType*/
	// Skip time.Time & convertibles because its text unmarshaling is overly rigid:
	return t.ConvertibleTo(timeType) || t.ConvertibleTo(timePtrType)
}

func unmarshalValue(v reflect.Value, x interface{}) bool {
	if skipTextMarshalling(v.Type()) {
		return false
	}

	tu, ok := v.Interface().(encoding.TextUnmarshaler)
	if !ok && !v.CanAddr() {
		return false
	} else if !ok {
		return unmarshalValue(v.Addr(), x)
	}

	s := getString(x)
	if err := tu.UnmarshalText([]byte(s)); err != nil {
		panic(err)
	}
	return true
}

func marshalValue(v reflect.Value) (string, bool) {
	if skipTextMarshalling(v.Type()) {
		return "", false
	}

	tm, ok := v.Interface().(encoding.TextMarshaler)
	if !ok && !v.CanAddr() {
		return "", false
	} else if !ok {
		return marshalValue(v.Addr())
	}

	bs, err := tm.MarshalText()
	if err != nil {
		panic(err)
	}
	return string(bs), true
}

// encOpts carries per-Encoder options through the encoding walk.
type encOpts struct {
	keepZeros bool
	omitEmpty bool
	hexColors bool
	keyFn     func(string) string
}
