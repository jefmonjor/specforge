# Standard de Java (Servicios) — SDDFramework Open Source

Fuente autoritativa: **Guía de Arquitectura de Servicios Java — SDD Open Specification**.
Arquetipo de referencia: **seed-services** (Spring Boot 3 + Java 21).
Este standard **endurece**, nunca debilita, la constitución corporativa (§7.3).

## Imagen base
- **Java 21 OpenJDK** + **Tomcat 10.1** sobre contenedor Linux.
- Configuración por entorno vía **ConfigMap**; contraseñas/datos sensibles como
  **Secrets** de Kubernetes/OpenShift cargados como **variables de entorno**.
- Proxy para conexiones externas (excepción para internas).
- **Truststore** con las CA root corporativas + CA externas reconocidas.

## Arquitectura, Clean Code y SOLID (Mandatorios)
- **Principios SOLID y Clean Code**: Su aplicación es **estricta y obligatoria** en la redacción, refactorización y revisión de cualquier fragmento de código (IA o humano). Cada clase debe tener una única responsabilidad (SRP), las dependencias deben inyectarse mediante interfaces (DIP) y el código debe estar libre de "olores" (code smells), favoreciendo la legibilidad y el mantenimiento a largo plazo.
- **Modelo de Dominio Rico vs Modelo Anémico**: Queda **prohibido el modelo anémico** (entidades con simples getters/setters y servicios que orquestan toda la lógica). El comportamiento de negocio debe residir en las entidades y agregados del dominio aplicando el principio *Tell, Don't Ask*.
- **Value Objects contra la "Primitive Obsession"**: Los conceptos de negocio nunca se modelan como tipos primitivos sueltos (`String`, `Long`, `BigDecimal`). Deben encapsularse en Value Objects inmutables con validación en construcción (ej. `Iban`, `Money`, `Currency`, `CustomerId`).
- **Casos de Uso Granulares (SRP Radical)**: Prohibidas las clases "Dios" (`*ServiceImpl` con decenas de métodos). Cada caso de uso debe ser una clase/handler independiente y cohesivo (ej. `RealizarTransferenciaUseCase`, `ConsultarSaldoQuery`).
- **Cero Contaminación de Frameworks en Dominio**: El paquete `domain` NO puede importar `@Entity`, `@Table`, `@Column`, anotaciones de Spring (`@Service`, `@Autowired`, `@Component`), ni librerías de infraestructura. La persistencia se realiza mediante mapeo explícito en el adaptador (JPA Entity != Domain Entity).
- **Gate Arquitectónico Automatizado (ArchUnit)**: Es mandatorio incluir tests de arquitectura con **ArchUnit** (`archunit-junit5`). `mvn test` verificará automáticamente que ninguna clase de `domain` importe paquetes de Spring, JPA o infraestructura. Si la IA o un desarrollador viola los límites hexagonales, el build debe fallar de inmediato.
- **Java 21** (Records, Pattern Matching, Sealed Classes/Interfaces, etc.) sobre **Spring Framework 6.x / Spring Boot 3.5.8+**, ejecución en **Tomcat** (starter-tomcat `provided`).
- Proyecto **Mavenizado**. En la **raíz del repo** solo pueden vivir `pom.xml`,
  `readme.md` y la carpeta `/src`; el resto no se modifica.
- **Control de Versiones**: Metodología **GitFlow** / Trunk-Based Development con ramas protegidas.
- Artefactos gestionados a través de repositorio centralizado (Nexus / Artifactory).
- Despliegue en **Kubernetes / OpenShift 4**, salvo excepción justificada que requiera VM.
- Un servicio nuevo se integra en un proyecto de su **dominio funcional** o crea uno nuevo.

## Patrones Arquitectónicos
1. **Clean Architecture / Hexagonal (OBLIGATORIA)** — separa estrictamente la lógica de negocio de los detalles técnicos y es agnóstica al origen del dato, facilitando la inyección de dependencias y el testeo unitario aislado. Tres componentes:
   - **Dominio**: entidades + reglas de negocio, puro Java, independiente de frameworks (cero dependencias de Spring).
   - **Puertos**: interfaces de casos de uso (in) y dependencias (out).
   - **Adaptadores**: implementaciones concretas (REST, JPA, clientes REST, seguridad).
2. **MVC** — **Deprecado** para nuevos desarrollos. Solo permitido para servicios muy sencillos (CRUD sin lógica) previa justificación técnica explícita en el ADR.

> **Elección de arquitectura**: Elige SIEMPRE **hexagonal** a menos que se justifique lo contrario. **El empaquetado es SIEMPRE WAR sobre Tomcat externo** (`spring-boot-starter-tomcat` con `scope=provided`), independientemente del patrón.

## Configuración
- **Un único archivo de configuración** (idóneamente **YAML**), sobrescrito por los
  ConfigMaps de OpenShift/Kubernetes. **Prohibidos los Spring profiles** en la configuración.
- Secretos siempre como Secret de K8s → variable de entorno → leída en config (§2.4).

## Servicios REST
- Todos los servicios bajo estándar **RESTful JSON**; cualquier excepción se justifica antes.
- Toda conexión REST por **SSL validado** contra las CA del truststore.
- Prefijo de versión (`/v1`) en los controladores.

## Seguridad y autenticación
Todo servicio lleva, como mínimo, control de la petición en el header `Authorization`:
- **Canales de cliente**: token SSO **OAuth2 / OIDC**.
- **Canales internos**: OAuth2/OIDC, o **Basic Auth / API-Key** para otras conexiones.
- **Nunca** hardcodear claves (inyección por env, §2.4). **Enmascarado de logs**
  obligatorio para datos sensibles (mínimo `Authorization`); sin PII en claro (§2.1).

## Librerías
**Permitidas**:
spring-boot-starter-`web`/`data-jpa`/`actuator`/`security`/`oauth2-resource-server`/
`tomcat`(provided); `slf4j-api` (+Logback); testing `spring-boot-starter-test` (JUnit 5, Mockito,
AssertJ, Hamcrest), `h2`, `jacoco-maven-plugin`, `maven-checkstyle-plugin`,
`sonar-maven-plugin`.
**No permitidas**: **Spring Cloud**, **profiles** en config.

## Manejo de errores
- Respuestas de error estandarizadas **RFC 7807** (`ProblemDetail`): `422` validación,
  `400` negocio (`BusinessException`), `500` no controlado.

## Observabilidad y entregables por componente
- **Endpoints de Salud y Balanceador (Health & Info desde la raíz `/`)**:
  - **Ubicación en Raíz**: Deben responder directamente en la raíz `/` (`/health` y `/info`), sin prefijos anidados (`/actuator/...` o `/v1/...`) para total compatibilidad con balanceadores (F5, Ingress, Envoy).
  - **Endpoint `/health` (Probe para Balanceador y K8s)**: Debe devolver siempre **HTTP 200 OK** con JSON estricto y limpio para sondas de liveness y readiness:
    ```json
    {
      "status": "UP"
    }
    ```
    *Prohibido exponer detalles internos o excepciones que confundan el estado de salud en el balanceador.*
  - **Endpoint `/info` (Metadatos Generales)**: Debe devolver la versión, nombre del servicio y estado operativo:
    ```json
    {
      "name": "${spring.application.name}",
      "version": "@project.version@",
      "status": "UP",
      "framework": "SDD-Java-Services"
    }
    ```
  - **Configuración requerida en Spring Boot (`application.yaml`)**:
    ```yaml
    management:
      endpoints:
        web:
          base-path: /
          exposure:
            include: health, info
      endpoint:
        health:
          show-details: never
    ```
- **Métricas Prometheus**: `/prometheus` expuesto para recolección centralizada.
- **Swagger / OpenAPI 3.0** accesible en entornos no productivos.
- `README.md` con visión general (objetivo, funcionalidad, tecnología).
- Diseño de arquitectura actualizado tras la implementación.
- **Ficha de servicio** por cada servicio creado/modificado.

## Calidad (gate, constitución §4.1)
- **JUnit 5** + **JaCoCo**: cobertura mínima **70%** (validada por Sonar). **OBLIGATORIO: Todo código generado por IA debe incluir tests automatizados.**
- **Checkstyle** (estilo Google) + **SonarQube** con perfil estándar de calidad +
  **OWASP Top 10**; no se instala nada que incumpla el umbral.
- **Gate: 0 BLOCKER y 0 CRITICAL.**
- Validación local: `mvn clean verify`.
