package form

import (
	"strings"
	"testing"
)

// A multi-byte delimiter rune must round-trip: escape/unescape and node.merge
// already handle any rune, so splitPath must too.
func TestMultibyteDelimiterRoundTrip(t *testing.T) {
	type inner struct {
		Bar string `form:"Bar"`
	}
	type outer struct {
		Foo inner `form:"Foo"`
	}

	for _, d := range []rune{'|', '·', '→', '\U0001d11e'} {
		s, err := EncodeToStringWith(&outer{inner{"x"}}, d, defaultEscape, false)
		if err != nil {
			t.Fatalf("delimiter %q: encode: %v", d, err)
		}
		var got outer
		if err := NewDecoder(strings.NewReader(s)).DelimitWith(d).Decode(&got); err != nil {
			t.Errorf("delimiter %q (%d bytes): decode %q: %v", d, len(string(d)), s, err)
			continue
		}
		if got.Foo.Bar != "x" {
			t.Errorf("delimiter %q (%d bytes): round-trip of %q gave %q, want %q",
				d, len(string(d)), s, got.Foo.Bar, "x")
		}
	}
}

func TestSplitPathMultibyteDelimiter(t *testing.T) {
	for _, c := range []struct {
		d          rune
		path       string
		want, rest string
	}{
		{'|', "Foo|Bar|Qux", "Foo", "Bar|Qux"},
		{'·', "Foo·Bar·Qux", "Foo", "Bar·Qux"},
		{'→', "Foo→Bar→Qux", "Foo", "Bar→Qux"},
		{'\U0001d11e', "Foo\U0001d11eBar", "Foo", "Bar"},
		{'·', "Foo\\·Bar·Qux", "Foo·Bar", "Qux"},
	} {
		k, rest := splitPath(c.d, defaultEscape, c.path)
		if k != c.want || rest != c.rest {
			t.Errorf("splitPath(%q, %q, %q)\n want (%#v, %#v)\n have (%#v, %#v)",
				c.d, defaultEscape, c.path, c.want, c.rest, k, rest)
		}
	}
}
