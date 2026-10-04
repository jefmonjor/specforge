# Go Standard — SDD-Free

## 1. Clean Architecture y Estructura
* `cmd/`: Punto de entrada de la aplicación y adaptadores de CLI (Cobra).
* `internal/domain/`: Entidades de negocio puras, lógica sin dependencias externas.
* `internal/ports/`: Interfaces y contratos (DIP).
* `internal/adapters/`: Implementaciones de infraestructura (base de datos, HTTP, sistema de archivos).

## 2. Convenciones de Código
* Formato estricto con `gofmt` / `goimports`.
* Linter corporativo: `golangci-lint` (o `go vet`).
* Manejo explícito de errores (`if err != nil`), prohibido ignorar errores silenciosamente.

## 3. Pruebas y TDD
* Pruebas basadas en tablas (*table-driven tests*).
* Cobertura de tests unitarios >= 80%.
* Mocks manuales o interfaces en el paquete que las consume, no en el que las implementa.
