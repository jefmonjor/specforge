### Python
- Python 3.11+, `src/` layout, type hints on every public function; `mypy`-clean when configured.
- `ruff check` and `ruff format` clean, using the project's `.venv`.
- Domain modules import no framework or I/O library; adapters wrap them. `dataclass(frozen=True)` or small classes for value objects; `Decimal` for money, never `float`.
- Errors are specific exception classes, never bare `except:` or returned error strings.
- `pytest` with plain asserts, fixtures and `pytest.mark.parametrize`; no network or real clock in unit tests (inject them).
