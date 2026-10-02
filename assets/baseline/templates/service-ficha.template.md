# Ficha de servicio — {Dominio funcional} / {Nombre del servicio}

> Plantilla corporativa (Whitebook Java Services: "documentación a entregar con cada
> servicio"). Una ficha por servicio; se actualiza ante cualquier evolutivo.

## Control de versiones
| Fecha | Versión | Modificado por | Aprobado por | Modificación |
|---|---|---|---|---|
| {YYYY-MM-DD} | {x.y} | {autor} | {aprobador} | {cambio} |

## Nombre y descripción
- **Dominio funcional:** {dominio}
- **Nombre del servicio:** {nombre}
- **Nombre técnico:** {nombre técnico}
- **Verbo:** {GET/POST/PUT/...}
- **Ruta (DEV):** {URL completa entorno DEV}
- **Descripción:** {qué hace y lógica aplicada}

## Seguridad
- {Mecanismo: Keycloak SAML/OAuth2 | EntraID | Basic/API-Key} (ver `standards/java.md`)

## Definición de campos
**Entrada:**
| Ubicación | Campo | Tipo | Req. | Valores/Comentarios |
|---|---|---|---|---|
| {header/body/query/path} | {campo} | {tipo} | {S/—} | {comentarios} |

**Salida:**
| Ubicación | Campo | Tipo | Req. | Valores/Comentarios |
|---|---|---|---|---|
| {body} | {campo} | {tipo} | {S/—} | {comentarios} |

## Códigos de error
| errorCode | errorDescription | Comentarios |
|---|---|---|
| {código} | {descripción} | {cuándo} |

## Configuraciones y parametrizaciones
- {configuración relevante del servicio}

## Ejemplos de uso
- **Llamada (CURL):** `{curl válida}`
- **Respuesta correcta:** `{ejemplo}`
- **Respuesta errónea:** `{ejemplo}`

## Dependientes, precedentes y flujos
- **Dependientes** (consumen este servicio): {lista}
- **Precedentes** (consumidos por este servicio: servicios/programas/BBDD): {lista}
- **Flujos** en los que participa: {lista}

## Código fuente
- **Tecnología:** {JAVA/COBOL/...}
- **Repositorio:** {URL}
- **Clases/programas/objetos:** {lista}

## Infraestructura y entornos
| Entorno | Plataforma | URLs/recursos |
|---|---|---|
| DEV | {plataforma} | {urls} |
| INT | {plataforma} | {urls} |
| UAT | {plataforma} | {urls} |
| PRE | {plataforma} | {urls} |
| PROD | {plataforma} | {urls} |
