# Plan Técnico NNNN — <Título de la funcionalidad>

> **Craftsmanship Note**: El plan traduce el negocio (la Spec) a decisiones de ingeniería puras. Debe respetar estrictamente los principios SOLID, Clean Architecture y las directrices del framework base (`sdd-baseline`).

- **Spec de Negocio asociada:** `specs/NNNN-*.md`
- **Standards aplicables:** `java.md` / `deploy-ocp4.md` / `ci-*.md`

---

## 1. Matriz de Impacto Predictivo (Pre-Plan Guard)
> *NOTA PARA LA IA: Utilizar CodeGraph o `graphify explore` para rellenar esta sección antes de proponer código.*
- **Componentes Alterados:** `<clases, ficheros o módulos directos>`
- **Impacto Colateral Detectado:** `<sistemas, batchs, flujos o dependientes que podrían romperse>`
- **Estrategia de Contención de Riesgos:** `<cómo aislar el fallo aplicando Inversión de Dependencias (DIP)>`

## 2. Arquitectura y Diseño (El Cómo)
*Describe el enfoque técnico basándote en la Arquitectura Hexagonal. ¿Cómo fluye el dato desde el exterior hasta el dominio?*

### 2.1 Capa de Dominio (Domain)
- **Modelos / Entidades:** `<Nuevas clases de dominio, 100% aisladas de frameworks (sin Spring ni JPA)>`
- **Value Objects:** `<Objetos de valor inmutables (ej. Money, Iban, CustomerId) para erradicar Primitive Obsession>`
- **Puertos de Entrada (Casos de Uso):** `<Interfaces que exponen la funcionalidad (1 caso de uso = 1 interfaz)>`
- **Puertos de Salida (Contratos de Infra):** `<Interfaces para persistencia o APIs externas (DIP/ISP)>`

### 2.2 Capa de Aplicación y Adaptadores
- **Controladores REST (In):** `<Nuevos endpoints, DTOs, versionado /v1>`
- **Persistencia (Out):** `<Modificaciones en JPA, Repositorios, migraciones SQL, mapeo explícito de JPA Entity a Domain Entity>`

## 3. Matriz de Principios SOLID (Verificación Obligatoria)
- [ ] **S (Single Responsibility)**: ¿Cada clase tiene una única razón para cambiar? ¿Los casos de uso están atomizados en clases separadas?
- [ ] **O (Open/Closed)**: ¿Las variaciones de comportamiento se resuelven mediante polimorfismo/Strategy en vez de árboles `if/else`?
- [ ] **L (Liskov Substitution)**: ¿Los adaptadores sustituyen sus interfaces sin alterar el comportamiento esperado ni lanzar excepciones no declaradas?
- [ ] **I (Interface Segregation)**: ¿Los puertos son específicos para este cliente y no interfaces gigantes multi-propósito?
- [ ] **D (Dependency Inversion)**: ¿El dominio depende únicamente de abstracciones? ¿ArchUnit validará esta regla?

## 4. Contratos de API (Interfaces)
*Formato de intercambio de datos y errores esperados (RFC 7807).*
```json
// Ejemplo de Request/Response
```

## 5. Estrategia de Testing (TDD - Red / Green / Refactor)
*Mapea directamente los escenarios BDD de la Spec a pruebas automatizadas reales.*
1. **Paso 1 (RED)**: Generar primero los tests que fallan (mediante `sdd.ps1 test-scaffold`).
2. **Paso 2 (GREEN)**: Escribir el código estrictamente necesario para que los tests pasen (respetando YAGNI).
3. **Paso 3 (REFACTOR)**: Limpiar nombres, eliminar duplicidades y asegurar 0 code smells antes de Gate 3.

- [ ] **Unit Tests (Dominio puro):** `<Validación de reglas de negocio en milisegundos sin contextos de Spring>`
- [ ] **Architecture Tests (ArchUnit):** `<Verificación automática de reglas de dependencia hexagonal>`
- [ ] **Integration Tests (Adaptadores):** `<Validación de repositorios JPA y controladores REST con MockMvc>`

## 6. Operación y Despliegue (OCP4)
- **Observabilidad:** `<Métricas adicionales en Actuator/Prometheus>`
- **Configuración:** `<Nuevas variables de entorno requeridas en ConfigMaps o Secrets>`

## 7. Decisiones Arquitectónicas (ADR)
- *¿Esta feature amerita crear un Architecture Decision Record (ADR)? [Sí / No, y por qué]*

---
**🛡️ GATE 2 (Aprobación de Ingeniería):** El Agente de IA se detendrá aquí. Requiere validación explícita del Tech Lead, Arquitecto o pares antes de escribir una sola línea de código de producción. No usar *fire-and-forget*.
