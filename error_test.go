package form

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorIsTypedOnDecode(t *testing.T) {
	var dst struct{ A int }
	err := DecodeString(&dst, "B=1")
	if err == nil {
		t.Fatal("expected an error")
	}
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatalf("expected *form.Error, got %T", err)
	}
	if fe.Op != OpDecode {
		t.Errorf("Op = %q, want decode", fe.Op)
	}
	if !strings.Contains(err.Error(), "doesn't exist") {
		t.Errorf("message not preserved: %q", err.Error())
	}
}

func TestErrorIsTypedOnEncode(t *testing.T) {
	s := make([]interface{}, 1)
	s[0] = s
	if _, err := EncodeToString(s); err == nil {
		t.Fatal("expected an error")
	} else {
		var fe *Error
		if !errors.As(err, &fe) {
			t.Fatalf("expected *form.Error, got %T", err)
		}
		if fe.Op != OpEncode {
			t.Errorf("Op = %q, want encode", fe.Op)
		}
	}
}

type textFailer struct{}

var errTextFail = errors.New("text failer boom")

func (*textFailer) UnmarshalText([]byte) error { return errTextFail }

func TestErrorUnwrapReachesCause(t *testing.T) {
	var dst struct{ F textFailer }
	err := DecodeString(&dst, "F=x")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, errTextFail) {
		t.Fatalf("errors.Is could not reach cause; err=%v", err)
	}
	var fe *Error
	if !errors.As(err, &fe) || fe.Err != errTextFail {
		t.Fatalf("expected wrapped cause; got %#v", fe)
	}
}
