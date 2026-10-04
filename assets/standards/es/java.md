### Java / Kotlin (Maven o Gradle)
- Estructura hexagonal: `domain` (entidades, value objects, reglas) sin imports de Spring, JPA ni otros frameworks; `application` (casos de uso, uno por clase); `adapters` (web, persistencia, clientes).
- Value objects en lugar de primitivos sueltos para conceptos de negocio (dinero, identificadores, emails).
- Solo inyección por constructor; nunca por campo.
- JUnit 5 + AssertJ; un test de ArchUnit mantiene el dominio libre de imports de frameworks.
