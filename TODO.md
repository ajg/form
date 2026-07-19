TODO
====

  - Improve encoding/decoding by reading/writing directly from/to the `io.Reader`/`io.Writer` when possible, rather than going through an intermediate representation (i.e. `node`) which requires more memory.
