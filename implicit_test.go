// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"reflect"
	"testing"
)

// Implicit indexing lets a repeated key stand in for an ordinal one: at the
// first level for a top-level slice/array destination, and at the last level
// via the "_" placeholder. These cases went untested for years (the removed
// "add tests for implicit indexing" TODO), which hid a bug where the second
// and later values of a repeated key were assigned corrupted, compounding
// keys — sparse, gap-riddled slices at the last level and outright decode
// failures at the first.

func TestImplicitFirstLevel(t *testing.T) {
	// A repeated key on a top-level slice indexes ordinally by value position.
	var dst []struct {
		Bar string `form:"bar"`
	}
	if err := DecodeString(&dst, "bar=A&bar=B&bar=C"); err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B", "C"}
	if len(dst) != len(want) {
		t.Fatalf("got %+v, want three elements", dst)
	}
	for i, w := range want {
		if dst[i].Bar != w {
			t.Errorf("elem %d: got %q, want %q", i, dst[i].Bar, w)
		}
	}
}

func TestImplicitLastLevel(t *testing.T) {
	// The "_" placeholder indexes the last level ordinally; three or more
	// values previously landed at indices 0, 1, 12, 123, … (digit
	// concatenation), producing a huge sparse slice.
	var dst struct {
		Bar []string `form:"bar"`
	}
	if err := DecodeString(&dst, "bar._=A&bar._=B&bar._=C&bar._=D"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"A", "B", "C", "D"}; !reflect.DeepEqual(dst.Bar, want) {
		t.Errorf("got %+v, want %+v", dst.Bar, want)
	}
}

func TestImplicitLastLevelNested(t *testing.T) {
	var dst struct {
		A struct {
			B []string `form:"b"`
		} `form:"a"`
	}
	if err := DecodeString(&dst, "a.b._=x&a.b._=y&a.b._=z"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"x", "y", "z"}; !reflect.DeepEqual(dst.A.B, want) {
		t.Errorf("got %+v, want %+v", dst.A.B, want)
	}
}

func TestImplicitIndexingRoundTrips(t *testing.T) {
	// Values a slice encodes to must decode back to the same slice; the
	// encoder emits explicit indices, which the decoder honors identically.
	type doc struct {
		Items []string `form:"items"`
	}
	src := doc{Items: []string{"one", "two", "three", "four"}}
	s, err := EncodeToString(&src)
	if err != nil {
		t.Fatal(err)
	}
	var dst doc
	if err := DecodeString(&dst, s); err != nil {
		t.Fatalf("re-decode of %q: %v", s, err)
	}
	if !reflect.DeepEqual(dst, src) {
		t.Errorf("round trip via %q: got %+v, want %+v", s, dst, src)
	}
}

func TestImplicitSingleValueUnchanged(t *testing.T) {
	// The single-value paths must be byte-for-byte unaffected by the fix.
	var slice []struct {
		Bar string `form:"bar"`
	}
	if err := DecodeString(&slice, "bar=solo"); err != nil {
		t.Fatal(err)
	}
	if len(slice) != 1 || slice[0].Bar != "solo" {
		t.Errorf("got %+v", slice)
	}
	var last struct {
		Bar []string `form:"bar"`
	}
	if err := DecodeString(&last, "bar._=solo"); err != nil {
		t.Fatal(err)
	}
	if len(last.Bar) != 1 || last.Bar[0] != "solo" {
		t.Errorf("got %+v", last.Bar)
	}
}
