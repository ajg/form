// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package form implements encoding and decoding of application/x-www-form-urlencoded data.
package form

const (
	implicitKey = "_"
	omittedKey  = "-"

	defaultDelimiter = '.'
	defaultEscape    = '\\'
	defaultKeepZeros = false

	// defaultMaxSize is the Decoder's zero value for its explicit slice-size
	// cap. Zero means "no explicit cap": use the proportional bound below.
	// See Decoder.MaxSize.
	defaultMaxSize = 0

	// The Decoder bounds how large a slice it will grow in response to explicit
	// indices in the input, so a small payload with a large index (e.g.
	// "Foo.900000000=x") cannot force a huge allocation and OOM the process.
	//
	// The bound is proportional to the number of elements actually supplied for
	// the slice: a legitimately large slice arrives as that many entries, so
	// real data is never rejected, while a tiny payload with one enormous index
	// is. sliceGrowthFactor is how many slice positions each supplied element
	// may span (tolerating some sparseness); sliceGrowthFloor is the minimum
	// length always permitted, so modest sparse inputs work out of the box.
	// Both are only defaults — see Decoder.MaxSize to set an absolute cap or to
	// disable the bound entirely for trusted input.
	sliceGrowthFactor = 16
	sliceGrowthFloor  = 1024

	// The Decoder also bounds key-path nesting depth, so a single key with many
	// delimiters (e.g. "a.a.a.…=x") cannot drive deep recursion or nested-map
	// allocation into a fatal stack overflow or OOM. Legitimate nesting is
	// bounded by the destination Go type, so builtinMaxDepth sits far above any
	// real form; see Decoder.MaxDepth to override or disable it. defaultMaxDepth
	// is the Decoder's zero value (0 => use builtinMaxDepth).
	defaultMaxDepth = 0
	builtinMaxDepth = 10000
)
