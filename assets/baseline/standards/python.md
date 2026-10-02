# Python Standard — SDD-Free

## 1. Estructura y Entorno
* Python 3.11+ con gestión de dependencias moderna (`uv` o `poetry`).
* Tipado estático estricto con `mypy` / `type hints`.

## 2. Guardarraíles de Calidad
* **Linter & Formatter:** `ruff check .` y `ruff format .`
* **Anti-Duplicación:** `jscpd` con umbral 0%.
* **Arquitectura:** Pruebas de dependencias con `pytest-archon`.

## 3. Pruebas y TDD
* `pytest` con plugins `pytest-cov` y `pytest-mock`.
* Cobertura mínima >= 80%.
