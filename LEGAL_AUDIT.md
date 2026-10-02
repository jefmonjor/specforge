# Auditoría Legal y Matriz de Licencias: SDD-Free

> **Proyecto:** SDD-Free (Spec-Driven Development CLI)  
> **Licencia Seleccionada:** **Apache License 2.0**  
> **Estado de Auditoría:** APROBADO PARA OPEN SOURCE (Cero Riesgo Legal)

---

## 1. Elección de la Licencia: ¿Por qué Apache 2.0 frente a MIT?

Para una herramienta de línea de comandos orientada a desarrolladores e integración en empresas, **Apache 2.0 es superior a MIT** por tres razones estratégicas:

1. **Concesión Explícita de Patentes (Patent Grant - Cláusula 3):**  
   A diferencia de la licencia MIT (que no menciona patentes), Apache 2.0 estipula que cualquier desarrollador o empresa que contribuya código otorga automáticamente una licencia perpetua y libre de regalías sobre sus patentes asociadas a esa contribución. Esto blinda a los usuarios contra demandas por infracción de patentes.
2. **Protección de Marca Registrada (Trademark Protection - Cláusula 6):**  
   Prohíbe a terceros usar el nombre del proyecto o su logotipo para publicitar sus propios productos comerciales sin autorización previa, protegiendo tu identidad de marca.
3. **Compatibilidad Empresarial y Comercial:**  
   Es una licencia permisiva. Cualquier persona o corporación puede descargar `sdd-free`, integrarlo en sus proyectos internos, crear derivados e incluso vender servicios sobre él sin pagar royalties. Al mismo tiempo, te permite ofrecer versiones comerciales cerradas ("Open-Core") en el futuro sin incompatibilidades.

---

## 2. Auditoría de Dependencias y Librerías

Todas las dependencias embebidas y enlazadas en el código han sido auditadas:

### A. Dependencias de Go (Compiladas en el Binario)
| Librería | Propósito | Licencia | Compatibilidad con Apache 2.0 |
| :--- | :--- | :---: | :---: |
| `github.com/spf13/cobra` | Framework de CLI y comandos | **Apache 2.0** | 100% Nativa |
| `github.com/spf13/pflag` | Parser de banderas POSIX | **BSD 3-Clause** | Totalmente Permisiva |
| `github.com/inconshreveable/mousetrap` | Detección de ejecución Windows | **Apache 2.0** | 100% Nativa |
| `golang.org/x/oauth2` | Autenticación OAuth2 / Google ADC | **BSD 3-Clause** | Totalmente Permisiva |
| `cloud.google.com/go/compute/metadata` | Detección de metadatos de GCP | **Apache 2.0** | 100% Nativa |

**Conclusión:** 100% de las librerías en Go utilizan licencias permisivas estándar sin cláusulas *copyleft* restrictivas (como GPL o AGPL).

---

### B. Herramientas Externas y de Calidad (Invocadas como subprocesos)
| Herramienta | Función | Licencia | Estado en `sdd-free` |
| :--- | :--- | :---: | :--- |
| **Knip** | Detección de código muerto | **MIT** | Compatible (invocación vía `npx knip`) |
| **jscpd** | Detección de duplicación DRY | **MIT** | Compatible (invocación vía `npx jscpd`) |
| **Stryker Mutator** | Mutation Testing | **Apache 2.0** | Compatible (invocación vía `npx stryker`) |
| **ESLint** | Linter TypeScript / JS | **MIT** | Compatible |
| **golangci-lint** | Linter de Go | **GPL v3 (CLI externo)** | **Seguro:** Se ejecuta como proceso externo independiente (`os/exec`). No contamina la licencia del binario Go al no enlazar bibliotecas. |
| **Ruff** | Linter Python | **MIT** | Compatible |
| **ArchUnit** | Validación arquitectónica Java | **Apache 2.0** | Compatible |
| **SonarQube Scanner** | Calidad estática | **LGPL v3 (CLI)** | **Seguro:** Invocación externa desacoplada. |

---

### C. Herramientas Excluidas de `sdd-free` (Por Riesgo Legal)
1. **`tgrep.exe` (Excluido):** Se elimina el binario propietario de `tgrep.exe`. El motor de escaneo de `sdd-free` utiliza el recolector nativo en Go (`filepath.WalkDir`), mucho más rápido, auditable y sin binarios opacos.
2. **Conectores Mainframe / MDOpen / MDCMS (Excluidos):** MDCMS es una suite comercial de software para IBM i. Se elimina cualquier referencia propietaria o script de empaquetado bancario.
3. **Configuraciones de Nexus Privadas (Excluidas):** Se eliminan URLs y credenciales apuntando a servidores internos.

---

## 3. Protocolo de Sanitización de Propiedad Intelectual

Para garantizar que `sdd-free` pueda publicarse en GitHub sin riesgo de reclamación corporativa:

1. **Cero Nombres o Dominios Corporativos:**  
   Se sustituyen referencias específicas por configuraciones neutras:
   * Proyecto GCP por defecto: `your-gcp-project-id` (en vez de proyectos internos).
   * Servidor SonarQube: `https://sonar.example.com` o configurable vía flag/entorno.
   * Modos de almacenamiento: local en `~/.sdd-free/`.
2. **Autoría e Identidad en Git:**  
   Al crear el repositorio público en tu cuenta personal de GitHub:
   * Configura tu correo personal:
     ```bash
     git config user.name "Tu Nombre"
     git config user.email "tu-correo-personal@gmail.com"
     ```
   * Realiza un commit inicial limpio (`Initial commit - SDD-Free v3.0.0`) sin arrastrar el historial de commits corporativos.
