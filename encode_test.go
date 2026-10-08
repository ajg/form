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
			t.Errorf("EncodeToString(%#v): %s", c.b, err)
		} else if !reflect.DeepEqual(c.s, s) {
			t.Errorf("EncodeToString(%#v)\n want (%#v)\n have (%#v)", c.b, c.s, s)
		}
	}
}

func TestEncodeToValues(t *testing.T) {
	for _, c := range testCases(encOnly) {
		cvs := mustParseQuery(c.s)
		if vs, err := EncodeToValues(c.b); err != nil {
			t.Errorf("EncodeToValues(%#v): %s", c.b, err)
		} else if !reflect.DeepEqual(cvs, vs) {
			t.Errorf("EncodeToValues(%#v)\n want (%#v)\n have (%#v)", c.b, cvs, vs)
		}
	}
}

func TestEncode(t *testing.T) {
	for _, c := range testCases(encOnly) {
		var w bytes.Buffer
		e := NewEncoder(&w)

		if err := e.Encode(c.b); err != nil {
			t.Errorf("Encode(%#v): %s", c.b, err)
		} else if s := w.String(); !reflect.DeepEqual(c.s, s) {
			t.Errorf("Encode(%#v)\n want (%#v)\n have (%#v)", c.b, c.s, s)
		}
	}
}

type Thing1 struct {
	String  string `form:"name,omitempty"`
	Integer *uint  `form:"num,omitempty"`
}

type Thing2 struct {
	String  string `form:"name,omitempty"`
	Integer uint   `form:"num,omitempty"`
}

type Thing3 struct {
	String  string `form:"name"`
	Integer *uint  `form:"num"`
}

type Thing4 struct {
	String  string `form:"name"`
	Integer uint   `form:"num"`
}

func TestEncode_KeepZero(t *testing.T) {
	num := uint(0)
	for _, c := range []struct {
		b interface{}
		s string
		z bool
	}{
		{Thing1{"test", &num}, "name=test&num=", false},
		{Thing1{"test", &num}, "name=test&num=0", true},
		{Thing2{"test", num}, "name=test", false},
		{Thing2{"test", num}, "name=test", true},
		{Thing3{"test", &num}, "name=test&num=", false},
		{Thing3{"test", &num}, "name=test&num=0", true},
		{Thing4{"test", num}, "name=test&num=", false},
		{Thing4{"test", num}, "name=test&num=0", true},
		{Thing1{"", &num}, "num=", false},
		{Thing1{"", &num}, "num=0", true},
		{Thing2{"", num}, "", false},
		{Thing2{"", num}, "", true},
		{Thing3{"", &num}, "name=&num=", false},
		{Thing3{"", &num}, "name=&num=0", true},
		{Thing4{"", num}, "name=&num=", false},
		{Thing4{"", num}, "name=&num=0", true},
	} {

		var w bytes.Buffer
		e := NewEncoder(&w)

		if err := e.KeepZeros(c.z).Encode(c.b); err != nil {
			t.Errorf("KeepZeros(%#v).Encode(%#v): %s", c.z, c.b, err)
		} else if s := w.String(); c.s != s {
			t.Errorf("KeepZeros(%#v).Encode(%#v)\n want (%#v)\n have (%#v)", c.z, c.b, c.s, s)
		}
	}
}

func TestEncode_OmitEmpty(t *testing.T) {
	num := uint(0)
	nonZeroNum := uint(42)
	for _, c := range []struct {
		b interface{}
		s string
		o bool
	}{
		// Thing3 and Thing4 have no omitempty tags, so OmitEmpty affects them.
		{Thing3{"test", &nonZeroNum}, "name=test&num=42", false},
		{Thing3{"test", &nonZeroNum}, "name=test&num=42", true},
		{Thing3{"", &nonZeroNum}, "name=&num=42", false},
		{Thing3{"", &nonZeroNum}, "num=42", true},
		{Thing3{"test", nil}, "name=test&num=", false},
		{Thing3{"test", nil}, "name=test", true},
		{Thing4{"test", 0}, "name=test&num=", false},
		{Thing4{"test", 0}, "name=test", true},
		{Thing4{"test", 42}, "name=test&num=42", false},
		{Thing4{"test", 42}, "name=test&num=42", true},
		// Thing1 and Thing2 already have omitempty tags.
		{Thing1{"test", &num}, "name=test&num=", false},
		{Thing1{"test", &num}, "name=test&num=", true},
		{Thing2{"test", 0}, "name=test", false},
		{Thing2{"test", 0}, "name=test", true},
	} {

		var w bytes.Buffer
		e := NewEncoder(&w)

		if err := e.OmitEmpty(c.o).Encode(c.b); err != nil {
			t.Errorf("OmitEmpty(%#v).Encode(%#v): %s", c.o, c.b, err)
		} else if s := w.String(); c.s != s {
			t.Errorf("OmitEmpty(%#v).Encode(%#v)\n want (%#v)\n have (%#v)", c.o, c.b, c.s, s)
		}
	}
}

func TestEncode_Cycle(t *testing.T) {
	t.Run("self-referential struct pointer", func(t *testing.T) {
		type Cyclic struct {
			Name string
			Next *Cyclic
		}
		a := &Cyclic{Name: "a"}
		a.Next = a

		if _, err := EncodeToString(a); err == nil {
			t.Error("expected error for cyclic struct pointer, got nil")
		}
	})

	t.Run("map containing itself", func(t *testing.T) {
		m := map[string]interface{}{}
		m["self"] = m

		if _, err := EncodeToString(m); err == nil {
			t.Error("expected error for cyclic map, got nil")
		}
	})

	t.Run("non-cyclic pointer sharing (DAG)", func(t *testing.T) {
		type Node struct {
			Value string
		}
		type DAG struct {
			A *Node
			B *Node
		}
		shared := &Node{Value: "shared"}
		dag := DAG{A: shared, B: shared}

		if _, err := EncodeToString(dag); err != nil {
			t.Errorf("unexpected error for DAG: %s", err)
		}
	})
}

func TestEncode_ConflictResolution(t *testing.T) {
	for _, c := range []struct {
		name string
		b    interface{}
		s    string
	}{
		{
			"depth shadow: parent field wins",
			&DepthShadow{X: "parent", DepthInner: DepthInner{X: "child"}},
			"X=parent",
		},
		{
			"ambiguous: same-depth fields omitted",
			&Ambiguous{AmbigA: AmbigA{X: "a"}, AmbigB: AmbigB{X: "b"}},
			"",
		},
		{
			"tagged wins: tagged field beats untagged at same depth",
			&TaggedWins{TaggedInner: TaggedInner{X: "tagged"}, UntaggedInner: UntaggedInner{X: "untagged"}},
			"X=tagged",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if s, err := EncodeToString(c.b); err != nil {
				t.Errorf("EncodeToString(%#v): %s", c.b, err)
			} else if s != c.s {
				t.Errorf("EncodeToString(%#v)\n want %q\n have %q", c.b, c.s, s)
			}
		})
	}
}

func TestEncode_RepeatedEmbedding(t *testing.T) {
	type Common struct{ X string }
	type Left struct{ Common }
	type Right struct{ Common }
	type DeepLeft struct{ Left }
	type DeepRight struct{ Left }
	type Recursive struct {
		*Recursive
		X string
	}
	type PointerLeft struct{ *Common }
	type PointerRight struct{ *Common }
	type Tagged struct {
		X string `form:"X"`
	}
	type TaggedLeft struct{ Tagged }
	type TaggedRight struct{ Tagged }
	type Untagged struct{ X string }
	type UntaggedPath struct{ Untagged }

	for _, c := range []struct {
		name string
		b    interface{}
		s    string
	}{
		{
			"same type through distinct paths",
			struct {
				Left
				Right
				Visible string
			}{Left{Common{"left"}}, Right{Common{"right"}}, "keep"},
			"Visible=keep",
		},
		{
			"multiplicity survives another embedding level",
			struct {
				DeepLeft
				DeepRight
				Visible string
			}{
				DeepLeft{Left{Common{"a"}}},
				DeepRight{Left{Common{"b"}}},
				"keep",
			},
			"Visible=keep",
		},
		{
			"repeated tagged fields are ambiguous",
			struct {
				TaggedLeft
				TaggedRight
			}{TaggedLeft{Tagged{"left"}}, TaggedRight{Tagged{"right"}}},
			"",
		},
		{
			"repeated tagged fields beat no untagged candidate",
			struct {
				TaggedLeft
				TaggedRight
				UntaggedPath
			}{TaggedLeft{Tagged{"left"}}, TaggedRight{Tagged{"right"}}, UntaggedPath{Untagged{"other"}}},
			"",
		},
		{
			"embedded pointers have the same conflict",
			struct {
				PointerLeft
				PointerRight
			}{PointerLeft{&Common{"left"}}, PointerRight{&Common{"right"}}},
			"",
		},
		{
			"nil first path does not select the second",
			struct {
				PointerLeft
				PointerRight
			}{PointerLeft{}, PointerRight{&Common{"right"}}},
			"",
		},
		{
			"nil second path does not select the first",
			struct {
				PointerLeft
				PointerRight
			}{PointerLeft{&Common{"left"}}, PointerRight{}},
			"",
		},
		{
			"shallower occurrence still wins",
			struct {
				Common
				Right
			}{Common{"shallow"}, Right{Common{"deep"}}},
			"X=shallow",
		},
		{
			"unique tag still wins at equal depth",
			struct {
				Left
				Right
				TaggedLeft
			}{Left{Common{"left"}}, Right{Common{"right"}}, TaggedLeft{Tagged{"tagged"}}},
			"X=tagged",
		},
		{
			"recursive embedding keeps the shallow field",
			Recursive{X: "keep"},
			"X=keep",
		},
		{
			"explicit outer tags keep separate paths",
			struct {
				Left  `form:"left"`
				Right `form:"right"`
			}{Left{Common{"a"}}, Right{Common{"b"}}},
			"left.X=a&right.X=b",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Run("string", func(t *testing.T) {
				if s, err := EncodeToString(c.b); err != nil {
					t.Fatal(err)
				} else if s != c.s {
					t.Errorf("want %q, have %q", c.s, s)
				}
			})
			t.Run("values", func(t *testing.T) {
				if vs, err := EncodeToValues(c.b); err != nil {
					t.Fatal(err)
				} else if s := vs.Encode(); s != c.s {
					t.Errorf("want %q, have %q", c.s, s)
				}
			})
			t.Run("writer", func(t *testing.T) {
				var w bytes.Buffer
				if err := NewEncoder(&w).Encode(c.b); err != nil {
					t.Fatal(err)
				} else if s := w.String(); s != c.s {
					t.Errorf("want %q, have %q", c.s, s)
				}
			})
		})
	}
}
