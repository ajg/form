// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"errors"
	"testing"
)

// These tests pin the contracts for degenerate arguments to the public
// entry points: everything is a typed error with a meaningful Kind, never a
// panic, and the benign cases are explicitly blessed.
func TestInputContracts(t *testing.T) {
	kindOf2 := func(err error) Kind {
		var fe *Error
		if err == nil || !errors.As(err, &fe) {
			t.Fatalf("expected typed error, got %v", err)
		}
		return fe.Kind
	}

	var m map[string]interface{}

	if k := kindOf2(DecodeString(nil, "a=1")); k != KindUnsupported {
		t.Errorf("nil dst: got %q", k)
	}
	if _, err := EncodeToString(nil); kindOf2(err) != KindUnsupported {
		t.Errorf("nil src: got %v", err)
	}
	err := NewDecoder(nil).DelimitWith('\\').DecodeString(&m, "a=1")
	if kindOf2(err) != KindUnsupported {
		t.Errorf("decode delimiter==escape: got %v", err)
	}
	var sb stringWriter
	err = NewEncoder(&sb).DelimitWith('\\').Encode(&struct{ A string }{"x"})
	if kindOf2(err) != KindUnsupported {
		t.Errorf("encode delimiter==escape: got %v", err)
	}
	// The free functions take the delimiter and escape directly; they must
	// enforce the same contract rather than garble silently.
	if _, err := EncodeToStringWith(&struct{ A string }{"x"}, '.', '.', false); kindOf2(err) != KindUnsupported {
		t.Errorf("EncodeToStringWith delimiter==escape: got %v", err)
	}
	if _, err := EncodeToValuesWith(&struct{ A string }{"x"}, '\\', '\\', false); kindOf2(err) != KindUnsupported {
		t.Errorf("EncodeToValuesWith delimiter==escape: got %v", err)
	}

	// Blessed benign cases:
	if err := DecodeValues(&m, nil); err != nil {
		t.Errorf("nil url.Values is a no-op: %v", err)
	}
	var p *struct{ A string }
	if err := DecodeString(&p, "A=1"); err != nil || p == nil || p.A != "1" {
		t.Errorf("pointer-to-nil-pointer is allocated: %v %+v", err, p)
	}
	if s, err := EncodeToString(struct{ a int }{1}); err != nil || s != "" {
		t.Errorf("unexported-only struct encodes to empty: %q %v", s, err)
	}
}

func TestNewErrorf(t *testing.T) {
	e := NewErrorf(OpDecode, KindParse, nil, "bad %q at index %d", "x", 3)
	if e.Op != OpDecode || e.Kind != KindParse || e.Error() != `bad "x" at index 3` {
		t.Errorf("got Op=%q Kind=%q msg=%q", e.Op, e.Kind, e.Error())
	}
}
