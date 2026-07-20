// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type node map[string]interface{}

func (n node) values(d, e rune) url.Values {
	vs := url.Values{}
	n.merge(d, e, "", &vs)
	return vs
}

func (n node) merge(d, e rune, p string, vs *url.Values) {
	for k, x := range n {
		switch y := x.(type) {
		case string:
			vs.Add(p+escape(d, e, k), y)
		case node:
			y.merge(d, e, p+escape(d, e, k)+string(d), vs)
		default:
			panic("value is neither string nor node")
		}
	}
}

func parseValues(d, e rune, vs url.Values, canIndexFirstLevelOrdinally bool, maxDepth int) node {
	// NOTE: Because of the flattening of potentially multiple strings to one key, implicit indexing works:
	//    i. At the first level;   e.g. Foo.Bar=A&Foo.Bar=B     becomes 0.Foo.Bar=A&1.Foo.Bar=B
	//   ii. At the last level;    e.g. Foo.Bar._=A&Foo.Bar._=B becomes Foo.Bar.0=A&Foo.Bar.1=B
	// TODO: At in-between levels; e.g. Foo._.Bar=A&Foo._.Bar=B becomes Foo.0.Bar=A&Foo.1.Bar=B
	//       (This last one requires that there only be one placeholder in order for it to be unambiguous.)

	m := map[string]string{}
	for k, ss := range vs {
		indexLastLevelOrdinally := strings.HasSuffix(k, string(d)+implicitKey)

		for i, s := range ss {
			// Derive each indexed key from the original k; mutating k in
			// place would compound across values (the second value of a
			// repeated key becoming "1."+"0."+k, etc.).
			key := k
			if canIndexFirstLevelOrdinally {
				key = strconv.Itoa(i) + string(d) + k
			} else if indexLastLevelOrdinally {
				key = strings.TrimSuffix(k, implicitKey) + strconv.Itoa(i)
			}

			m[key] = s
		}
	}

	// Split in sorted key order so outcomes never depend on map iteration
	// order: a scalar key is a strict prefix of any composite key through it,
	// and prefixes sort first, so the scalar is always absorbed into the node
	// (rather than the node colliding with a later scalar).
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	n := node{}
	for _, k := range keys {
		n = n.split(d, e, k, m[k], 1, maxDepth)
	}
	return n
}

func splitPath(d, e rune, path string) (k, rest string) {
	esc := false
	for i, r := range path {
		switch {
		case !esc && r == e:
			esc = true
		case !esc && r == d:
			return unescape(d, e, path[:i]), path[i+1:]
		default:
			esc = false
		}
	}
	return unescape(d, e, path), ""
}

func (n node) split(d, e rune, path, s string, depth, maxDepth int) node {
	if maxDepth >= 0 && depth > maxDepth {
		panic(errKind(KindLimit, "key path nesting exceeds the maximum depth ("+strconv.Itoa(maxDepth)+
			"); set Decoder.MaxDepth for trusted input"))
	}
	k, rest := splitPath(d, e, path)
	if rest == "" {
		return add(n, k, s)
	}
	if _, ok := n[k]; !ok {
		n[k] = node{}
	}

	c := getNode(n[k])
	n[k] = c.split(d, e, rest, s, depth+1, maxDepth)
	return n
}

func add(n node, k, s string) node {
	if n == nil {
		return node{k: s}
	}

	if _, ok := n[k]; ok {
		panic("key " + k + " already set") // Unreachable: sorted splitting absorbs scalars first.
	}

	n[k] = s
	return n
}

func isEmpty(x interface{}) bool {
	switch y := x.(type) {
	case string:
		return y == ""
	case node:
		if s, ok := y[""].(string); ok {
			return s == ""
		}
		return false
	}
	panic("value is neither string nor node")
}

func getNode(x interface{}) node {
	switch y := x.(type) {
	case string:
		return node{"": y}
	case node:
		return y
	}
	panic("value is neither string nor node")
}

func getString(x interface{}) string {
	switch y := x.(type) {
	case string:
		return y
	case node:
		if s, ok := y[""].(string); ok {
			return s
		}
		return ""
	}
	panic("value is neither string nor node")
}

func escape(d, e rune, s string) string {
	s = strings.Replace(s, string(e), string(e)+string(e), -1) // Escape the escape    (\ => \\)
	s = strings.Replace(s, string(d), string(e)+string(d), -1) // Escape the delimiter (. => \.)
	return s
}

func unescape(d, e rune, s string) string {
	s = strings.Replace(s, string(e)+string(d), string(d), -1) // Unescape the delimiter (\. => .)
	s = strings.Replace(s, string(e)+string(e), string(e), -1) // Unescape the escape    (\\ => \)
	return s
}
