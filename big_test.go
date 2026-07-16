// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"math/big"
	"strings"
	"testing"
)

// The math/big types implement encoding.TextMarshaler and
// encoding.TextUnmarshaler, so they encode and decode out of the box — Int
// and Rat at arbitrary precision, Float at the destination's precision
// (a 64-bit mantissa unless pre-set); these tests pin that behavior.

type bigValues struct {
	I *big.Int   `form:"i"`
	V big.Int    `form:"v"`
	F *big.Float `form:"f"`
	R *big.Rat   `form:"r"`
}

const oversized = "123456789012345678901234567890" // > 2^64

func TestBigDecode(t *testing.T) {
	var dst bigValues
	if err := DecodeString(&dst, "i="+oversized+"&v=42&f=3.25&r="+oversized+"/7"); err != nil {
		t.Fatal(err)
	}
	if dst.I == nil || dst.I.String() != oversized {
		t.Errorf("*big.Int: got %v, want %s", dst.I, oversized)
	}
	if dst.V.String() != "42" {
		t.Errorf("big.Int value field: got %v, want 42", &dst.V)
	}
	if dst.F == nil || dst.F.String() != "3.25" {
		t.Errorf("*big.Float: got %v, want 3.25", dst.F)
	}
	want := new(big.Rat)
	want.SetString(oversized + "/7")
	if dst.R == nil || dst.R.Cmp(want) != 0 {
		t.Errorf("*big.Rat: got %v, want %v", dst.R, want)
	}
}

func TestBigEncode(t *testing.T) {
	i, ok := new(big.Int).SetString(oversized, 10)
	if !ok {
		t.Fatal("SetString failed")
	}
	src := bigValues{I: i, F: big.NewFloat(3.25), R: big.NewRat(22, 7)}
	src.V.SetInt64(42)
	s, err := EncodeToString(&src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"i=" + oversized, "v=42", "f=3.25", "r=22%2F7"} {
		if !strings.Contains(s, want) {
			t.Errorf("encoded form %q lacks %q", s, want)
		}
	}
}

// Untyped decoding performs no numeric interpretation: every simple value is
// preserved as a string, so numerals of any magnitude survive losslessly and
// the caller chooses how (and at what precision) to parse them.
func TestUntypedNumeralsRemainStrings(t *testing.T) {
	dst := map[string]interface{}{}
	if err := DecodeString(&dst, "a=42&b="+oversized+"&c=3.14&d=1e400&e=007"); err != nil {
		t.Fatal(err)
	}
	for k, v := range dst {
		if _, ok := v.(string); !ok {
			t.Errorf("untyped value %q: got %T, want string", k, v)
		}
	}
	if dst["b"] != oversized {
		t.Errorf("oversized numeral not preserved: got %v", dst["b"])
	}
	if dst["e"] != "007" {
		t.Errorf("leading zeros not preserved: got %v", dst["e"])
	}
}

// Values that exceed a primitive destination's range fail loudly instead of
// being silently truncated; math/big destinations are the remedy.
func TestPrimitiveOverflowErrors(t *testing.T) {
	var i struct {
		N int64 `form:"n"`
	}
	if err := DecodeString(&i, "n="+oversized); err == nil {
		t.Error("expected error decoding oversized integer into int64")
	}
	var f struct {
		X float64 `form:"x"`
	}
	if err := DecodeString(&f, "x=1e400"); err == nil {
		t.Error("expected error decoding out-of-range float into float64")
	}
}

// TestBigFloatPrecision pins Float's precision contract: decoding rounds to
// the destination's precision — 64-bit mantissa when unset — a precision
// pre-set with SetPrec is honored, and a decoded value re-encodes and
// re-decodes to an equal value.
func TestBigFloatPrecision(t *testing.T) {
	const long = "3.14159265358979323846264338327950288419716939937510582097494459"

	var d struct {
		F *big.Float `form:"f"`
	}
	if err := DecodeString(&d, "f="+long); err != nil {
		t.Fatal(err)
	}
	if d.F.Prec() != 64 {
		t.Errorf("default precision: got %d, want 64", d.F.Prec())
	}

	var p struct {
		F *big.Float `form:"f"`
	}
	p.F = new(big.Float).SetPrec(200)
	if err := DecodeString(&p, "f="+long); err != nil {
		t.Fatal(err)
	}
	if p.F.Prec() != 200 {
		t.Errorf("pre-set precision: got %d, want 200", p.F.Prec())
	}
	want, _, err := big.ParseFloat(long, 10, 200, big.ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if p.F.Cmp(want) != 0 {
		t.Errorf("pre-set decode: got %s, want %s", p.F.Text('g', 65), want.Text('g', 65))
	}

	s, err := EncodeToString(&d)
	if err != nil {
		t.Fatal(err)
	}
	var d2 struct {
		F *big.Float `form:"f"`
	}
	if err := DecodeString(&d2, s); err != nil {
		t.Fatal(err)
	}
	if d.F.Cmp(d2.F) != 0 {
		t.Errorf("round-trip: %s re-decoded as %s", d.F.Text('g', 25), d2.F.Text('g', 25))
	}
}
