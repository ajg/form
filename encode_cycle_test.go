package form

import "testing"

func TestEncodeSelfReferentialSliceErrors(t *testing.T) {
	s := make([]interface{}, 1)
	s[0] = s
	if _, err := EncodeToString(s); err == nil {
		t.Fatal("expected a cycle error, got nil")
	}
}

func TestEncodeNonCyclicSliceStillWorks(t *testing.T) {
	if _, err := EncodeToString([]int{1, 2, 3}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	base := []int{1, 2, 3}
	v := struct{ A, B []int }{A: base[:2], B: base[1:]}
	if _, err := EncodeToString(v); err != nil {
		t.Fatalf("unexpected error on shared-backing siblings: %v", err)
	}
}
