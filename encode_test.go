package form

import (
	"bytes"
	"reflect"
	"testing"
)

func TestEncodeToString(t *testing.T) {
	for _, c := range testCases(enc) {
		if s, err := EncodeToString(c.b); err != nil {
			t.Errorf("EncodeToString(%v): %s", c.b, err)
		} else if !reflect.DeepEqual(c.s, s) {
			t.Errorf("EncodeToString(%v)\nwant (%v)\nhave (%v)", c.b, c.s, s)
		}
	}
}

func TestEncodeToValues(t *testing.T) {
	for _, c := range testCases(enc) {
		cvs := mustParseQuery(c.s)
		if vs, err := EncodeToValues(c.b); err != nil {
			t.Errorf("EncodeToValues(%v): %s", c.b, err)
		} else if !reflect.DeepEqual(cvs, vs) {
			t.Errorf("EncodeToValues(%v)\nwant (%v)\nhave (%v)", c.b, cvs, vs)
		}
	}
}

func TestEncode(t *testing.T) {
	for _, c := range testCases(enc) {
		var w bytes.Buffer
		e := NewEncoder(&w)

		if err := e.Encode(c.b); err != nil {
			t.Errorf("Encode(%v): %s", w, err)
		} else if s := w.String(); !reflect.DeepEqual(c.s, s) {
			t.Errorf("Encode(%v)\nwant (%v)\nhave (%v)", w, c.s, s)
		}
	}
}
