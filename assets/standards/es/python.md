### Python
- Python 3.11+, estructura `src/`, type hints en toda función pública; limpio en `mypy` cuando esté configurado.
- `ruff check` y `ruff format` limpios, usando el `.venv` del proyecto.
- Los módulos de dominio no importan frameworks ni librerías de E/S; los adaptadores los envuelven. `dataclass(frozen=True)` o clases pequeñas para value objects; `Decimal` para dinero, nunca `float`.
- Los errores son clases de excepción concretas, nunca `except:` a secas ni cadenas de error devueltas.
- `pytest` con asserts simples, fixtures y `pytest.mark.parametrize`; sin red ni reloj real en los tests unitarios (inyéctalos).
