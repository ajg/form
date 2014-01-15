Form
====

A form encoding & decoding package for Go, written by [Alvaro J. Genial](http://alva.ro).

Synopsis
--------

This library is designed to allow seamless, high-fidelity encoding and decoding of arbitrary data in `application/x-www-form-urlencoded` format. It is intended to be useful primarily in dealing with web forms and URI query strings, both of which natively employ said format.

Unsurprisingly, `form` is modeled after other Go encoding packages, in particular `encoding/json`, and follows the same conventions. It aims to handle any kind of Go data value (i.e., not functions, channels, etc.)

Usage
-----

```go
import "github.com/ajg/form"
```

Given a data type such as the following...

```go
type User struct {
	Name        string            `form:"name"`
	Email       string            `form:"email"`
	Joined      time.Time         `form:"joined,omitempty"`
	Posts       []int             `form:"posts"`
	Preferences map[string]string `form:"prefs"`
	Hash        int64             `form:"-"`
}
```

...you can now easily encode such values...


```go
func PostUser(url string, u User) error {
	var c http.Client
	_, err := c.PostForm(url, form.EncodeToValues(u))
	return err
}
```

...as well as decode them...


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


...all without having to do any manual form parsing.



License
-------

This library is distributed under a BSD-style [LICENSE](./LICENSE).
