# Reglas Prohibidas (NG Rules — Never-Go Anti-Patterns)

El incumplimiento de cualquiera de estas reglas provocará el aborto inmediato del ciclo de desarrollo:

1. **NO a Frameworks en Dominio:** NUNCA importes anotaciones de Spring (`@Service`, `@Autowired`, `@Component`), JPA (`@Entity`, `@Table`, `@Column`) ni librerías de infraestructura dentro del paquete de dominio.
2. **NO a Tipos Primitivos Sueltos (Primitive Obsession):** NUNCA modeles conceptos de negocio críticos (IBAN, importes de dinero, divisas, identificadores de cliente) como simples `String` o `Double`. Encapsúlalos en Value Objects inmutables con validación en construcción.
3. **NO a Tipos Inseguros:** NUNCA uses `any` o type assertions no verificadas en TypeScript. En Go, prohíbe `interface{}` vacíos cuando existan contratos tipados.
4. **NO a Excepciones Silenciadas:** NUNCA captures excepciones sin registrarlas en el log corporativo (`catch (Exception e) {}` está prohibido).
5. **NO a Código Duplicado (DRY estricto):** NUNCA copies y pegues bloques de lógica. El umbral corporativo de duplicación es **0%** verificado por `jscpd`.
6. **NO a Código Muerto ni Dependencias Huérfanas:** Cero funciones sin llamar, variables no leídas o paquetes innecesarios en `package.json` o `pom.xml` (verificado por `knip`).
7. **NO a Tests Triviales o Débiles:** Las pruebas unitarias deben resistir pruebas de mutación (Stryker) y matar mutantes lógicos. Prohibidas las aserciones tautológicas (`assertTrue(true)`).
8. **NO a Sentencias SQL Crudas en Negocio:** Prohibido acoplar la lógica de dominio a esquemas relacionales o sentencias SQL directas.
