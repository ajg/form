// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"reflect"
	"testing"
)

type foo int
type bar interface {
	void()
}
type qux struct{}
type zee []bar

func TestCanIndexOrdinally(t *testing.T) {
	for _, c := range []struct {
		x interface{}
		b bool
	}{
		{int(0), false},
		{foo(0), false},
		{qux{}, false},
		{(*int)(nil), false},
		{(*foo)(nil), false},
		{(*bar)(nil), false},
		{(*qux)(nil), false},
		{[]qux{}, true},
		{[5]qux{}, true},
		{&[]foo{}, true},
		{&[5]foo{}, true},
		{zee{}, true},
		{&zee{}, true},
		{map[int]foo{}, false},
		{map[string]interface{}{}, false},
		{map[interface{}]bar{}, false},
		{(chan<- int)(nil), false},
		{(chan bar)(nil), false},
		{(<-chan foo)(nil), false},
	} {
		v := reflect.ValueOf(c.x)
		if b := canIndexOrdinally(v); b != c.b {
			t.Errorf("canIndexOrdinally(%v)\nwant (%v)\nhave (%v)", v, c.b, b)
		}
	}
}

func TestEscape(t *testing.T) {
	for _, c := range []struct {
		a, b string
	}{
		{"Foo", "Foo"},
		{"Foo.Bar.Qux", "Foo\\.Bar\\.Qux"},
		{"0", "0"},
		{"0.1.2", "0\\.1\\.2"},
	} {
		if b := escape(c.a); b != c.b {
			t.Errorf("escape(%v)\nwant (%v)\nhave (%v)", c.a, c.b, b)
		}
	}
}

func TestUnescape(t *testing.T) {
	for _, c := range []struct {
		a, b string
	}{
		{"Foo", "Foo"},
		{"Foo.Bar.Qux", "Foo\\.Bar\\.Qux"},
		{"0", "0"},
		{"0.1.2", "0\\.1\\.2"},
	} {
		if a := unescape(c.b); a != c.a {
			t.Errorf("unescape(%v)\nwant (%v)\nhave (%v)", c.b, c.a, a)
		}
	}
}
