// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form_test

import (
	"fmt"
	"strings"

	"github.com/ajg/form"
)

func ExampleDecodeString() {
	type profile struct {
		Name string   `form:"name"`
		Age  int      `form:"age"`
		Tags []string `form:"tags"`
	}
	var p profile
	if err := form.DecodeString(&p, "name=Alice&age=30&tags.0=go&tags.1=forms"); err != nil {
		panic(err)
	}
	fmt.Printf("%s is %d; tags=%v", p.Name, p.Age, p.Tags)
	// Output: Alice is 30; tags=[go forms]
}

func ExampleEncodeToString() {
	type profile struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}
	// Keys come out sorted, so encoding is deterministic.
	s, err := form.EncodeToString(&profile{Name: "Alice", Age: 30})
	if err != nil {
		panic(err)
	}
	fmt.Println(s)
	// Output: age=30&name=Alice
}

func ExampleDecoder() {
	// Decode directly from any io.Reader — e.g. an HTTP request body.
	// Nested keys use dotted paths.
	type request struct {
		User struct {
			Name string `form:"name"`
			Age  int    `form:"age"`
		} `form:"user"`
	}
	var req request
	if err := form.NewDecoder(strings.NewReader("user.name=Bob&user.age=42")).Decode(&req); err != nil {
		panic(err)
	}
	fmt.Printf("%+v", req)
	// Output: {User:{Name:Bob Age:42}}
}

func ExampleDecoder_MaxBytes() {
	// Bound how much raw input a Decoder will buffer from an untrusted
	// stream; oversized input fails instead of being read into memory.
	var m map[string]string
	oversized := "a=" + strings.Repeat("x", 1000)
	err := form.NewDecoder(strings.NewReader(oversized)).MaxBytes(32).Decode(&m)
	fmt.Println(err != nil)
	// Output: true
}
