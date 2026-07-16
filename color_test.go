// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"errors"
	"image/color"
	"testing"
)

type palette struct {
	G   color.Gray    `form:"g"`
	G16 color.Gray16  `form:"g16"`
	A   color.Alpha   `form:"a"`
	A16 color.Alpha16 `form:"a16"`
	N   color.NRGBA   `form:"n"`
	N64 color.NRGBA64 `form:"n64"`
	P   color.RGBA    `form:"p"`
	P64 color.RGBA64  `form:"p64"`
	K   color.CMYK    `form:"k"`
}

func TestColorDecode(t *testing.T) {
	var dst palette
	err := DecodeString(&dst,
		"g=7f&g16=7fff&a=80&a16=8000&n=ff007f80&n64=ffff00007fff8000&p=80004080&p64=80000000400a8000&k=ff00807f")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Gray", dst.G, color.Gray{Y: 0x7f}},
		{"Gray16", dst.G16, color.Gray16{Y: 0x7fff}},
		{"Alpha", dst.A, color.Alpha{A: 0x80}},
		{"Alpha16", dst.A16, color.Alpha16{A: 0x8000}},
		{"NRGBA", dst.N, color.NRGBA{R: 0xff, G: 0x00, B: 0x7f, A: 0x80}},
		{"NRGBA64", dst.N64, color.NRGBA64{R: 0xffff, G: 0x0000, B: 0x7fff, A: 0x8000}},
		{"RGBA", dst.P, color.RGBA{R: 0x80, G: 0x00, B: 0x40, A: 0x80}},
		{"RGBA64", dst.P64, color.RGBA64{R: 0x8000, G: 0x0000, B: 0x400a, A: 0x8000}},
		{"CMYK", dst.K, color.CMYK{C: 0xff, M: 0x00, Y: 0x80, K: 0x7f}},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, c.got, c.want)
		}
	}
}

func TestColorDecodeLenient(t *testing.T) {
	// A leading '#' (as HTML <input type=color> submits), uppercase, and
	// CSS shorthand are all accepted.
	var dst struct {
		N color.NRGBA `form:"n"`
	}
	for src, want := range map[string]color.NRGBA{
		"%23ff007f": {R: 0xff, G: 0x00, B: 0x7f, A: 0xff}, // "#ff007f"
		"FF007F80":  {R: 0xff, G: 0x00, B: 0x7f, A: 0x80},
		"f07":       {R: 0xff, G: 0x00, B: 0x77, A: 0xff},
		"f078":      {R: 0xff, G: 0x00, B: 0x77, A: 0x88},
		"":          {},
	} {
		dst.N = color.NRGBA{R: 1} // Ensure the field is actually written.
		if err := DecodeString(&dst, "n="+src); err != nil {
			t.Errorf("%q: unexpected error: %v", src, err)
		} else if dst.N != want {
			t.Errorf("%q: got %+v, want %+v", src, dst.N, want)
		}
	}
}

func TestColorDecodeErrors(t *testing.T) {
	var dst struct {
		N color.NRGBA `form:"n"`
		P color.RGBA  `form:"p"`
	}
	for name, src := range map[string]string{
		"bad digits":   "n=zz0000",
		"bad length":   "n=ff000",
		"way too long": "n=ff0000ff00",
	} {
		err := DecodeString(&dst, src)
		var fe *Error
		if err == nil || !errors.As(err, &fe) || fe.Kind != KindParse {
			t.Errorf("%s (%q): got %v, want KindParse", name, src, err)
		}
	}

	// A premultiplied color with a channel above alpha is rejected loudly:
	// it is almost always a straight-alpha value aimed at the wrong type.
	err := DecodeString(&dst, "p=ff000080")
	var fe *Error
	if err == nil || !errors.As(err, &fe) || fe.Kind != KindParse {
		t.Fatalf("premultiplied: got %v, want KindParse", err)
	}

	// The same value is valid for the straight-alpha type.
	if err := DecodeString(&dst, "n=ff000080"); err != nil {
		t.Errorf("straight alpha: unexpected error: %v", err)
	}
}

func TestColorCompositeStillDecodes(t *testing.T) {
	// The pre-existing generic struct representation keeps working.
	var dst struct {
		P color.RGBA `form:"p"`
	}
	if err := DecodeString(&dst, "p.R=255&p.G=10&p.B=20&p.A=255"); err != nil {
		t.Fatal(err)
	}
	if (dst.P != color.RGBA{R: 255, G: 10, B: 20, A: 255}) {
		t.Errorf("got %+v", dst.P)
	}
}

func TestColorEncodeDefaultIsComposite(t *testing.T) {
	// Without HexColors, the wire format is unchanged from prior versions.
	src := struct {
		N color.NRGBA `form:"n"`
	}{color.NRGBA{R: 255, G: 10, B: 20, A: 255}}
	s, err := EncodeToString(&src)
	if err != nil {
		t.Fatal(err)
	}
	if s != "n.A=255&n.B=20&n.G=10&n.R=255" {
		t.Errorf("composite default changed: %q", s)
	}
}

func TestColorEncodeHexAndRoundTrip(t *testing.T) {
	src := palette{
		G:   color.Gray{Y: 0x7f},
		G16: color.Gray16{Y: 0x7fff},
		A:   color.Alpha{A: 0x80},
		A16: color.Alpha16{A: 0x8000},
		N:   color.NRGBA{R: 0xff, G: 0x00, B: 0x7f, A: 0x80},
		N64: color.NRGBA64{R: 0xffff, G: 0x0000, B: 0x7fff, A: 0x8000},
		P:   color.RGBA{R: 0x80, G: 0x00, B: 0x40, A: 0x80},
		P64: color.RGBA64{R: 0x8000, G: 0x0000, B: 0x400a, A: 0x8000},
		K:   color.CMYK{C: 0xff, M: 0x00, Y: 0x80, K: 0x7f},
	}
	var sb stringWriter
	if err := NewEncoder(&sb).HexColors(true).Encode(&src); err != nil {
		t.Fatal(err)
	}
	s := sb.String()

	var dst palette
	if err := DecodeString(&dst, s); err != nil {
		t.Fatalf("re-decode of %q: %v", s, err)
	}
	if dst != src {
		t.Errorf("round trip: got %+v, want %+v (via %q)", dst, src, s)
	}
}

func TestColorNamedTypeFallsBack(t *testing.T) {
	// Named types based on color types keep the generic composite
	// representation: hex handling matches by exact type only.
	type Tint color.NRGBA
	src := struct {
		T Tint `form:"t"`
	}{Tint{R: 1, G: 2, B: 3, A: 4}}
	var sb stringWriter
	if err := NewEncoder(&sb).HexColors(true).Encode(&src); err != nil {
		t.Fatal(err)
	}
	if s := sb.String(); s != "t.A=4&t.B=3&t.G=2&t.R=1" {
		t.Errorf("named type: got %q, want composite form", s)
	}
}

// stringWriter is a minimal in-memory io.Writer for encoder tests.
type stringWriter struct{ bs []byte }

func (w *stringWriter) Write(p []byte) (int, error) { w.bs = append(w.bs, p...); return len(p), nil }
func (w *stringWriter) String() string              { return string(w.bs) }
