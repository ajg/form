// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package multipart decodes multipart/form-data — as submitted by HTML forms
// containing file inputs — into Go structs, using github.com/ajg/form to
// decode the value fields and mapping file parts onto struct fields by name.
//
// Value fields follow form's decoding rules (struct tags, nesting, custom
// delimiters, and so on). File fields are matched by their top-level part
// name — via the `form` or `json` struct tag, or the field name — and may be
// declared as any of:
//
//	*multipart.FileHeader   // lazily read: the first file sent under the key
//	[]*multipart.FileHeader // lazily read: every file sent under the key
//	[]byte                  // eagerly read: contents of the first file
//	[][]byte                // eagerly read: contents of every file
//
// The *multipart.FileHeader forms hand over mime/multipart's own handle: the
// caller Opens (and Closes) the file itself and may stream it, so no bound is
// imposed here. The []byte forms read the file into memory during decoding,
// bounded by Decoder.MaxFileSize and Decoder.MaxFiles.
//
// Like form, decoding is strict by default: a value or file whose key matches
// no destination field is an error. Real browser submissions often include
// fields that aren't interesting to model (CSRF tokens, submit-button names);
// either add matching struct fields or opt out of strictness with
// NewDecoder().IgnoreUnknownKeys(true).
package multipart

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/ajg/form"
)

const (
	// The Decoder bounds how much file data it will read into memory for
	// []byte and [][]byte fields, so a large upload cannot OOM the process
	// merely by being decoded. builtinMaxFileSize is the per-file bound and
	// builtinMaxFiles the per-field file-count bound used when the Decoder is
	// not configured otherwise; see Decoder.MaxFileSize and Decoder.MaxFiles.
	// These bounds do not apply to *multipart.FileHeader fields, which are
	// read lazily by the caller.
	builtinMaxFileSize = 10 << 20 // 10 MiB
	builtinMaxFiles    = 1000
)

var (
	fileHeaderPtr   = reflect.TypeOf((*multipart.FileHeader)(nil))
	fileHeaderSlice = reflect.TypeOf([]*multipart.FileHeader(nil))
	bytesValue      = reflect.TypeOf([]byte(nil))
	bytesSlice      = reflect.TypeOf([][]byte(nil))
)

// Decoder decodes multipart/form-data into a struct. The zero configuration
// (NewDecoder) is strict and bounded; see the methods for the knobs.
type Decoder struct {
	fd            *form.Decoder
	ignoreUnknown bool
	maxFileSize   int64
	maxFiles      int
}

// NewDecoder returns a new multipart Decoder.
func NewDecoder() *Decoder {
	return &Decoder{fd: form.NewDecoder(nil)}
}

// DelimitWith sets r as the delimiter used for composite value keys and
// returns the Decoder; it is '.' by default.
func (d *Decoder) DelimitWith(r rune) *Decoder {
	d.fd.DelimitWith(r)
	return d
}

// EscapeWith sets r as the escape used for delimiters (and to escape itself)
// in value keys and returns the Decoder; it is '\\' by default.
func (d *Decoder) EscapeWith(r rune) *Decoder {
	d.fd.EscapeWith(r)
	return d
}

// IgnoreUnknownKeys, if set to true, makes the Decoder silently skip values
// and files whose keys match no destination field, instead of returning an
// error. It is false by default, matching form; setting it to true is the
// pragmatic choice for raw browser submissions, which routinely carry fields
// (CSRF tokens, submit-button names) that the destination doesn't model.
func (d *Decoder) IgnoreUnknownKeys(ignoreUnknown bool) *Decoder {
	d.ignoreUnknown = ignoreUnknown
	d.fd.IgnoreUnknownKeys(ignoreUnknown)
	return d
}

// IgnoreCase, if set to true, makes the Decoder try to set value fields even
// if the case of the key does not match. It applies to value fields only;
// file fields are always matched exactly.
func (d *Decoder) IgnoreCase(ignoreCase bool) *Decoder {
	d.fd.IgnoreCase(ignoreCase)
	return d
}

// MaxSize corresponds to form's Decoder.MaxSize, bounding how large a slice
// the value decoder will grow in response to an explicit index in the input.
func (d *Decoder) MaxSize(maxSize int) *Decoder {
	d.fd.MaxSize(maxSize)
	return d
}

// MaxDepth corresponds to form's Decoder.MaxDepth, bounding the key-path
// nesting depth the value decoder will parse.
func (d *Decoder) MaxDepth(maxDepth int) *Decoder {
	d.fd.MaxDepth(maxDepth)
	return d
}

// MaxFileSize overrides how many bytes of a single file the Decoder will read
// into memory for a []byte or [][]byte field; a larger file is an error. A
// value > 0 sets the bound; a value < 0 disables it (trusted input only); the
// zero value uses the built-in default of 10 MiB. It has no effect on
// *multipart.FileHeader fields, which the caller reads lazily.
func (d *Decoder) MaxFileSize(maxFileSize int64) *Decoder {
	d.maxFileSize = maxFileSize
	return d
}

// MaxFiles overrides how many files the Decoder will accept under a single
// key for a slice-valued file field; more is an error. A value > 0 sets the
// bound; a value < 0 disables it; the zero value uses the built-in default of
// 1000.
func (d *Decoder) MaxFiles(maxFiles int) *Decoder {
	d.maxFiles = maxFiles
	return d
}

// fileSizeLimit reports the largest single file the Decoder will read into
// memory. A negative result means unbounded.
func (d *Decoder) fileSizeLimit() int64 {
	switch {
	case d.maxFileSize < 0:
		return -1 // Explicitly unbounded (trusted input).
	case d.maxFileSize > 0:
		return d.maxFileSize // Explicit bound.
	default:
		return builtinMaxFileSize
	}
}

// filesLimit reports the largest number of files the Decoder will accept for
// a slice-valued file field. A negative result means unbounded.
func (d *Decoder) filesLimit() int {
	switch {
	case d.maxFiles < 0:
		return -1 // Explicitly unbounded (trusted input).
	case d.maxFiles > 0:
		return d.maxFiles // Explicit bound.
	default:
		return builtinMaxFiles
	}
}

// DecodeForm decodes the already-parsed multipart form mf into dst, which
// must be a non-nil pointer to a struct.
func (d *Decoder) DecodeForm(dst interface{}, mf *multipart.Form) error {
	if mf == nil {
		return errors.New("form/multipart: nil *multipart.Form")
	}
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("form/multipart: dst must be a non-nil pointer to a struct")
	}
	if v.Elem().Kind() != reflect.Struct {
		return errors.New("form/multipart: dst must point to a struct")
	}
	if len(mf.Value) > 0 {
		if err := d.fd.DecodeValues(dst, url.Values(mf.Value)); err != nil {
			return err
		}
	}
	return d.setFiles(v.Elem(), mf.File)
}

// DecodeRequest parses r's body as multipart/form-data — via
// (*http.Request).ParseMultipartForm, buffering up to maxMemory bytes of file
// data in memory and the remainder in temporary files — and decodes the
// result into dst, which must be a non-nil pointer to a struct.
func (d *Decoder) DecodeRequest(dst interface{}, r *http.Request, maxMemory int64) error {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return err
	}
	return d.DecodeForm(dst, r.MultipartForm)
}

// DecodeForm decodes the already-parsed multipart form mf into dst using a
// default (strict, bounded) Decoder.
func DecodeForm(dst interface{}, mf *multipart.Form) error {
	return NewDecoder().DecodeForm(dst, mf)
}

// DecodeRequest parses r's body as multipart/form-data and decodes the result
// into dst using a default (strict, bounded) Decoder.
func DecodeRequest(dst interface{}, r *http.Request, maxMemory int64) error {
	return NewDecoder().DecodeRequest(dst, r, maxMemory)
}

// setFiles maps each file part onto the matching struct field of v.
func (d *Decoder) setFiles(v reflect.Value, files map[string][]*multipart.FileHeader) error {
	fields := fieldsByName(v)
	for name, fhs := range files {
		if len(fhs) == 0 {
			continue
		}
		fv, ok := fields[name]
		if !ok {
			if d.ignoreUnknown {
				continue
			}
			return fmt.Errorf("form/multipart: unknown file key %q; set Decoder.IgnoreUnknownKeys(true) to skip unmodeled fields", name)
		}
		switch fv.Type() {
		case fileHeaderPtr:
			fv.Set(reflect.ValueOf(fhs[0]))
		case fileHeaderSlice:
			if limit := d.filesLimit(); limit >= 0 && len(fhs) > limit {
				return tooManyFiles(name, len(fhs), limit)
			}
			fv.Set(reflect.ValueOf(fhs))
		case bytesValue:
			bs, err := d.readFile(fhs[0])
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(bs))
		case bytesSlice:
			if limit := d.filesLimit(); limit >= 0 && len(fhs) > limit {
				return tooManyFiles(name, len(fhs), limit)
			}
			bss := make([][]byte, 0, len(fhs))
			for _, fh := range fhs {
				bs, err := d.readFile(fh)
				if err != nil {
					return err
				}
				bss = append(bss, bs)
			}
			fv.Set(reflect.ValueOf(bss))
		default:
			if d.ignoreUnknown {
				continue
			}
			return fmt.Errorf("form/multipart: cannot decode file %q into field of type %v", name, fv.Type())
		}
	}
	return nil
}

// readFile reads a single file part fully into memory, bounded by
// Decoder.MaxFileSize.
func (d *Decoder) readFile(fh *multipart.FileHeader) ([]byte, error) {
	limit := d.fileSizeLimit()
	if limit >= 0 && fh.Size > limit {
		return nil, tooLarge(fh, limit)
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := io.Reader(f)
	if limit >= 0 {
		// Trust the actual bytes, not just the reported Size.
		r = io.LimitReader(f, limit+1)
	}
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && int64(len(bs)) > limit {
		return nil, tooLarge(fh, limit)
	}
	return bs, nil
}

func tooLarge(fh *multipart.FileHeader, limit int64) error {
	return fmt.Errorf("form/multipart: file %q exceeds the maximum size (%d bytes) read into memory; see Decoder.MaxFileSize, or use a *multipart.FileHeader field to stream it", fh.Filename, limit)
}

func tooManyFiles(name string, n, limit int) error {
	return fmt.Errorf("form/multipart: key %q carries %d files, exceeding the maximum (%d); see Decoder.MaxFiles", name, n, limit)
}

// fieldsByName indexes the settable, exported fields of struct value v by
// their decoded name (per fieldName).
func fieldsByName(v reflect.Value) map[string]reflect.Value {
	fields := map[string]reflect.Value{}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue // Unexported.
		}
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}
		name := fieldName(sf)
		if name == "-" {
			continue
		}
		fields[name] = fv
	}
	return fields
}

// fieldName reports the key under which struct field sf is decoded: the name
// in its `form` tag, else its `json` tag, else the field's own name.
func fieldName(sf reflect.StructField) string {
	for _, key := range [2]string{"form", "json"} {
		if tag := sf.Tag.Get(key); tag != "" {
			if name := strings.SplitN(tag, ",", 2)[0]; name != "" {
				return name
			}
		}
	}
	return sf.Name
}
