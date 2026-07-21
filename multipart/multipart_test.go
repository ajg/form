// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart

import (
	"bytes"
	"errors"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/ajg/form"
)

type target struct {
	Name   string                  `form:"name"`
	Age    int                     `form:"age"`
	Avatar *multipart.FileHeader   `form:"avatar"`
	Docs   []*multipart.FileHeader `form:"docs"`
}

type eagerTarget struct {
	Name   string   `form:"name"`
	Avatar []byte   `form:"avatar"`
	Docs   [][]byte `form:"docs"`
}

type field struct {
	name, value string
}

type file struct {
	key, name, content string
}

// writeMultipart builds a multipart/form-data body from vs and fs, returning
// the body and its boundary.
func writeMultipart(t *testing.T, vs []field, fs []file) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for _, v := range vs {
		if err := w.WriteField(v.name, v.value); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range fs {
		fw, err := w.CreateFormFile(f.key, f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return body, w.Boundary()
}

// parseForm turns a built body back into a *multipart.Form.
func parseForm(t *testing.T, body *bytes.Buffer, boundary string) *multipart.Form {
	t.Helper()
	mf, err := multipart.NewReader(body, boundary).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	return mf
}

func defaultBody(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	return writeMultipart(t,
		[]field{{"name", "Alice"}, {"age", "30"}},
		[]file{
			{"avatar", "avatar.png", "fake png bytes"},
			{"docs", "a.txt", "first doc"},
			{"docs", "b.txt", "second doc"},
		},
	)
}

func TestDecodeForm(t *testing.T) {
	body, boundary := defaultBody(t)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst target
	if err := DecodeForm(&dst, mf); err != nil {
		t.Fatal(err)
	}
	if dst.Name != "Alice" || dst.Age != 30 {
		t.Errorf("values: got (%q, %d), want (\"Alice\", 30)", dst.Name, dst.Age)
	}
	if dst.Avatar == nil || dst.Avatar.Filename != "avatar.png" {
		t.Errorf("avatar: got %+v, want avatar.png", dst.Avatar)
	}
	if len(dst.Docs) != 2 || dst.Docs[0].Filename != "a.txt" || dst.Docs[1].Filename != "b.txt" {
		t.Errorf("docs: got %+v, want [a.txt b.txt]", dst.Docs)
	}
}

func TestDecodeRequest(t *testing.T) {
	body, boundary := defaultBody(t)
	r := httptest.NewRequest("POST", "/upload", body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	var dst target
	if err := DecodeRequest(&dst, r, 1<<20); err != nil {
		t.Fatal(err)
	}
	defer r.MultipartForm.RemoveAll()
	if dst.Name != "Alice" || dst.Age != 30 || dst.Avatar == nil || len(dst.Docs) != 2 {
		t.Errorf("got %+v", dst)
	}
}

func TestDecodeFormEager(t *testing.T) {
	body, boundary := defaultBody(t)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst eagerTarget
	if err := NewDecoder().IgnoreUnknownKeys(true).DecodeForm(&dst, mf); err != nil {
		t.Fatal(err)
	}
	if string(dst.Avatar) != "fake png bytes" {
		t.Errorf("avatar: got %q", dst.Avatar)
	}
	if len(dst.Docs) != 2 || string(dst.Docs[0]) != "first doc" || string(dst.Docs[1]) != "second doc" {
		t.Errorf("docs: got %q", dst.Docs)
	}
}

func TestUnknownValueKey(t *testing.T) {
	body, boundary := writeMultipart(t,
		[]field{{"name", "Alice"}, {"age", "30"}, {"csrf_token", "abc123"}},
		nil,
	)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst target
	if err := DecodeForm(&dst, mf); err == nil {
		t.Error("strict: expected error for unknown value key, got nil")
	}
	dst = target{}
	if err := NewDecoder().IgnoreUnknownKeys(true).DecodeForm(&dst, mf); err != nil {
		t.Errorf("lenient: unexpected error: %v", err)
	} else if dst.Name != "Alice" {
		t.Errorf("lenient: got %+v", dst)
	}
}

func TestUnknownFileKey(t *testing.T) {
	body, boundary := writeMultipart(t,
		[]field{{"name", "Alice"}, {"age", "30"}},
		[]file{{"surprise", "s.txt", "unexpected"}},
	)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst target
	err := DecodeForm(&dst, mf)
	if err == nil {
		t.Error("strict: expected error for unknown file key, got nil")
	} else if !strings.Contains(err.Error(), `"surprise"`) {
		t.Errorf("strict: error should name the key: %v", err)
	}
	dst = target{}
	if err := NewDecoder().IgnoreUnknownKeys(true).DecodeForm(&dst, mf); err != nil {
		t.Errorf("lenient: unexpected error: %v", err)
	}
}

func TestUnsupportedFileField(t *testing.T) {
	body, boundary := writeMultipart(t, nil,
		[]file{{"name", "n.txt", "a file aimed at a string field"}},
	)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst target
	if err := DecodeForm(&dst, mf); err == nil {
		t.Error("strict: expected error for file into string field, got nil")
	}
	dst = target{}
	if err := NewDecoder().IgnoreUnknownKeys(true).DecodeForm(&dst, mf); err != nil {
		t.Errorf("lenient: unexpected error: %v", err)
	} else if dst.Name != "" {
		t.Errorf("lenient: string field should be untouched, got %q", dst.Name)
	}
}

func TestMaxFileSize(t *testing.T) {
	body, boundary := writeMultipart(t, nil,
		[]file{{"avatar", "big.png", strings.Repeat("x", 100)}},
	)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst eagerTarget
	if err := NewDecoder().MaxFileSize(99).DecodeForm(&dst, mf); err == nil {
		t.Error("expected error for file over MaxFileSize, got nil")
	}
	dst = eagerTarget{}
	if err := NewDecoder().MaxFileSize(100).DecodeForm(&dst, mf); err != nil {
		t.Errorf("file at exactly MaxFileSize should decode: %v", err)
	} else if len(dst.Avatar) != 100 {
		t.Errorf("got %d bytes, want 100", len(dst.Avatar))
	}

	// The bound must not apply to lazily-read *multipart.FileHeader fields.
	body, boundary = writeMultipart(t, nil,
		[]file{{"avatar", "big.png", strings.Repeat("x", 100)}},
	)
	mf = parseForm(t, body, boundary)
	defer mf.RemoveAll()
	var lazy target
	if err := NewDecoder().MaxFileSize(1).DecodeForm(&lazy, mf); err != nil {
		t.Errorf("FileHeader field should ignore MaxFileSize: %v", err)
	}
}

func TestMaxFiles(t *testing.T) {
	body, boundary := writeMultipart(t, nil, []file{
		{"docs", "a.txt", "a"},
		{"docs", "b.txt", "b"},
		{"docs", "c.txt", "c"},
	})
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst target
	if err := NewDecoder().MaxFiles(2).DecodeForm(&dst, mf); err == nil {
		t.Error("expected error for more files than MaxFiles, got nil")
	}
	dst = target{}
	if err := NewDecoder().MaxFiles(3).DecodeForm(&dst, mf); err != nil {
		t.Errorf("exactly MaxFiles files should decode: %v", err)
	} else if len(dst.Docs) != 3 {
		t.Errorf("got %d docs, want 3", len(dst.Docs))
	}
}

func TestDecodeFormErrors(t *testing.T) {
	if err := DecodeForm(&target{}, nil); err == nil {
		t.Error("nil form: expected error")
	}
	mf := &multipart.Form{}
	if err := DecodeForm(target{}, mf); err == nil {
		t.Error("non-pointer dst: expected error")
	}
	if err := DecodeForm((*target)(nil), mf); err == nil {
		t.Error("nil pointer dst: expected error")
	}
	s := "nope"
	if err := DecodeForm(&s, mf); err == nil {
		t.Error("pointer to non-struct dst: expected error")
	}
}

func TestTypedErrors(t *testing.T) {
	body, boundary := writeMultipart(t, nil, []file{{"surprise", "s.txt", "x"}})
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst target
	err := DecodeForm(&dst, mf)
	var fe *form.Error
	if err == nil || !errors.As(err, &fe) || fe.Op != form.OpDecode || fe.Kind != form.KindUnknownKey {
		t.Errorf("unknown file key: got %v, want *form.Error KindUnknownKey", err)
	}

	body, boundary = writeMultipart(t, nil, []file{{"avatar", "big.png", strings.Repeat("x", 100)}})
	mf = parseForm(t, body, boundary)
	defer mf.RemoveAll()
	var eager eagerTarget
	err = NewDecoder().IgnoreUnknownKeys(true).MaxFileSize(10).DecodeForm(&eager, mf)
	if err == nil || !errors.As(err, &fe) || fe.Kind != form.KindLimit {
		t.Errorf("oversized file: got %v, want KindLimit", err)
	}

	if err := DecodeForm(&dst, nil); err == nil || !errors.As(err, &fe) || fe.Kind != form.KindUnsupported {
		t.Errorf("nil form: got %v, want KindUnsupported", err)
	}
}

func TestKeysWithFilesAndValues(t *testing.T) {
	snake := func(s string) string {
		var b strings.Builder
		for i, r := range s {
			if unicode.IsUpper(r) && i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		}
		return b.String()
	}
	type profile struct {
		HomeCity    string                // value field via keyFn
		AvatarImage *multipart.FileHeader // file field via keyFn
		Tagged      []byte                `form:"exact"` // tags stay verbatim
	}
	body, boundary := writeMultipart(t,
		[]field{{"home_city", "lisbon"}},
		[]file{
			{"avatar_image", "a.png", "img"},
			{"exact", "t.bin", "tagged"},
		},
	)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst profile
	if err := NewDecoder().KeysWith(snake).DecodeForm(&dst, mf); err != nil {
		t.Fatal(err)
	}
	if dst.HomeCity != "lisbon" {
		t.Errorf("value via keyFn: got %+v", dst.HomeCity)
	}
	if dst.AvatarImage == nil || dst.AvatarImage.Filename != "a.png" {
		t.Errorf("file via keyFn: got %+v", dst.AvatarImage)
	}
	if string(dst.Tagged) != "tagged" {
		t.Errorf("tagged file: got %q", dst.Tagged)
	}
}

func TestKeysWithNilClears(t *testing.T) {
	snake := func(s string) string { return strings.ToLower(s) }
	body, boundary := writeMultipart(t, []field{{"HomeCity", "lisbon"}}, nil)
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst struct{ HomeCity string }
	d := NewDecoder().KeysWith(snake)
	d.KeysWith(nil)
	if err := d.DecodeForm(&dst, mf); err != nil || dst.HomeCity != "lisbon" {
		t.Errorf("nil should clear the mapper: err=%v v=%+v", err, dst)
	}
}

func TestNilRequest(t *testing.T) {
	var dst struct{ A string }
	err := DecodeRequest(&dst, nil, 1<<20)
	var fe *form.Error
	if err == nil || !errors.As(err, &fe) || fe.Kind != form.KindUnsupported {
		t.Errorf("nil request: got %v, want typed KindUnsupported (not a panic)", err)
	}
}

// TestTagResolutionMatchesForm pins that file-field naming follows form's
// exact tag rules: a present-but-unnamed `form` tag is authoritative (its
// json neighbor is NOT consulted), so value and file parts agree on keys.
func TestTagResolutionMatchesForm(t *testing.T) {
	type v struct {
		Foo *multipart.FileHeader `form:",omitempty" json:"bar"`
	}
	body, boundary := writeMultipart(t, nil, []file{{"Foo", "f.txt", "x"}})
	mf := parseForm(t, body, boundary)
	defer mf.RemoveAll()

	var dst v
	if err := DecodeForm(&dst, mf); err != nil {
		t.Fatalf("field-name key: %v", err)
	}
	if dst.Foo == nil {
		t.Error("file under the field name should match, as in form")
	}

	body, boundary = writeMultipart(t, nil, []file{{"bar", "f.txt", "x"}})
	mf = parseForm(t, body, boundary)
	defer mf.RemoveAll()
	dst = v{}
	if err := DecodeForm(&dst, mf); err == nil {
		t.Error("json name must NOT match when a form tag is present (parity with form)")
	}
}

func TestNotMultipartKind(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("a=1"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var dst struct{ A string }
	err := DecodeRequest(&dst, r, 1<<20)
	var fe *form.Error
	if err == nil || !errors.As(err, &fe) || fe.Kind != form.KindUnsupported {
		t.Errorf("non-multipart request: got %v, want KindUnsupported", err)
	}
	if !errors.Is(err, http.ErrNotMultipart) {
		t.Errorf("underlying http.ErrNotMultipart not reachable: %v", err)
	}
}

func TestMaxFileSizeMaxInt64(t *testing.T) {
	// The bound is unexceedable, so it must behave as if unbounded — not
	// overflow into silently reading files as empty.
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("doc", "a.txt")
	fw.Write([]byte("hello file"))
	w.Close()

	req, _ := http.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	var dst struct {
		Doc []byte `form:"doc"`
	}
	if err := DecodeRequest(&dst, req, 1<<20); err != nil {
		t.Fatal(err)
	}
	// Sanity of the fixture itself before the edge case:
	if string(dst.Doc) != "hello file" {
		t.Fatalf("fixture broken: %q", dst.Doc)
	}

	body.Reset()
	w = multipart.NewWriter(&body)
	fw, _ = w.CreateFormFile("doc", "a.txt")
	fw.Write([]byte("hello file"))
	w.Close()
	req, _ = http.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	dst.Doc = nil
	if err := NewDecoder().MaxFileSize(math.MaxInt64).DecodeRequest(&dst, req, 1<<20); err != nil {
		t.Fatal(err)
	}
	if string(dst.Doc) != "hello file" {
		t.Errorf("file silently truncated: got %q", dst.Doc)
	}
}
