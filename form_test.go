// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"encoding"
	"fmt"
	"net/url"
	"time"
)

type Struct struct {
	B  bool
	I  int `form:"life"`
	F  float64
	R  rune `form:",omitempty"` // For testing when non-empty.
	Re rune `form:",omitempty"` // For testing when empty.
	S  string
	T  time.Time
	A  Array
	M  Map
	Y  interface{} `form:"-"`
	Zs Slice
	E  // Embedded.
}

type SXs map[string]interface{}
type E struct{ Bytes []byte }
type Z time.Time // Defined as such to test conversions.

func (z Z) String() string { return time.Time(z).String() }

type Array [3]string
type Map map[string]int
type Slice []struct {
	Z  Z
	U  U
	Up *U
	U_ U `form:"-"`
	E  `form:"-"`
}

// Custom marshaling
type U struct {
	a, b uint16
}

var (
	_ encoding.TextMarshaler   = &U{}
	_ encoding.TextUnmarshaler = &U{}
)

func (u U) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%d_%d", u.a, u.b)), nil
}

func (u *U) UnmarshalText(bs []byte) error {
	_, err := fmt.Sscanf(string(bs), "%d_%d", &u.a, &u.b)
	return err
}

func prepopulate(sxs SXs) SXs {
	var B bool
	var I int
	var F float64
	var R rune
	var S string
	var T time.Time
	var A Array
	var M Map
	// Y is ignored.
	var Zs Slice
	var E E
	sxs["B"] = B
	sxs["life"] = I
	sxs["F"] = F
	sxs["R"] = R
	// Re is omitted.
	sxs["S"] = S
	sxs["T"] = T
	sxs["A"] = A
	sxs["M"] = M
	// Y is ignored.
	sxs["Zs"] = Zs
	sxs["E"] = E
	return sxs
}

const (
	enc = 1
	dec = 2
	rnd = enc | dec
)

func testCases(mask int) (cs []testCase) {
	var B bool
	var I int
	var F float64
	var R rune
	var S string
	var T time.Time
	const canonical = "A.0=x&A.1=y&A.2=z&B=true&E.Bytes=%00%01%02&F=6.6&M.Bar=8&M.Foo=7&M.Qux=9&R=8734&S=Hello%2C+there.&T=2013-10-01T07%3A05%3A34.000000088Z&Zs.0.U=11_22&Zs.0.Up=33_44&Zs.0.Z=2006-12-01&life=42"
	const variation = ";A.0=x;M.Bar=8;F=6.6;A.1=y;R=8734;A.2=z;Zs.0.Up=33_44;B=true;M.Foo=7;T=2013-10-01T07:05:34.000000088Z;E.Bytes=%00%01%02;Zs.0.U=11_22;Zs.0.Z=2006-12-01;M.Qux=9;life=42;S=Hello,+there.;"

	for _, c := range []testCase{
		// Bools
		{&B, rnd, "", b(false)},
		{&B, rnd, "=true", b(true)},
		{&B, dec, "=false", b(false)},

		// Ints
		{&I, rnd, "", i(0)},
		{&I, rnd, "=42", i(42)},
		{&I, rnd, "=-42", i(-42)},
		{&I, dec, "=0", i(0)},
		{&I, dec, "=-0", i(0)},

		// Floats
		{&F, rnd, "", f(0)},
		{&F, rnd, "=6.6", f(6.6)},
		{&F, rnd, "=-6.6", f(-6.6)},

		// Runes
		{&R, rnd, "", r(0)},
		{&R, rnd, "=97", r('a')},
		{&R, rnd, "=8734", r('\u221E')},

		// Strings
		{&S, rnd, "", s("")},
		{&S, rnd, "=X+%26+Y+%26+Z", s("X & Y & Z")},
		{&S, rnd, "=Hello%2C+there.", s("Hello, there.")},
		{&S, dec, "=Hello, there.", s("Hello, there.")},

		// Dates/Times
		{&T, rnd, "", t(time.Time{})},
		{&T, rnd, "=2013-10-01T07%3A05%3A34.000000088Z", t(time.Date(2013, 10, 1, 7, 5, 34, 88, time.UTC))},
		{&T, dec, "=2013-10-01T07:05:34.000000088Z", t(time.Date(2013, 10, 1, 7, 5, 34, 88, time.UTC))},
		{&T, rnd, "=07%3A05%3A34.000000088Z", t(time.Date(0, 1, 1, 7, 5, 34, 88, time.UTC))},
		{&T, dec, "=07:05:34.000000088Z", t(time.Date(0, 1, 1, 7, 5, 34, 88, time.UTC))},
		{&T, rnd, "=2013-10-01", t(time.Date(2013, 10, 1, 0, 0, 0, 0, time.UTC))},

		// Structs
		{&Struct{}, rnd, canonical,
			&Struct{
				true,
				42,
				6.6,
				'\u221E',
				rune(0),
				"Hello, there.",
				time.Date(2013, 10, 1, 7, 5, 34, 88, time.UTC),
				Array{"x", "y", "z"},
				Map{"Foo": 7, "Bar": 8, "Qux": 9},
				nil,
				Slice{{Z(time.Date(2006, 12, 1, 0, 0, 0, 0, time.UTC)), U{11, 22}, &U{33, 44}, U{}, E{}}},
				E{[]byte{0, 1, 2}},
			},
		},

		{&Struct{}, dec, variation,
			&Struct{
				true,
				42,
				6.6,
				'\u221E',
				rune(0),
				"Hello, there.",
				time.Date(2013, 10, 1, 7, 5, 34, 88, time.UTC),
				Array{"x", "y", "z"},
				Map{"Foo": 7, "Bar": 8, "Qux": 9},
				nil,
				Slice{{Z(time.Date(2006, 12, 1, 0, 0, 0, 0, time.UTC)), U{11, 22}, &U{33, 44}, U{}, E{}}},
				E{[]byte{0, 1, 2}},
			},
		},

		// Maps
		{prepopulate(SXs{}), rnd, canonical,
			SXs{"B": true,
				"life": 42,
				"F":    6.6,
				"R":    '\u221E',
				// Re is omitted.
				"S": "Hello, there.",
				"T": time.Date(2013, 10, 1, 7, 5, 34, 88, time.UTC),
				"A": Array{"x", "y", "z"},
				"M": Map{"Foo": 7, "Bar": 8, "Qux": 9},
				// Y is ignored.
				"Zs": Slice{{Z(time.Date(2006, 12, 1, 0, 0, 0, 0, time.UTC)), U{11, 22}, &U{33, 44}, U{}, E{}}},
				"E":  E{[]byte{0, 1, 2}},
			},
		},
		{prepopulate(SXs{}), dec, variation,
			SXs{"B": true,
				"life": 42,
				"F":    6.6,
				"R":    '\u221E',
				// Re is omitted.
				"S": "Hello, there.",
				"T": time.Date(2013, 10, 1, 7, 5, 34, 88, time.UTC),
				"A": Array{"x", "y", "z"},
				"M": Map{"Foo": 7, "Bar": 8, "Qux": 9},
				// Y is ignored.
				"Zs": Slice{{Z(time.Date(2006, 12, 1, 0, 0, 0, 0, time.UTC)), U{11, 22}, &U{33, 44}, U{}, E{}}},
				"E":  E{[]byte{0, 1, 2}},
			},
		},

		{SXs{}, rnd, canonical,
			SXs{"B": "true",
				"life": "42",
				"F":    "6.6",
				"R":    "8734",
				// Re is omitted.
				"S": "Hello, there.",
				"T": "2013-10-01T07:05:34.000000088Z",
				"A": map[string]interface{}{"0": "x", "1": "y", "2": "z"},
				"M": map[string]interface{}{"Foo": "7", "Bar": "8", "Qux": "9"},
				// Y is ignored.
				"Zs": map[string]interface{}{
					"0": map[string]interface{}{
						"Z":  "2006-12-01",
						"U":  "11_22",
						"Up": "33_44",
					},
				},
				"E": map[string]interface{}{"Bytes": string([]byte{0, 1, 2})},
			},
		},
		{SXs{}, dec, variation,
			SXs{"B": "true",
				"life": "42",
				"F":    "6.6",
				"R":    "8734",
				// Re is omitted.
				"S": "Hello, there.",
				"T": "2013-10-01T07:05:34.000000088Z",
				"A": map[string]interface{}{"0": "x", "1": "y", "2": "z"},
				"M": map[string]interface{}{"Foo": "7", "Bar": "8", "Qux": "9"},
				// Y is ignored.
				"Zs": map[string]interface{}{
					"0": map[string]interface{}{
						"Z":  "2006-12-01",
						"U":  "11_22",
						"Up": "33_44",
					},
				},
				"E": map[string]interface{}{"Bytes": string([]byte{0, 1, 2})},
			},
		},
	} {
		if c.m&mask != 0 {
			cs = append(cs, c)
		}
	}
	return cs
}

type testCase struct {
	a interface{}
	m int
	s string
	b interface{}
}

func t(t time.Time) *time.Time { return &t }
func b(b bool) *bool           { return &b }
func i(i int) *int             { return &i }
func f(f float64) *float64     { return &f }
func r(r rune) *rune           { return &r }
func s(s string) *string       { return &s }

func mustParseQuery(s string) url.Values {
	vs, err := url.ParseQuery(s)
	if err != nil {
		panic(err)
	}
	return vs
}
