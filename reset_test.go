// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"bytes"
	"strings"
	"testing"
)

// Reset (issue #8) lets a configured Decoder/Encoder be reused across
// readers/writers; these tests pin that configuration survives the swap.

func TestDecoderReset(t *testing.T) {
	type v struct {
		A struct {
			B string `form:"b"`
		} `form:"a"`
	}
	d := NewDecoder(strings.NewReader("a|b=first")).DelimitWith('|')

	var one v
	if err := d.Decode(&one); err != nil {
		t.Fatal(err)
	}
	var two v
	if err := d.Reset(strings.NewReader("a|b=second")).Decode(&two); err != nil {
		t.Fatal(err)
	}
	if one.A.B != "first" || two.A.B != "second" {
		t.Errorf("got %q then %q", one.A.B, two.A.B)
	}
}

func TestEncoderReset(t *testing.T) {
	src := struct {
		A struct {
			B string `form:"b"`
		} `form:"a"`
	}{}
	src.A.B = "x"

	var b1, b2 bytes.Buffer
	e := NewEncoder(&b1).DelimitWith('|')
	if err := e.Encode(&src); err != nil {
		t.Fatal(err)
	}
	if err := e.Reset(&b2).Encode(&src); err != nil {
		t.Fatal(err)
	}
	if b1.String() != "a%7Cb=x" || b2.String() != b1.String() {
		t.Errorf("got %q then %q; configuration not retained", b1.String(), b2.String())
	}
}
