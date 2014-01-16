Form
====

A form encoding & decoding package for Go, written by [Alvaro J. Genial](http://alva.ro).

Synopsis
--------

This library is designed to allow seamless, high-fidelity encoding and decoding of arbitrary data in `application/x-www-form-urlencoded` format and as [`url.Values`](http://golang.org/pkg/net/url/#Values). It is intended to be useful primarily in dealing with web forms and URI query strings, both of which natively employ said format.

Unsurprisingly, `form` is modeled after other Go [`encoding`](http://golang.org/pkg/encoding/) packages, in particular [`encoding/json`](http://golang.org/pkg/encoding/json/), and follows the same conventions (see below for more.) It aims to automatically handle any kind of concrete Go data value (i.e., not functions, channels, etc.) while providing mechanisms for custom behavior.

Status
------

The implementation is in usable shape and is fairly well tested with its accompanying test suite. The API is unlikely to change much, but still may. Lastly, the code has not yet undergone a security review to ensure it is free of vulnerabilities. Please file an issue or send a pull request for fixes & improvements.

(Note: Proper `godoc`-style documentation is in the works; for now, there is this document and the source.)

Usage
-----

```go
import "github.com/ajg/form"
```

Given a type like the following...

```go
type User struct {
	Name         string            `form:"name"`
	Email        string            `form:"email"`
	Joined       time.Time         `form:"joined,omitempty"`
	Posts        []int             `form:"posts"`
	Preferences  map[string]string `form:"prefs"`
	Avatar       []byte            `form:"avatar"`
	PasswordHash int64             `form:"-"`
}
```

...it is easy to encode data of that type...


```go
func PostUser(url string, u User) error {
	var c http.Client
	_, err := c.PostForm(url, form.EncodeToValues(u))
	return err
}
```

...as well as decode it...


```go
func Handler(w http.ResponseWriter, r *http.Request) {
	var u User

	d := form.NewDecoder(r.Body)
	if err := d.Decode(&u); err != nil {
		http.Error(w, "Form could not be decoded", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Decoded: %#v", u)
}
```

...without having to do any grunt work.

Field Tags
----------

Like other encoding packages, `form` supports the following options for fields:

 - `` `form:"-"` ``: Causes the field to be ignored during encoding and decoding.
 - `` `form:"<name>"` ``: Overrides the field's name; useful especially when dealing with external identifiers in camelCase, as are commonly found on the web.
 - `` `form:",omitempty"` ``: Elides the field during encoding if it is empty (typically meaning equal to the type's zero value.)
 - `` `form:"<name>,omitempty"` ``: The way to combine the two options above.

Untyped Values
--------------

While encouraged, it is not necessary to define a type (e.g. a `struct`) in order to use `form`, since it is able to encode and decode untyped data generically using the following rules:

 - Scalar values (basic types and [`time.Time`](http://golang.org/pkg/time/#Time), including aliases thereof) will be treated as a `string`.
 - Compound values (everything else) will be treated as a `map[string]interface{}`, itself able to contain nested values (both scalar and compound) ad infinitum.
 - However, if there is a value (of any supported type) already present in a map for a given key, then it will be used when possible, rather than being replaced with a value as specified above; this makes it possible to handle partially typed, dynamic or schema-less values.


Custom Marshaling
-----------------

There is a default (generally lossless) marshaling & unmarshaling scheme for any concrete data value in Go, which is good enough in most cases. However, it is possible to override it and use a custom scheme. For instance, a "binary" field could be marshaled more efficiently using [base64](http://golang.org/pkg/encoding/base64/) to prevent it from being percent-escaped during serialization to `application/x-www-form-urlencoded` format.

Because `form` provides support for [`encoding.TextMarshaler`](http://golang.org/pkg/encoding/#TextMarshaler) and [`encoding.TextUnmarshaler`](http://golang.org/pkg/encoding/#TextUnmarshaler) it is easy to do that; for instance, like this:

```go
import "encoding"

type Binary []byte

var (
	_ encoding.TextMarshaler   = &Binary{}
	_ encoding.TextUnmarshaler = &Binary{}
)

func (b Binary) MarshalText() ([]byte, error) {
	return []byte(base64.URLEncoding.EncodeToString([]byte(b))), nil
}

func (b *Binary) UnmarshalText(text []byte) error {
	bs, err := base64.URLEncoding.DecodeString(string(text))
	if err == nil {
		*b = Binary(bs)
	}
	return err
}
```

Now any value with type `Binary` will automatically be encoded using the [URL](http://golang.org/pkg/encoding/base64/#URLEncoding) variant of base64. It is left as an exercise to the reader to improve upon this scheme by eliminating the need for padding (which, besides being superfluous, uses `=`, a character that will end up percent-escaped.)

Reference
---------

### Encoding

 - `form.NewEncoder(io.Writer)`: Returns a new encoder whose `Encode(interface{})` method encodes the provided value and writes it directly to the `io.Writer`.
 - `form.EncodeToString(interface{})`: Returns a `application/x-www-form-urlencoded` string of the provided value.
 - `form.EncodeToValues(interface{})`: Returns a `url.Values` encoding of the provided value.

### Decoding

 - `form.NewDecoder(io.Reader)`: Returns a new decoder whose `Decode(interface{})` method reads and decodes into the provided value directly from the `io.Reader`.
 - `form.DecodeString(interface{}, string)`: Decodes a `application/x-www-form-urlencoded` string into the provided value.
 - `form.DecodeValues(interface{}, url.Values)`: Decodes a `url.Values` directly into the provided value.

(Note: all forms of decoding require a pointer to the value being decoded into, in order to be able to mutate it; this restriction may be relaxed in the future for certain reference types like maps.)

Known Issues
------------

 - Maps with keys (or structs with custom field names) that contain a dot (`.`) are unlikely to work correctly.
 - At the moment there's no support for `complex64`/`complex128`.
 - Circular (self-referential) values are untested.

License
-------

This library is distributed under a BSD-style [LICENSE](./LICENSE).
