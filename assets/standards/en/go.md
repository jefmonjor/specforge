### Go
- Layout: `cmd/` for entry points, `internal/` for everything else; the domain package imports no I/O (`os`, `net`, `database/sql`).
- `gofmt` clean; `go vet` (and `golangci-lint` when configured) without findings.
- Errors: wrap with `%w`, compare with `errors.Is`/`errors.As`, never `_ =` on a write.
- Pass `context.Context` as the first parameter of anything that does I/O.
- Table-driven tests; fakes over mocks; `t.TempDir()` instead of shared directories.
