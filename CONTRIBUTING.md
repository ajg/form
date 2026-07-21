# Contributing to form

Thanks for your interest. `form` is a small, stable library, and contributions
are welcome — with a bias toward keeping it small.

## Ground rules

- **The public API and the wire format are stable.** A change that alters the
  decoded or encoded result of an existing input, or an existing error message
  string, needs a strong justification and a compatibility note in the PR.
  Additive, opt-in changes are much easier to accept than changes to defaults.
- **No new dependencies.** The library is standard-library-only by design, and
  intends to stay that way.
- **CI is the gate.** A change lands when CI is green; review is a norm, not a
  required approval. So make CI easy to trust.

## Before opening a PR

From the repo root:

```
gofmt -l .      # must print nothing
go vet ./...
go test ./...
```

The root package and `./multipart` must both pass. If you change decoding or
encoding behavior, add a test that pins it — ideally one that shows the exact
before/after on the wire.

The build targets Go 1.17 and up. The fuzz target is guarded by a `go1.18`
build tag; on a newer toolchain,
`go test -run='^$' -fuzz=FuzzDecodeString` exercises it.

## Scope

Bug fixes, tests, and documentation are always welcome. For a new feature or
anything that changes behavior, please open an issue first so we can talk it
through before you invest time — several directions are deliberately deferred
to a future v2 (see **Future Work** in the README).

## Commits

Keep each PR to one concern. Write commit messages in the imperative mood
("Add …", "Fix …"). PRs are squash-merged, so the PR title becomes the commit
subject on `master`.

By contributing, you agree that your work is licensed under the repository's
BSD-style license (see LICENSE).
