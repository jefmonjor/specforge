### Java / Kotlin (Maven o Gradle)
- Estructura hexagonal: `domain` (entidades, value objects, reglas) sin imports de Spring, JPA ni otros frameworks; `application` (casos de uso, uno por clase); `adapters` (web, persistencia, clientes). El test de ArchUnit (`ArchitectureTest`) lo impone: nunca lo relajes para que el código pase.
- Java moderno (17/21): `record` para value objects y DTOs, interfaces `sealed` para alternativas cerradas, `switch` como expresión con pattern matching, `java.time` para fechas (nunca `Date`/`Calendar`), `List.of`/`Map.of`, `Optional` solo como tipo de retorno, try-with-resources para todo lo que se cierra, `var` solo donde el tipo es evidente.
- El dinero es `BigDecimal` con `RoundingMode` explícito, o un value object; nunca `double`.
- Jakarta EE (`jakarta.*`), no `javax.*`; SLF4J para logging, no Log4j 1 ni `System.out`.
- Solo inyección por constructor; nunca por campo; sin estado estático mutable.
- JUnit 5 + AssertJ; un comportamiento por test; `@ParameterizedTest` para tablas de ejemplos. `mvn pmd:check` limpio.
