// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

// kindOf decodes src into dst and reports the resulting *Error's Kind.
func kindOf(t *testing.T, dst interface{}, src string) Kind {
	t.Helper()
	err := DecodeString(dst, src)
	if err == nil {
		t.Fatalf("expected an error decoding %q", src)
	}
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatalf("expected *form.Error, got %T: %v", err, err)
	}
	if fe.Op != OpDecode {
		t.Errorf("Op: got %q, want %q", fe.Op, OpDecode)
	}
	return fe.Kind
}

func TestKinds(t *testing.T) {
	type dst struct {
		N  int       `form:"n"`
		S  []int     `form:"s"`
		A  [2]int    `form:"a"`
		Ch chan int  `form:"ch"`
		M  yesReader `form:"m"`
	}

	for _, c := range []struct {
		name string
		src  string
		want Kind
	}{
		{"syntax", "n=%zz", KindSyntax},
		{"parse", "n=abc", KindParse},
		{"unknown key", "nope=1", KindUnknownKey},
		{"index malformed", "s.x=1", KindIndex},
		{"index above array", "a.5=1", KindIndex},
		{"limit size", "s.999999999=1", KindLimit},
		{"limit depth", strings.Repeat("q.", 10001) + "z=1", KindLimit},
		{"unsupported", "ch=1", KindUnsupported},
	} {
		var d dst
		if got := kindOf(t, &d, c.src); got != c.want {
			t.Errorf("%s: got Kind %q, want %q", c.name, got, c.want)
		}
	}
}

func TestConflictingKeyPathsAreDeterministic(t *testing.T) {
	// "a=1&a.b=2" sets the key path "a" both as a scalar and as a composite.
	// Keys are split in sorted order, so the scalar is always absorbed and
	// the outcome never depends on map iteration order: struct destinations
	// always fail the same way (the absorbed composite cannot decode into
	// the struct), and map destinations always succeed with both values.
	for i := 0; i < 50; i++ {
		var d struct {
			A struct {
				B string `form:"b"`
			} `form:"a"`
		}
		err := DecodeString(&d, "a=1&a.b=2")
		var fe *Error
		if err == nil || !errors.As(err, &fe) || fe.Kind != KindParse {
			t.Fatalf("struct dst: got %v, want KindParse", err)
		}

		m := map[string]interface{}{}
		if err := DecodeString(&m, "a=1&a.b=2"); err != nil {
			t.Fatalf("map dst: unexpected error: %v", err)
		}
		inner, ok := m["a"].(map[string]interface{})
		if !ok || inner[""] != "1" || inner["b"] != "2" {
			t.Fatalf("map dst: got %v, want both values retained", m)
		}
	}
}

func TestKindCycleAndIO(t *testing.T) {
	type box struct {
		Self *box `form:"self"`
	}
	b := &box{}
	b.Self = b
	_, err := EncodeToString(b)
	var fe *Error
	if err == nil || !errors.As(err, &fe) || fe.Kind != KindCycle || fe.Op != OpEncode {
		t.Errorf("cycle: got %v", err)
	}

	if err := NewEncoder(failWriter{}).Encode(struct{ A int }{1}); err == nil {
		t.Error("io: expected error")
	} else if !errors.As(err, &fe) || fe.Kind != KindIO {
		t.Errorf("io: got %v with kind %q", err, fe.Kind)
	}
}

// TestKindUnclassified pins that external causes (e.g. a TextUnmarshaler
// failure) keep an empty Kind and remain reachable via Unwrap.
func TestKindUnclassified(t *testing.T) {
	var d struct {
		M yesReader `form:"m"`
	}
	err := DecodeString(&d, "m=x")
	var fe *Error
	if err == nil || !errors.As(err, &fe) {
		t.Fatalf("got %v", err)
	}
	if fe.Kind != Kind("") {
		t.Errorf("got Kind %q, want empty", fe.Kind)
	}
	if fe.Err == nil {
		t.Error("expected wrapped cause")
	}
}

// yesReader is a TextUnmarshaler that always fails, to exercise the
// unclassified path.
type yesReader struct{}

func (yesReader) UnmarshalText([]byte) error { return errors.New("nope") }

// TestSyntaxKindMatchesURLError pins that syntax errors wrap the underlying
// net/url error.
func TestSyntaxKindMatchesURLError(t *testing.T) {
	var d struct{}
	err := DecodeString(&d, "%zz=1")
	var fe *Error
	if err == nil || !errors.As(err, &fe) || fe.Kind != KindSyntax {
		t.Fatalf("got %v", err)
	}
	var ue url.EscapeError
	if !errors.As(err, &ue) {
		t.Errorf("underlying url.EscapeError not reachable: %v", fe.Err)
	}
}
