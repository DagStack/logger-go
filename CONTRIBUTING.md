# Contributing

Thanks for your interest! A few light conventions to keep review fast.

## Environment

- Go 1.22+ (see `go.mod`).
- `make test` before commit — required.
- `staticcheck` — optional, but appreciated.

## Code style

- `gofmt -s` is required (`make fmt`).
- Public types and functions get doc comments in English, starting with the
  subject's name (`// Logger is the primary handle for emitting LogRecords...`).
- Tests live in the `logger_test` package (external test package) so they
  exercise only the public API. White-box access is reserved for test
  helpers (`testhelpers_test.go` or `export_test.go`).

## Commit structure

- Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`.
- First line — imperative, English. The body may use Russian (rationale,
  motivation, links to issues).
- No `Co-Authored-By Claude`, no AI attribution.

## Identity for dagstack repositories

```bash
git config user.name "Evgenii Demchenko"
git config user.email "demchenkoev@gmail.com"
```

## Roadmap-driven phases

Implementation lands in phases (see README). A new phase is its own PR with
a checklist:

- [ ] All previous phases are complete and merged.
- [ ] `make test` green.
- [ ] `make vet` clean.
- [ ] Coverage on changed files >= 80%.
- [ ] `CHANGELOG.md` updated.
- [ ] Architect review passed.
