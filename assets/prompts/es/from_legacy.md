# Tarea: ESPECIFICACIÓN DESDE LEGACY — dejar escrito lo que el sistema antiguo hace de verdad

Se está reescribiendo un sistema legacy{{if .JavaRelease}} en Java {{.JavaRelease}}{{end}}. Su código está en `{{.Legacy}}`: **léelo, no lo cambies nunca** (SpecForge comprueba que ningún fichero de ahí cambió). Completa la especificación `{{.SpecPath}}` para esta capacidad:

> **{{.Capability}}**

La especificación recoge el comportamiento **tal como es hoy**, para poder probar el código nuevo contra él. No es una lista de deseos.

## Qué hacer
1. Encuentra el código que implementa la capacidad (el mapa de capacidades y el inventario de abajo ayudan) y léelo, incluidos el SQL, las páginas JSP y la configuración de los que depende.
2. Sustituye cada `TODO` de `{{.SpecPath}}`, en palabras de negocio y en su idioma, manteniendo las secciones numeradas y el front matter YAML:
   - cada regla que el código impone se convierte en una invariante `INV-NN` con un escenario de fallo que la menciona;
   - cada comportamiento se convierte en un escenario Gherkin en la sección 6: un `Cuando`, al menos un `Entonces`, los valores reales que usa el código (límites, redondeos, mensajes), sin palabras de tecnología;
   - la sección 8 enumera los errores con los mensajes que el código legacy muestra de verdad.
3. Añade una última sección, `## 13. Fuentes legacy`, con una línea por regla o escenario y el código del que sale: `- INV-01: \`ruta/A.java:40-52\``. Las rutas son relativas a `{{.Legacy}}`. **SpecForge abre cada cita**: un fichero que no existe o una línea más allá de su final devuelve la especificación.
4. Lo que el código hace y no sabes explicar, o que parece un error (una regla aplicada en un sitio y no en otro, un número mágico, código muerto), va a la sección 12 como `- [NEEDS CLARIFICATION]: <pregunta>` con su fuente. **No lo decidas tú nunca**: el desarrollador lo responde antes de aprobar, y puede decidir mantener o corregir el comportamiento.
5. Deja fuera lo que es de la plataforma y no del negocio (fontanería de servlets, pools de conexiones, logging): de eso se encarga el plan de la reescritura.

No cambies ningún fichero aparte de `{{.SpecPath}}`.

## Inventario del código legacy
{{.Inventory}}
{{- if .Capabilities}}

## Mapa de capacidades
{{.Capabilities}}
{{- end}}
{{- if .Draft}}

## La especificación tal como está ahora
````markdown
{{.Draft}}
````
{{- end}}
{{template "turn" .}}
{{template "context" .}}
{{template "contract" .}}
