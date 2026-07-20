// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"errors"
	"strings"
	"testing"
)

// MaxBytes completes the resource-bounding triad (with MaxSize and MaxDepth):
// it caps how much raw input Decode will buffer before parsing.
func TestMaxBytes(t *testing.T) {
	payload := "a=" + strings.Repeat("x", 98) // 100 bytes total

	var dst struct {
		A string `form:"a"`
	}

	// Over the bound: a typed limit error.
	err := NewDecoder(strings.NewReader(payload)).MaxBytes(99).Decode(&dst)
	var fe *Error
	if err == nil || !errors.As(err, &fe) || fe.Kind != KindLimit {
		t.Errorf("over bound: got %v, want KindLimit", err)
	}

	// Exactly at the bound: decodes.
	if err := NewDecoder(strings.NewReader(payload)).MaxBytes(100).Decode(&dst); err != nil {
		t.Errorf("at bound: %v", err)
	} else if len(dst.A) != 98 {
		t.Errorf("at bound: got %d bytes", len(dst.A))
	}

	// Default is unbounded (compatibility): the same input decodes with no
	// bound configured.
	if err := NewDecoder(strings.NewReader(payload)).Decode(&dst); err != nil {
		t.Errorf("default: %v", err)
	}

	// The bound does not apply to already-materialized input.
	d := NewDecoder(nil).MaxBytes(1)
	if err := d.DecodeString(&dst, payload); err != nil {
		t.Errorf("DecodeString exempt: %v", err)
	}
}
