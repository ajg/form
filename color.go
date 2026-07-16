// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"image/color"
	"reflect"
	"strings"
)

// The fixed-channel image/color types decode from hex strings — as submitted
// by HTML <input type=color> elements — and can optionally encode to them
// (see Encoder.HexColors). The encoding is byte-faithful per type: each
// channel is written as stored, so every value round-trips exactly. Input is
// case-insensitive, tolerates a leading '#' (never emitted), and accepts
// CSS-style shorthand for the 8-bit RGBA types. Digit counts per type:
//
//	color.Gray, color.Alpha       2
//	color.Gray16, color.Alpha16   4
//	color.NRGBA, color.RGBA       3, 4, 6 or 8 (shorthand nibbles doubled;
//	                              omitted alpha defaults to ff)
//	color.NRGBA64, color.RGBA64   12 or 16 (omitted alpha defaults to ffff)
//	color.CMYK                    8
//
// The premultiplied types (RGBA, RGBA64) reject values whose color channels
// exceed their alpha, since no valid premultiplied color has that shape; it
// almost always indicates straight-alpha (CSS) semantics, for which NRGBA
// and NRGBA64 are the faithful destinations.
//
// Matching is by exact type; named types based on these fall back to the
// generic composite-struct representation, as all struct types did before
// hex support existed. The composite form also remains accepted on decode
// for every color type, and remains the default encoding.
var (
	grayType    = reflect.TypeOf(color.Gray{})
	gray16Type  = reflect.TypeOf(color.Gray16{})
	alphaType   = reflect.TypeOf(color.Alpha{})
	alpha16Type = reflect.TypeOf(color.Alpha16{})
	nrgbaType   = reflect.TypeOf(color.NRGBA{})
	nrgba64Type = reflect.TypeOf(color.NRGBA64{})
	rgbaType    = reflect.TypeOf(color.RGBA{})
	rgba64Type  = reflect.TypeOf(color.RGBA64{})
	cmykType    = reflect.TypeOf(color.CMYK{})
)

func isColorType(t reflect.Type) bool {
	switch t {
	case grayType, gray16Type, alphaType, alpha16Type,
		nrgbaType, nrgba64Type, rgbaType, rgba64Type, cmykType:
		return true
	}
	return false
}

// decodeColor sets v (a fixed-channel color type) from hex string s.
func decodeColor(v reflect.Value, s string) {
	t := v.Type()
	h := strings.ToLower(strings.TrimPrefix(s, "#"))
	ns, ok := nibbles(h)

	badLength := func(want string) {
		panic(errKind(KindParse, s+" cannot be decoded as "+t.String()+
			" (want "+want+" hex digits)"))
	}
	if !ok {
		panic(errKind(KindParse, s+" cannot be decoded as "+t.String()+
			" (not a hex string)"))
	}

	var c interface{}
	switch t {
	case grayType, alphaType:
		if len(ns) != 2 {
			badLength("2")
		}
		y := ns[0]<<4 | ns[1]
		if t == grayType {
			c = color.Gray{Y: y}
		} else {
			c = color.Alpha{A: y}
		}
	case gray16Type, alpha16Type:
		if len(ns) != 4 {
			badLength("4")
		}
		y := pack16(ns)
		if t == gray16Type {
			c = color.Gray16{Y: y}
		} else {
			c = color.Alpha16{A: y}
		}
	case nrgbaType, rgbaType:
		r, g, b, a, ok := rgba8(ns)
		if !ok {
			badLength("3, 4, 6 or 8")
		}
		if t == nrgbaType {
			c = color.NRGBA{R: r, G: g, B: b, A: a}
		} else {
			if r > a || g > a || b > a {
				panic(premultErr(s, t))
			}
			c = color.RGBA{R: r, G: g, B: b, A: a}
		}
	case nrgba64Type, rgba64Type:
		r, g, b, a, ok := rgba16(ns)
		if !ok {
			badLength("12 or 16")
		}
		if t == nrgba64Type {
			c = color.NRGBA64{R: r, G: g, B: b, A: a}
		} else {
			if r > a || g > a || b > a {
				panic(premultErr(s, t))
			}
			c = color.RGBA64{R: r, G: g, B: b, A: a}
		}
	case cmykType:
		if len(ns) != 8 {
			badLength("8")
		}
		c = color.CMYK{
			C: ns[0]<<4 | ns[1], M: ns[2]<<4 | ns[3],
			Y: ns[4]<<4 | ns[5], K: ns[6]<<4 | ns[7],
		}
	}
	v.Set(reflect.ValueOf(c))
}

func premultErr(s string, t reflect.Type) *Error {
	return errKind(KindParse, s+" is not a valid premultiplied "+t.String()+
		" (a color channel exceeds alpha); use the straight-alpha color.N"+
		strings.TrimPrefix(t.String(), "color.")+" instead, or premultiply")
}

// encodeColor renders v (a fixed-channel color type) as bare lowercase hex.
func encodeColor(v reflect.Value) string {
	const digits = "0123456789abcdef"
	var bs []byte
	appendByte := func(b uint8) {
		bs = append(bs, digits[b>>4], digits[b&0xf])
	}
	appendWord := func(w uint16) {
		appendByte(uint8(w >> 8))
		appendByte(uint8(w))
	}
	switch c := v.Interface().(type) {
	case color.Gray:
		appendByte(c.Y)
	case color.Alpha:
		appendByte(c.A)
	case color.Gray16:
		appendWord(c.Y)
	case color.Alpha16:
		appendWord(c.A)
	case color.NRGBA:
		appendByte(c.R)
		appendByte(c.G)
		appendByte(c.B)
		appendByte(c.A)
	case color.RGBA:
		appendByte(c.R)
		appendByte(c.G)
		appendByte(c.B)
		appendByte(c.A)
	case color.NRGBA64:
		appendWord(c.R)
		appendWord(c.G)
		appendWord(c.B)
		appendWord(c.A)
	case color.RGBA64:
		appendWord(c.R)
		appendWord(c.G)
		appendWord(c.B)
		appendWord(c.A)
	case color.CMYK:
		appendByte(c.C)
		appendByte(c.M)
		appendByte(c.Y)
		appendByte(c.K)
	}
	return string(bs)
}

// nibbles converts a hex string to nibble values; false if any rune is not a
// hex digit.
func nibbles(s string) ([]uint8, bool) {
	ns := make([]uint8, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case '0' <= c && c <= '9':
			ns = append(ns, c-'0')
		case 'a' <= c && c <= 'f':
			ns = append(ns, c-'a'+10)
		default:
			return nil, false
		}
	}
	return ns, true
}

// rgba8 interprets 3/4/6/8 nibbles as 8-bit RGBA, doubling shorthand nibbles
// and defaulting an omitted alpha to 0xff.
func rgba8(ns []uint8) (r, g, b, a uint8, ok bool) {
	switch len(ns) {
	case 3:
		return ns[0] * 0x11, ns[1] * 0x11, ns[2] * 0x11, 0xff, true
	case 4:
		return ns[0] * 0x11, ns[1] * 0x11, ns[2] * 0x11, ns[3] * 0x11, true
	case 6:
		return ns[0]<<4 | ns[1], ns[2]<<4 | ns[3], ns[4]<<4 | ns[5], 0xff, true
	case 8:
		return ns[0]<<4 | ns[1], ns[2]<<4 | ns[3], ns[4]<<4 | ns[5], ns[6]<<4 | ns[7], true
	}
	return 0, 0, 0, 0, false
}

// rgba16 interprets 12/16 nibbles as 16-bit RGBA, defaulting an omitted
// alpha to 0xffff.
func rgba16(ns []uint8) (r, g, b, a uint16, ok bool) {
	switch len(ns) {
	case 12:
		return pack16(ns[0:4]), pack16(ns[4:8]), pack16(ns[8:12]), 0xffff, true
	case 16:
		return pack16(ns[0:4]), pack16(ns[4:8]), pack16(ns[8:12]), pack16(ns[12:16]), true
	}
	return 0, 0, 0, 0, false
}

func pack16(ns []uint8) uint16 {
	return uint16(ns[0])<<12 | uint16(ns[1])<<8 | uint16(ns[2])<<4 | uint16(ns[3])
}
