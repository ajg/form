// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18

package form

import (
	"errors"
	"image/color"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"
)

// FuzzDecodeString asserts three invariants over arbitrary input: decoding
// never panics (the fuzz driver fails on any uncaught panic), every returned
// error is a typed *Error, and any input that decodes into an untyped map
// re-encodes without error. The seed corpus includes the resource-exhaustion
// and conflict patterns from the v1.7.x hardening work; CI runs the seeds as
// ordinary tests on every build.
func FuzzDecodeString(f *testing.F) {
	for _, seed := range []string{
		"a=1&b=two&c.d=3&c.e=4",
		"Foo.900000000=x",
		strings.Repeat("q.", 128) + "z=1",
		"a=1&a.b=2",
		"tags._=a&tags._=b",
		"n=%2300aaff&p=ff000080&g=7f",
		"t=2016-03-24&u=http%3A%2F%2Fx&i=123456789012345678901234567890",
		"%zz=1",
		";a=1;b=2;",
		`P\.D\\Q\.B.A=P/D`,
		"a.-1=x",
		"", "=", "&", "a=", "=b", "a", "a=&a.b=",
	} {
		f.Add(seed)
	}

	type typed struct {
		N color.NRGBA `form:"n"`
		P color.RGBA  `form:"p"`
		T time.Time   `form:"t"`
		U url.URL     `form:"u"`
		I *big.Int    `form:"i"`
		S []string    `form:"tags"`
		M map[string]string
	}

	f.Fuzz(func(t *testing.T, s string) {
		var m map[string]interface{}
		if err := DecodeString(&m, s); err != nil {
			var fe *Error
			if !errors.As(err, &fe) {
				t.Fatalf("untyped error %T from %q: %v", err, s, err)
			}
		} else if _, err := EncodeToString(m); err != nil {
			t.Fatalf("input %q decoded but its map failed to re-encode: %v", s, err)
		}

		var d typed
		if err := DecodeString(&d, s); err != nil {
			var fe *Error
			if !errors.As(err, &fe) {
				t.Fatalf("untyped error %T from %q into struct: %v", err, s, err)
			}
		}
	})
}
