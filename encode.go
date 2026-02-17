// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"encoding"
	"errors"
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
	return &Encoder{w, defaultDelimiter, defaultEscape, false, false}
}

// Encoder provides a way to encode to a Writer.
type Encoder struct {
	w io.Writer
	d rune
	e rune
	z bool
	o bool
}

// DelimitWith sets r as the delimiter used for composite keys by Encoder e and returns the latter; it is '.' by default.
func (e *Encoder) DelimitWith(r rune) *Encoder {
	e.d = r
	return e
}

// EscapeWith sets r as the escape used for delimiters (and to escape itself) by Encoder e and returns the latter; it is '\\' by default.
func (e *Encoder) EscapeWith(r rune) *Encoder {
	e.e = r
	return e
}

// KeepZeros sets whether Encoder e should keep zero (default) values in their literal form when encoding, and returns the former; by default zero values are not kept, but are rather encoded as the empty string.
func (e *Encoder) KeepZeros(z bool) *Encoder {
	e.z = z
	return e
}

// OmitEmpty sets whether Encoder e should omit empty (zero) struct fields during encoding, and returns the former; this is equivalent to having ",omitempty" on every field. By default, empty fields are included.
func (e *Encoder) OmitEmpty(o bool) *Encoder {
	e.o = o
	return e
}

// Encode encodes dst as form and writes it out using the Encoder's Writer.
func (e Encoder) Encode(dst interface{}) error {
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v, e.z, e.o)
	if err != nil {
		return err
	}
	s := n.values(e.d, e.e).Encode()
	l, err := io.WriteString(e.w, s)
	switch {
	case err != nil:
		return err
	case l != len(s):
		return errors.New("could not write data completely")
	}
	return nil
}

// EncodeToString encodes dst as a form and returns it as a string.
func EncodeToString(dst interface{}, needEmptyValue ...bool) (string, error) {
	v := reflect.ValueOf(dst)
	z := false
	if len(needEmptyValue) != 0 {
		z = needEmptyValue[0]
	}
	n, err := encodeToNode(v, z, false)
	if err != nil {
		return "", err
	}
	vs := n.values(defaultDelimiter, defaultEscape)
	return vs.Encode(), nil
}

// EncodeToValues encodes dst as a form and returns it as Values.
func EncodeToValues(dst interface{}, needEmptyValue ...bool) (url.Values, error) {
	v := reflect.ValueOf(dst)
	z := false
	if len(needEmptyValue) != 0 {
		z = needEmptyValue[0]
	}
	n, err := encodeToNode(v, z, false)
	if err != nil {
		return nil, err
	}
	vs := n.values(defaultDelimiter, defaultEscape)
	return vs, nil
}

func encodeToNode(v reflect.Value, z bool, o bool) (n node, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()
	return getNode(encodeValue(v, z, o)), nil
}

func encodeValue(v reflect.Value, z bool, o bool) interface{} {
	t := v.Type()
	k := v.Kind()

	if s, ok := marshalValue(v); ok {
		return s
	} else if !z && isEmptyValue(v) {
		return "" // Treat the zero value as the empty string.
	}

	switch k {
	case reflect.Ptr, reflect.Interface:
		return encodeValue(v.Elem(), z, o)
	case reflect.Struct:
		if t.ConvertibleTo(timeType) {
			return encodeTime(v)
		} else if t.ConvertibleTo(urlType) {
			return encodeURL(v)
		}
		return encodeStruct(v, z, o)
	case reflect.Slice:
		return encodeSlice(v, z, o)
	case reflect.Array:
		return encodeArray(v, z, o)
	case reflect.Map:
		return encodeMap(v, z, o)
	case reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer, reflect.Chan, reflect.Func:
		panic(t.String() + " has unsupported kind " + t.Kind().String())
	default:
		return encodeBasic(v)
	}
}

type encoderField struct {
	index     []int
	name      string
	omitempty bool
}

func encodeStruct(v reflect.Value, z bool, o bool) interface{} {
	fields := collectFields(v.Type())
	n := node{}
	for _, f := range fields {
		fv := fieldByIndex(v, f.index)
		if !fv.IsValid() {
			continue
		}
		if (o || f.omitempty) && isEmptyValue(fv) {
			continue
		}
		n[f.name] = encodeValue(fv, z, o)
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

func encodeMap(v reflect.Value, z bool, o bool) interface{} {
	n := node{}
	for _, i := range v.MapKeys() {
		k := getString(encodeValue(i, z, o))
		n[k] = encodeValue(v.MapIndex(i), z, o)
	}
	return n
}

func encodeArray(v reflect.Value, z bool, o bool) interface{} {
	n := node{}
	for i := 0; i < v.Len(); i++ {
		n[strconv.Itoa(i)] = encodeValue(v.Index(i), z, o)
	}
	return n
}

func encodeSlice(v reflect.Value, z bool, o bool) interface{} {
	t := v.Type()
	if t.Elem().Kind() == reflect.Uint8 {
		return string(v.Bytes()) // Encode byte slices as a single string by default.
	}
	n := node{}
	for i := 0; i < v.Len(); i++ {
		n[strconv.Itoa(i)] = encodeValue(v.Index(i), z, o)
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
	panic(t.String() + " has unsupported kind " + t.Kind().String())
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

func findField(v reflect.Value, n string, ignoreCase bool) (reflect.Value, bool) {
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
		} else if n == k {
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
		if ev, ok := findField(fv, n, ignoreCase); ok {
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
