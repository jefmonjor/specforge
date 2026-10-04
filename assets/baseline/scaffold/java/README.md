# {{MODULE}}

Servicio Java (Spring Boot 3.5 / Java 21) del banco. **Empaquetado WAR** desplegado en un
**Tomcat externo** (el `spring-boot-starter-tomcat` va con `scope=provided`); NO usa el Tomcat
embebido. Se despliega en OCP4 con `app.yaml` (WAR montado en `/usr/local/tomcat/...`).

- **Arquitectura**: {{ARCH}}  <!-- hexagonal (puertos/adaptadores) | mvc (capas controller/service/repository) -->
- **Baseline SDD**: sdd-baseline-{{BASELINE}}
- **Artifact**: `com.SDDFramework:{{ARTIFACT}}`

## Estructura
| Fichero | Para qué |
|---|---|
| `pom.xml` | WAR + Tomcat `provided` + Checkstyle (google_checks) + JaCoCo (≥70%) + Sonar |
| `.mvn/maven.config` + `.mvn/settings.xml` | fuerza el Nexus corporativo (`-s.mvn/settings.xml`) |
| `app.yaml` | descriptor OCP4 (Deployment + Service, puertos 8080/8443) |

## Build y verificación (gate real)
```bash
mvn clean verify        # compila + JUnit + JaCoCo (≥70%) + Checkstyle
# o vía SDD:
sdd.ps1 build           # mismo gate, stack-aware
```

## Flujo SDD (loop engineer)
```powershell
sdd.ps1 setup -Ai gemini -BaselineVersion {{BASELINE}}
sdd.ps1 loop -Intent "<qué construir>"      # greenfield
sdd.ps1 loop -Feature "<qué cambia>"        # código nuevo sobre lo existente
```
El loop especifica, implementa, corre el **build real**, documenta y certifica; el dev solo
resuelve dudas y verifica en cada GATE. Estado en `docs/STATUS.md`, decisiones en `docs/MEMORY.md`.

## Arquitectura: hexagonal vs MVC
- **MVC** (por defecto, menor complejidad): capas `controller` → `service` → `repository`.
  Adecuado para CRUD / orquestación ligera.
- **Hexagonal** (mayor complejidad): `domain` (núcleo) + `application` (casos de uso) +
  `ports` + `adapters` (in/out). Adecuado cuando hay lógica de dominio rica o múltiples
  integraciones. Elegir según la complejidad real del servicio (lo pregunta `sdd.ps1 loop`).
