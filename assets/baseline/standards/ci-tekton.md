# Standard de CI/CD — Tekton (OpenShift Pipelines)

> **ESTÁNDAR OFICIAL CORPORATIVO**  
> Tekton es la plataforma oficial de CI/CD y despliegue continuo en OpenShift 4 (OCP4) para el banco, operando bajo el paradigma **GitOps**.

---

## 1. Separación de Responsabilidades: Código vs. Repositorio de Values

Siguiendo las mejores prácticas de **GitOps**:
1. **El repositorio de código de la aplicación** contiene exclusivamente el código fuente, pruebas, especificaciones SDD y lógica de negocio.
2. **El repositorio de despliegue (GitOps / Values)** almacena los manifiestos, pipelines de Tekton y ficheros de configuración por entorno (`dev`, `pre`, `pro`).
3. **Plantilla de Referencia Local (`values/dev-app-values.yaml`):**  
   Para facilitar el trabajo del desarrollador y la sincronización con el equipo de infraestructura, el framework genera localmente el directorio `values/` con `dev-app-values.yaml`.
4. **Exclusión Obligatoria en Git:**  
   La carpeta `values/` está **estrictamente excluida en `.gitignore`** para garantizar que los valores de despliegue e infraestructura nunca se mezclen ni se suban al repositorio de código fuente.

---

## 2. Estructura de `values/dev-app-values.yaml`

El fichero de referencia local define los parámetros requeridos por las Tasks de Tekton y los balanceadores de OpenShift:

```yaml
app:
  name: "${APP_NAME}"
  environment: "dev"
  replicas: 2

image:
  repository: "image-registry.openshift-image-registry.svc:5000/${NAMESPACE}/${APP_NAME}"
  tag: "latest"
  pullPolicy: "Always"

probes:
  liveness:
    path: "/health"
    port: 8080
    initialDelaySeconds: 30
    periodSeconds: 10
  readiness:
    path: "/health"
    port: 8080
    initialDelaySeconds: 15
    periodSeconds: 5

resources:
  limits:
    cpu: "1000m"
    memory: "1Gi"
  requests:
    cpu: "250m"
    memory: "512Mi"

service:
  type: "ClusterIP"
  port: 8080
  targetPort: 8080
```

---

## 3. Mínimos de Calidad en Pipelines de Tekton
* **Ejecución No-Root:** Todas las Tasks deben ejecutarse bajo `runAsNonRoot: true` y `allowPrivilegeEscalation: false` (Constitución §5.1).
* **Dependencias y Artefactos:** Dependencias resueltas exclusivamente desde Nexus corporativo.
* **Secretos Seguros:** Contraseñas y certificados gestionados mediante Secrets de OpenShift / Vault inyectados como variables de entorno, nunca en el Pipeline ni en logs.
* **Gate de Frescura SDD:** Las tareas de verificación de Pull Request ejecutan la Task de frescura (`ci/tekton-sdd-freshness.yaml`) para impedir merges si la documentación está obsoleta (§8.6).
