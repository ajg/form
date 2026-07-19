// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"errors"
	"strings"
	"testing"
	"unicode"
)

// snake is a deliberately simple example mapper: an underscore before every
// non-initial uppercase rune, then lowercase. (Real users may prefer a
// library with initialism handling; form only requires consistency, not
// linguistic taste.)
func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

type account struct {
	UserName string `form:"explicit"` // Tagged: exempt from the mapping.
	HomeCity string
	Profile  profile
}

type profile struct {
	AvatarUrl string
	Age       int
}

func TestKeysWithDecode(t *testing.T) {
	var dst account
	d := NewDecoder(nil).KeysWith(snake)
	err := d.DecodeString(&dst, "explicit=alice&home_city=lisbon&profile.avatar_url=http%3A%2F%2Fx&profile.age=30")
	if err != nil {
		t.Fatal(err)
	}
	if dst.UserName != "alice" || dst.HomeCity != "lisbon" ||
		dst.Profile.AvatarUrl != "http://x" || dst.Profile.Age != 30 {
		t.Errorf("got %+v", dst)
	}
}

func TestKeysWithEncodeAndRoundTrip(t *testing.T) {
	src := account{
		UserName: "alice",
		HomeCity: "lisbon",
		Profile:  profile{AvatarUrl: "http://x", Age: 30},
	}
	var sb stringWriter
	if err := NewEncoder(&sb).KeysWith(snake).Encode(&src); err != nil {
		t.Fatal(err)
	}
	s := sb.String()
	for _, want := range []string{"explicit=alice", "home_city=lisbon", "profile.avatar_url=", "profile.age=30"} {
		if !strings.Contains(s, want) {
			t.Errorf("encoded %q lacks %q", s, want)
		}
	}

	var dst account
	if err := NewDecoder(nil).KeysWith(snake).DecodeString(&dst, s); err != nil {
		t.Fatalf("re-decode of %q: %v", s, err)
	}
	if dst != src {
		t.Errorf("round trip: got %+v, want %+v", dst, src)
	}
}

func TestKeysWithTagsExempt(t *testing.T) {
	// The tag names the key verbatim: the mapper must not apply, so the
	// mapped spelling of the field name is an unknown key.
	var dst account
	err := NewDecoder(nil).KeysWith(snake).DecodeString(&dst, "user_name=alice")
	if err == nil {
		t.Error("expected unknown-key error for mapped spelling of a tagged field")
	}
}

func TestKeysWithUntransformedKeyNoLongerMatches(t *testing.T) {
	// With a mapper set, the raw field name is not a valid key.
	var dst account
	err := NewDecoder(nil).KeysWith(snake).DecodeString(&dst, "HomeCity=lisbon")
	if err == nil {
		t.Error("expected unknown-key error for untransformed field name")
	}
}

func TestKeysWithNilIsIdentity(t *testing.T) {
	// Without a mapper, behavior is exactly as before.
	var dst account
	if err := DecodeString(&dst, "HomeCity=lisbon"); err != nil {
		t.Fatal(err)
	}
	if dst.HomeCity != "lisbon" {
		t.Errorf("got %+v", dst)
	}
}

func TestKeysWithIgnoreCaseComposes(t *testing.T) {
	// IgnoreCase applies to the transformed keys.
	var dst account
	d := NewDecoder(nil).KeysWith(snake)
	d.IgnoreCase(true)
	if err := d.DecodeString(&dst, "HOME_CITY=lisbon"); err != nil {
		t.Fatal(err)
	}
	if dst.HomeCity != "lisbon" {
		t.Errorf("got %+v", dst)
	}
}

func TestKeysWithNilClears(t *testing.T) {
	d := NewDecoder(nil).KeysWith(snake)
	var v account
	if err := d.DecodeString(&v, "home_city=a"); err != nil {
		t.Fatalf("mapped: %v", err)
	}
	d.KeysWith(nil)
	v = account{}
	if err := d.DecodeString(&v, "HomeCity=b"); err != nil || v.HomeCity != "b" {
		t.Errorf("nil should restore default matching: err=%v v=%+v", err, v)
	}
	if err := d.DecodeString(&v, "home_city=c"); err == nil {
		t.Error("after clearing, the mapped spelling should be unknown again")
	}
}

func TestKeysWithPanickingMapper(t *testing.T) {
	boom := func(s string) string { panic("mapper exploded on " + s) }
	var v account
	err := NewDecoder(nil).KeysWith(boom).DecodeString(&v, "home_city=x")
	var fe *Error
	if err == nil || !errors.As(err, &fe) || fe.Op != OpDecode {
		t.Errorf("decode: panicking mapper must become a typed error, got %v", err)
	}
	var sb stringWriter
	err = NewEncoder(&sb).KeysWith(boom).Encode(&account{HomeCity: "x"})
	if err == nil || !errors.As(err, &fe) || fe.Op != OpEncode {
		t.Errorf("encode: panicking mapper must become a typed error, got %v", err)
	}
}

// TestKeysWithBeforeAndAfter is the specification at a glance: one struct,
// encoded first without a mapper (keys are the Go field names) and then with
// the snake mapper (keys are the wire convention), each as an exact, full
// payload — and the mapped payload decodes back to the identical struct.
func TestKeysWithBeforeAndAfter(t *testing.T) {
	src := account{
		UserName: "alice", // tagged `form:"explicit"`: exempt from mapping
		HomeCity: "lisbon",
		Profile:  profile{AvatarUrl: "http://x", Age: 30},
	}

	// Before — no mapper; keys are the field names themselves:
	var before stringWriter
	if err := NewEncoder(&before).Encode(&src); err != nil {
		t.Fatal(err)
	}
	const wireBefore = "HomeCity=lisbon" +
		"&Profile.Age=30" +
		"&Profile.AvatarUrl=http%3A%2F%2Fx" +
		"&explicit=alice"
	if before.String() != wireBefore {
		t.Errorf("without mapper:\n got  %q\n want %q", before.String(), wireBefore)
	}

	// After — with the snake mapper; same struct, wire-convention keys
	// (the tagged field is untouched):
	var after stringWriter
	if err := NewEncoder(&after).KeysWith(snake).Encode(&src); err != nil {
		t.Fatal(err)
	}
	const wireAfter = "explicit=alice" +
		"&home_city=lisbon" +
		"&profile.age=30" +
		"&profile.avatar_url=http%3A%2F%2Fx"
	if after.String() != wireAfter {
		t.Errorf("with mapper:\n got  %q\n want %q", after.String(), wireAfter)
	}

	// And back: the mapped payload decodes to the identical struct.
	var got account
	if err := NewDecoder(nil).KeysWith(snake).DecodeString(&got, wireAfter); err != nil {
		t.Fatal(err)
	}
	if got != src {
		t.Errorf("round trip:\n got  %+v\n want %+v", got, src)
	}
}
