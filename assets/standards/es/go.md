### Go
- Estructura: `cmd/` para los puntos de entrada e `internal/` para todo lo demás; el paquete de dominio no importa E/S (`os`, `net`, `database/sql`).
- `gofmt` limpio; `go vet` (y `golangci-lint` si está configurado) sin hallazgos.
- Errores: envuelve con `%w`, compara con `errors.Is`/`errors.As`, nunca `_ =` sobre una escritura.
- `context.Context` como primer parámetro de todo lo que haga E/S.
- Tests por tabla; fakes antes que mocks; `t.TempDir()` en lugar de directorios compartidos.
