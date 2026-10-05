# Tarea: PLAN — decidir dónde va el código, antes de escribir ninguno

Trabajas sobre la especificación aprobada **{{.SpecTitle}}** (`{{.SpecPath}}`) en un proyecto {{.Stack}}.

## Especificación
{{.Spec}}

## Ficheros del proyecto
```
{{.Tree}}
```

## Qué hacer
Escribe el plan técnico en `{{.PlanPath}}` y no cambies ningún otro fichero. Usa estas secciones:

1. `## Enfoque`: como mucho cinco frases sobre cómo se implementarán los escenarios, en el estilo que ya tiene el proyecto.
2. `## Componentes`: una línea por fichero a crear o cambiar, con la ruta entre comillas invertidas, como esta línea: - `internal/pay/neto.go`: calcula el salario neto (nuevo). SpecForge solo deja al agente editar estos ficheros, los ficheros de test planificados y ficheros nuevos bajo el directorio de un componente nuevo. Cuando un componente sirve solo a algunos escenarios, termina su línea con sus marcadores (` · SDD_0001_002`): con `loop.parallel`, los escenarios cuyos ficheros no se solapan se ejecutan en paralelo.
3. `## Tests por escenario`: una tabla con una fila por escenario: marcador, título del escenario, fichero de test (entre comillas invertidas), nombre del test. Deben aparecer todos los marcadores: {{range $i, $m := .Markers}}{{if $i}}, {{end}}`{{$m}}`{{end}}.
4. `## Interfaces y dependencias`: las interfaces (puertos) que necesita el dominio y qué código o librería existente las implementa. Prefiere lo que el proyecto ya usa; no añadas dependencias que la especificación no exija.
5. `## Riesgos`: lo que podría hacer un escenario más difícil de lo que parece.

Sé breve: una persona revisa el plan antes de que exista código. No escribas código ni tests. Si una decisión de arquitectura no está resuelta por la especificación, el código existente o el registro de decisiones, pregunta en lugar de elegir.
{{- if .Draft}}
## Borrador actual (revísalo; conserva lo que siga siendo correcto)
````markdown
{{.Draft}}
````
{{end}}
{{template "turn" .}}
{{template "context" .}}
{{template "contract" .}}
