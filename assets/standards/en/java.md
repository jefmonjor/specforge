### Java / Kotlin (Maven or Gradle)
- Hexagonal layout: `domain` (entities, value objects, rules) without Spring, JPA or other framework imports; `application` (use cases, one per class); `adapters` (web, persistence, clients). The ArchUnit test (`ArchitectureTest`) enforces it: never weaken it to make code pass.
- Modern Java (17/21): `record` for value objects and DTOs, `sealed` interfaces for closed alternatives, `switch` expressions with pattern matching, `java.time` for dates (never `Date`/`Calendar`), `List.of`/`Map.of`, `Optional` as a return type only, try-with-resources for anything closeable, `var` only where the type is obvious.
- Money is `BigDecimal` with an explicit `RoundingMode`, or a value object; never `double`.
- Jakarta EE (`jakarta.*`), not `javax.*`; SLF4J for logging, not Log4j 1 or `System.out`.
- Constructor injection only; no field injection; no static mutable state.
- JUnit 5 + AssertJ; one behaviour per test; `@ParameterizedTest` for tables of examples. `mvn pmd:check` clean.
