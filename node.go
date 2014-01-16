// Copyright 2014 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"net/url"
	"strconv"
	"strings"
)

type node map[string]interface{}

func (n node) Values() url.Values {
	vs := url.Values{}
	n.merge("", &vs)
	return vs
}

func (n node) merge(p string, vs *url.Values) {
	for k, x := range n {
		switch y := x.(type) {
		case string:
			vs.Add(p+k, y)
		case node:
			y.merge(p+k+".", vs)
		default:
			panic("value is neither string nor node")
		}
	}
}

func parseValues(vs url.Values, canIndexOrdinally bool) node {
	m := map[string]string{}
	for k, ss := range vs {
		for i, s := range ss {
			if canIndexOrdinally {
				m[strconv.Itoa(i)+"."+k] = s
			} else {
				m[k] = s
			}
		}
	}

	n := node{}
	for k, s := range m {
		n = n.split(k, s)
	}
	return n
}

func (n node) split(path, s string) node {
	ps := strings.SplitN(path, ".", 2)
	switch len(ps) {
	case 2:
		k, rest := ps[0], ps[1]
		if _, ok := n[k]; !ok {
			n[k] = node{}
		}

		c := getNode(n[k])
		n[k] = c.split(rest, s)
		return n
	}
	return add(n, ps[0], s)
}

func add(n node, k, s string) node {
	if n == nil {
		return node{k: s}
	}
	n[k] = s
	return n
}

func isEmpty(x interface{}) bool {
	switch y := x.(type) {
	case string:
		return y == ""
	case node:
		return len(y) == 0
	}
	panic("value is neither string nor node")
}

func getNode(x interface{}) node {
	switch y := x.(type) {
	case string:
		if y == "" {
			return node{}
		}
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
