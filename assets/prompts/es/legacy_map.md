# Tarea: MAPA LEGACY — encontrar las capacidades de negocio de un sistema legacy

Se está reescribiendo un sistema legacy. Su código está en `{{.Legacy}}`: **léelo, no lo cambies nunca** (SpecForge comprueba que ningún fichero de ahí cambió). Abajo tienes su inventario, medido por SpecForge.

## Inventario
{{.Inventory}}

## Qué hacer
Escribe `{{.DocPath}}` y no cambies ningún otro fichero. Enumera lo que el sistema hace para el negocio, una capacidad por sección, para que cada una pueda convertirse en una especificación:

```markdown
# Mapa de capacidades

## <nombre de la capacidad, en palabras de negocio>
- **Qué hace**: una o dos frases, en el lenguaje del negocio.
- **Puntos de entrada**: `ruta/relativa/Servlet.java:12-40`, `ruta/relativa/pagina.jsp`
- **Reglas encontradas en el código**: cada regla con la fuente que la implementa, `ruta:línea`.
- **Datos**: las tablas, ficheros o mensajes que lee y escribe.
- **Depende de**: las otras capacidades que necesita.
- **Sin aclarar**: lo que el código hace y no sabes explicar (ramas muertas, números mágicos, código comentado). No inventes nunca un motivo.
```

Reglas:
- Cada fuente se cita entre comillas invertidas como `ruta:línea` o `ruta:desde-hasta`, relativa a `{{.Legacy}}`. SpecForge abre cada cita; una que no existe, o una línea más allá del final del fichero, devuelve el mapa.
- Describe comportamiento, no tecnología: «el neto de un empleado es el bruto menos la retención» y no «PayrollServlet llama a EmployeeDao».
- Ordena las capacidades de modo que las que otras necesitan vayan primero: es el orden en que migrarlas.
- No propongas aquí el diseño nuevo; eso lo hace el plan de cada especificación.
{{template "turn" .}}
{{template "context" .}}
{{template "contract" .}}
