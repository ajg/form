// Copyright 2013 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

// NewEncoder returns a new form encoder.
func NewEncoder(w io.Writer) *encoder {
	return &encoder{w}
}

// encoder provides a way to encode to a Writer.
type encoder struct {
	w io.Writer
}

// Encode encodes dst as form and writes it out using the encoder's Writer.
func (e encoder) Encode(dst interface{}) error {
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v)
	if err != nil {
		return err
	}
	s := n.Values().Encode()
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
func EncodeToString(dst interface{}) (string, error) {
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v)
	if err != nil {
		return "", err
	}
	return n.Values().Encode(), nil
}

// EncodeToValues encodes dst as a form and returns it as Values.
func EncodeToValues(dst interface{}) (url.Values, error) {
	v := reflect.ValueOf(dst)
	n, err := encodeToNode(v)
	if err != nil {
		return nil, err
	}
	return n.Values(), nil
}

func encodeToNode(v reflect.Value) (n node, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()
	return getNode(encodeValue(v)), nil
}

func encodeValue(v reflect.Value) interface{} {
	t := v.Type()
	k := v.Kind()

	if isEmptyValue(v) {
		return "" // Treat the zero value as the empty string.
	}

	switch k {
	case reflect.Ptr, reflect.Interface:
		return encodeValue(v.Elem())
	case reflect.Struct:
		if t.ConvertibleTo(timeType) {
			return encodeTime(v)
		} else {
			return encodeStruct(v)
		}
	case reflect.Slice:
		return encodeSlice(v)
	case reflect.Array:
		return encodeArray(v)
	case reflect.Map:
		return encodeMap(v)
	case reflect.Invalid, reflect.Uintptr, reflect.UnsafePointer,
		reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func:
		panic(t.String() + " has unsupported kind " + t.Kind().String())
	default:
		return encodeBasic(v)
	}
}

func encodeStruct(v reflect.Value) interface{} {
	t := v.Type()
	n := node{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		k, oe := fieldInfo(f)

		if k == "-" {
			continue
		} else if fv := v.Field(i); oe && isEmptyValue(fv) {
			delete(n, k)
		} else {
			n[k] = encodeValue(fv)
		}
	}
	return n
}

func encodeMap(v reflect.Value) interface{} {
	n := node{}
	for _, k := range v.MapKeys() {
		n[encodeBasic(k)] = encodeValue(v.MapIndex(k)) // TODO: encodeValue.
	}
	return n
}

func encodeArray(v reflect.Value) interface{} {
	n := node{}
	for i := 0; i < v.Len(); i++ {
		n[strconv.Itoa(i)] = encodeValue(v.Index(i))
	}
	return n
}

func encodeSlice(v reflect.Value) interface{} {
	t := v.Type()
	if t.Elem().Kind() == reflect.Uint8 {
		return string(v.Bytes()) // Encode byte slices as a single string by default.
	}
	n := node{}
	for i := 0; i < v.Len(); i++ {
		n[strconv.Itoa(i)] = encodeValue(v.Index(i))
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

func encodeBasic(v reflect.Value) string {
	t := v.Type()
	if isEmptyValue(v) {
		return "" // Treat the zero value as the empty string.
	}

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
	case reflect.String:
		return v.String()
	}
	panic(t.String() + " has unsupported kind " + t.Kind().String())
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
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
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Struct:
		if t := v.Type(); t.ConvertibleTo(timeType) {
			return v.Convert(timeType).Interface().(time.Time).IsZero()
		} else {
			return reflect.DeepEqual(v, reflect.Zero(t))
		}
	}
	return false
}

func canIndex(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	switch t := v.Type(); t.Kind() {
	case reflect.Ptr, reflect.Interface:
		return canIndex(v.Elem())
	case reflect.Slice, reflect.Array:
		return true
	}
	return false
}

var (
	timeType      = reflect.TypeOf(time.Time{})
	stringType    = reflect.TypeOf(string(""))
	stringMapType = reflect.TypeOf(map[string]interface{}{})
)
