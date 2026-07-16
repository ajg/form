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
// encoding.TextUnmarshaler, so they encode and decode out of the box at
// arbitrary precision; these tests pin that behavior.

type bigValues struct {
	I *big.Int   `form:"i"`
	V big.Int    `form:"v"`
	F *big.Float `form:"f"`
	R *big.Rat   `form:"r"`
}

const oversized = "123456789012345678901234567890" // > 2^64

func TestBigDecode(t *testing.T) {
	var dst bigValues
	if err := DecodeString(&dst, "i="+oversized+"&v=42&f=3.25&r=22/7"); err != nil {
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
	if dst.R == nil || dst.R.String() != "22/7" {
		t.Errorf("*big.Rat: got %v, want 22/7", dst.R)
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
