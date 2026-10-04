### Python
- Python 3.11+, type hints everywhere, `mypy`-clean when configured.
- `ruff check` clean; `ruff format` style.
- Domain modules import no framework or I/O library; adapters wrap them.
- `pytest` with plain asserts and fixtures; no network in unit tests.
