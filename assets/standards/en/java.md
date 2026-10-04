### Java / Kotlin (Maven or Gradle)
- Hexagonal layout: `domain` (entities, value objects, rules) without Spring, JPA or other framework imports; `application` (use cases, one per class); `adapters` (web, persistence, clients).
- Value objects instead of primitive obsession for business concepts (money, identifiers, emails).
- Constructor injection only; no field injection.
- JUnit 5 + AssertJ; an ArchUnit test keeps the domain free of framework imports.
