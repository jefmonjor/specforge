# CATÁLOGO DE CLASES DE ATAQUE ADVERSARIAL (ATTACK CLASSES)

## 1. Web Protocol & Auth
* `auth_bypass`: Omisión de autenticación, comprobaciones de rol rotas, manipulación de JWT / firmas ausentes.
* `idor`: Insecure Direct Object References (acceso a identificadores numéricos/UUIDs sin validar propiedad del recurso).
* `ssrf`: Server-Side Request Forgery (peticiones a URLs proporcionadas por el usuario hacia endpoints internos o metadatos de nube).
* `csrf`: Cross-Site Request Forgery en operaciones de mutación de estado sensibles.

## 2. Inyecciones & Flujos de Datos
* `sql_injection`: Concatenación manual de strings en consultas SQL/NoSQL.
* `command_injection`: Paso de entradas no confiables a `exec`, `spawn`, `os/exec` o llamadas al shell.
* `path_traversal`: Lectura/escritura arbitraria de ficheros mediante secuencias `../` en rutas de disco.
* `xss`: Inyección de scripts en respuestas HTML/DOM sin codificación adecuada.

## 3. Supply Chain & Secrets
* `hardcoded_secrets`: Claves API, tokens JWT o contraseñas quemadas en el código fuente.
* `prototype_pollution`: Fusión insegura de objetos JSON en entornos JavaScript/TypeScript.
* `insecure_deserialization`: Carga no confiable de objetos serializados (pickle, Java serialization, YAML unsafe load).

## 4. AI & LLM Systems
* `prompt_injection`: Entradas de usuario no confiables que sobreescriben directivas del sistema en LLMs.
* `untrusted_tool_execution`: El modelo invoca herramientas del sistema o muta datos sin validación ni gates de aprobación.
