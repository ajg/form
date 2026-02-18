TODO
====

  - Document IgnoreCase and IgnoreUnknownKeys in README.
  - An option to automatically treat all field names in `camelCase` or `underscore_case`.
  - Built-in support for the types in [`math/big`](http://golang.org/pkg/math/big/).
  - Built-in support for the types in [`image/color`](http://golang.org/pkg/image/color/).
  - Improve encoding/decoding by reading/writing directly from/to the `io.Reader`/`io.Writer` when possible, rather than going through an intermediate representation (i.e. `node`) which requires more memory.
