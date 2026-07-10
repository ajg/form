// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"strconv"
	"strings"
	"testing"
)

type sliceTarget struct {
	Foo []int64 `form:"foo"`
}

// TestDecodeSliceLargeIndexIsBounded ensures a tiny payload carrying a very
// large explicit slice index does not force a huge allocation, but is rejected
// with an error. Regression test for a memory-exhaustion (DoS) vulnerability in
// decodeSlice.
func TestDecodeSliceLargeIndexIsBounded(t *testing.T) {
	var dst sliceTarget
	// ~15-byte payload that, unbounded, would allocate ~7.2 GB of int64.
	err := DecodeString(&dst, "foo.900000000=1")
	if err == nil {
		t.Fatalf("expected an error for an oversized index, got nil (len(Foo)=%d)", len(dst.Foo))
	}
	if !strings.Contains(err.Error(), "allowed size") {
		t.Fatalf("expected an allowed-size error, got: %v", err)
	}
	if len(dst.Foo) != 0 {
		t.Fatalf("expected no allocation, got len(Foo)=%d", len(dst.Foo))
	}
}

// TestDecodeSliceNegativeIndex ensures a negative explicit index is rejected
// cleanly rather than panicking deep in reflect.
func TestDecodeSliceNegativeIndex(t *testing.T) {
	var dst sliceTarget
	if err := DecodeString(&dst, "foo.-5=1"); err == nil {
		t.Fatalf("expected an error for a negative index, got nil")
	}
}

// TestDecodeSliceModestSparseWorks ensures ordinary sparse indexing within the
// floor continues to work with no configuration.
func TestDecodeSliceModestSparseWorks(t *testing.T) {
	var dst sliceTarget
	if err := DecodeString(&dst, "foo.5=42"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst.Foo) != 6 || dst.Foo[5] != 42 {
		t.Fatalf("unexpected result: %#v", dst.Foo)
	}
}

// TestDecodeSliceLargeLegitimateSliceWorks is the key guarantee behind the
// proportional bound: a genuinely large slice (far past the sparse floor)
// decodes with no configuration, because its elements are actually supplied.
// This is what prevents a fixed cap from someday rejecting real data.
func TestDecodeSliceLargeLegitimateSliceWorks(t *testing.T) {
	const count = 50000 // well beyond sliceGrowthFloor
	var b strings.Builder
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString("foo.")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("=1")
	}

	var dst sliceTarget
	if err := DecodeString(&dst, b.String()); err != nil {
		t.Fatalf("unexpected error decoding a large legitimate slice: %v", err)
	}
	if len(dst.Foo) != count {
		t.Fatalf("expected len(Foo)=%d, got %d", count, len(dst.Foo))
	}
}

// TestDecodeSliceMaxSizeAbsoluteCap ensures MaxSize(n>0) sets a fixed cap that
// both permits indices below it and rejects those at or above it.
func TestDecodeSliceMaxSizeAbsoluteCap(t *testing.T) {
	var ok sliceTarget
	d := NewDecoder(nil)
	d.MaxSize(20000)
	if err := d.DecodeString(&ok, "foo.10050=7"); err != nil {
		t.Fatalf("unexpected error under raised cap: %v", err)
	}
	if len(ok.Foo) != 10051 || ok.Foo[10050] != 7 {
		t.Fatalf("unexpected result: len=%d", len(ok.Foo))
	}

	var bad sliceTarget
	d2 := NewDecoder(nil)
	d2.MaxSize(10)
	if err := d2.DecodeString(&bad, "foo.50=1"); err == nil {
		t.Fatalf("expected an error above the absolute cap, got nil")
	}
}

// TestDecodeSliceMaxSizeDisabled ensures MaxSize(<0) restores unbounded growth
// for trusted input.
func TestDecodeSliceMaxSizeDisabled(t *testing.T) {
	var dst sliceTarget
	d := NewDecoder(nil)
	d.MaxSize(-1)
	// Far past the default floor; only a few KB, safe to actually allocate.
	if err := d.DecodeString(&dst, "foo.5000=9"); err != nil {
		t.Fatalf("unexpected error with the bound disabled: %v", err)
	}
	if len(dst.Foo) != 5001 || dst.Foo[5000] != 9 {
		t.Fatalf("unexpected result: len=%d", len(dst.Foo))
	}
}

// TestDecodePanicDoesNotEscape is a regression test for a panic escaping
// Decode/DecodeString/DecodeValues on a crafted duplicate key path. "a" appears
// as both a leaf and a parent, which makes parseValues' add() panic depending
// on map iteration order; that panic must be recovered into an error, never
// propagate to the caller and crash the process.
func TestDecodePanicDoesNotEscape(t *testing.T) {
	for i := 0; i < 5000; i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic escaped DecodeString on iter %d: %v", i, r)
				}
			}()
			var m map[string]interface{}
			_ = DecodeString(&m, "a=1&a.b=2") // may return an error; must not panic
		}()
	}
}

// TestDecodeDeepKeyBounded ensures a single key nested past the default depth is
// rejected with an error rather than driving recursion/allocation into a fatal
// crash. Regression test for the unbounded key-path recursion DoS.
func TestDecodeDeepKeyBounded(t *testing.T) {
	payload := strings.Repeat("a.", builtinMaxDepth+50) + "b=1"
	var m map[string]interface{}
	err := DecodeString(&m, payload)
	if err == nil {
		t.Fatalf("expected an error for an over-deep key, got nil")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected a depth error, got: %v", err)
	}
}

// TestDecodeModestDepthWorks ensures ordinary nested keys decode with no config.
func TestDecodeModestDepthWorks(t *testing.T) {
	var m map[string]interface{}
	if err := DecodeString(&m, "a.b.c.d.e=1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDecodeMaxDepthConfigurable ensures MaxDepth tightens and disables the
// depth bound.
func TestDecodeMaxDepthConfigurable(t *testing.T) {
	var tight map[string]interface{}
	d := NewDecoder(nil)
	d.MaxDepth(3)
	if err := d.DecodeString(&tight, "a.b.c.d.e=1"); err == nil {
		t.Fatalf("expected an error past the configured depth, got nil")
	}

	var off map[string]interface{}
	d2 := NewDecoder(nil)
	d2.MaxDepth(-1)
	// Past the default limit but small enough to build safely.
	if err := d2.DecodeString(&off, strings.Repeat("a.", builtinMaxDepth+2000)+"b=1"); err != nil {
		t.Fatalf("unexpected error with depth disabled: %v", err)
	}
}

func TestDecodeArrayNegativeIndex(t *testing.T) {
	var dst struct {
		Foo [3]int `form:"foo"`
	}
	if err := DecodeString(&dst, "foo.-1=1"); err == nil {
		t.Fatal("expected an error for a negative array index, got nil")
	} else if !strings.Contains(err.Error(), "not a valid index") {
		t.Fatalf("expected a clean index error, got: %v", err)
	}
}
