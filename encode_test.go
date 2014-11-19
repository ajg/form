// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"bytes"
	"reflect"
	"testing"
)

func TestEncodeToString(t *testing.T) {
	for _, c := range testCases(encOnly) {
		if s, err := EncodeToString(c.b); err != nil {
			t.Errorf("EncodeToString(%v): %s", c.b, err)
		} else if !reflect.DeepEqual(c.s, s) {
			t.Errorf("EncodeToString(%v)\nwant (%v)\nhave (%v)", c.b, c.s, s)
		}
	}
}

func TestEncodeToValues(t *testing.T) {
	for _, c := range testCases(encOnly) {
		cvs := mustParseQuery(c.s)
		if vs, err := EncodeToValues(c.b); err != nil {
			t.Errorf("EncodeToValues(%v): %s", c.b, err)
		} else if !reflect.DeepEqual(cvs, vs) {
			t.Errorf("EncodeToValues(%v)\nwant (%v)\nhave (%v)", c.b, cvs, vs)
		}
	}
}

func TestEncode(t *testing.T) {
	for _, c := range testCases(encOnly) {
		var w bytes.Buffer
		e := NewEncoder(&w)

		if err := e.Encode(c.b); err != nil {
			t.Errorf("Encode(%v): %s", w, err)
		} else if s := w.String(); !reflect.DeepEqual(c.s, s) {
			t.Errorf("Encode(%v)\nwant (%v)\nhave (%v)", w, c.s, s)
		}
	}
}
