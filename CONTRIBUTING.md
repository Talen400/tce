# Contributing

## Setup

- Go 1.24+
- Clone the repo, run `make build` to compile

## Development

```bash
make build   # compile
make test    # run all tests
make bench   # run benchmarks
make clean   # remove binary
```

## Lint

We use `golangci-lint`. Run before pushing:

```bash
golangci-lint run
```

CI also runs lint on every PR.

## Tests

All tests use the standard `testing` package — no external test frameworks.

```bash
go test ./... -count=1 -v
```

Write tests for new code. Place them in `*_test.go` next to the source file.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add .tceignore support for glob/grep
fix: prevent panic on empty tool result
docs: add CONTRIBUTING.md
chore: update dependencies
refactor: extract SSE parser to separate type
test: add TestIgnoreMatcher
```

## Pull Requests

- Keep PRs focused on a single concern
- Rebase onto the target branch before opening
- Squash fixup commits before merge
- Ensure CI (build + vet + lint + test) passes
