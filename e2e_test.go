// Copyright 2026 Alvaro J. Genial. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package form

import (
	"image/color"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

// These tests trade the surgical style of the rest of the suite for
// legibility: one realistic, human-readable submission decoded end to end,
// so a reader can see at a glance what the library does with a whole form.

type signup struct {
	Name    string      `form:"name"`
	Age     int         `form:"age"`
	Accent  color.NRGBA `form:"accent"`  // e.g. an <input type=color> value
	Balance *big.Int    `form:"balance"` // arbitrary precision
	Home    url.URL     `form:"home"`
	Since   time.Time   `form:"since"`
	Tags    []string    `form:"tags"`
	Address struct {
		City string `form:"city"`
		Zip  string `form:"zip"`
	} `form:"address"`
	News bool `form:"news"`
}

func TestMixedPayload(t *testing.T) {
	// As a browser would send it (percent-encoded); shown one field per
	// line for readability:
	payload := "name=Alice+Liddell" +
		"&age=30" +
		"&accent=%2300aaff" + // "#00aaff"
		"&balance=123456789012345678901234567890" +
		"&home=https%3A%2F%2Fexample.com%2Falice" +
		"&since=2016-03-24" +
		"&tags._=go&tags._=forms" + // implicit trailing index => slice
		"&address.city=Lisbon" +
		"&address.zip=1100-048" +
		"&news=true"

	var got signup
	if err := DecodeString(&got, payload); err != nil {
		t.Fatal(err)
	}

	want := signup{
		Name:    "Alice Liddell",
		Age:     30,
		Accent:  color.NRGBA{R: 0x00, G: 0xaa, B: 0xff, A: 0xff},
		Balance: bigInt(t, "123456789012345678901234567890"),
		Home:    url.URL{Scheme: "https", Host: "example.com", Path: "/alice"},
		Since:   time.Date(2016, 3, 24, 0, 0, 0, 0, time.UTC),
		Tags:    []string{"go", "forms"},
		News:    true,
	}
	want.Address.City = "Lisbon"
	want.Address.Zip = "1100-048"

	if !reflect.DeepEqual(got, want) {
		t.Errorf("decoded submission diverges:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestEndToEndHTTP round-trips through an actual HTTP server: a client posts
// a form, the handler decodes the request body directly, exactly as the
// README suggests.
func TestEndToEndHTTP(t *testing.T) {
	type subscription struct {
		Email  string      `form:"email"`
		Accent color.NRGBA `form:"accent"`
		Plan   string      `form:"plan"`
	}

	var got subscription
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	resp, err := http.PostForm(srv.URL, url.Values{
		"email":  {"alice@example.com"},
		"accent": {"#ff8800"},
		"plan":   {"pro"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("handler rejected the form: %d", resp.StatusCode)
	}

	want := subscription{
		Email:  "alice@example.com",
		Accent: color.NRGBA{R: 0xff, G: 0x88, B: 0x00, A: 0xff},
		Plan:   "pro",
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func bigInt(t *testing.T, s string) *big.Int {
	t.Helper()
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad big.Int literal %q", s)
	}
	return i
}
