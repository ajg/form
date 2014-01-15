// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecodeString(t *testing.T) {
	for _, c := range testCases(dec) {
		if err := DecodeString(c.a, c.s); err != nil {
			t.Errorf("DecodeString(%v): %s", c.s, err)
		} else if !reflect.DeepEqual(c.a, c.b) {
			t.Errorf("DecodeString(%v)\nwant (%v)\nhave (%v)", c.s, c.b, c.a)
		}
	}
}

func TestDecodeValues(t *testing.T) {
	for _, c := range testCases(dec) {
		vs := mustParseQuery(c.s)

		if err := DecodeValues(c.a, vs); err != nil {
			t.Errorf("DecodeValues(%v): %s", vs, err)
		} else if !reflect.DeepEqual(c.a, c.b) {
			t.Errorf("DecodeValues(%v)\nwant (%v)\nhave (%v)", vs, c.b, c.a)
		}
	}
}

func TestDecode(t *testing.T) {
	for _, c := range testCases(dec) {
		r := strings.NewReader(c.s)
		d := NewDecoder(r)

		if err := d.Decode(c.a); err != nil {
			t.Errorf("Decode(%v): %s", r, err)
		} else if !reflect.DeepEqual(c.a, c.b) {
			t.Errorf("Decode(%v)\nwant (%v)\nhave (%v)", r, c.b, c.a)
		}
	}
}
