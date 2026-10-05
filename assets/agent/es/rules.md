## Trabajar bajo SpecForge

Este repositorio se construye a partir de especificaciones en `specs/` con un ciclo Red → Green → Refactor que SpecForge verifica por sí mismo: ejecuta los tests, guarda la huella de los ficheros de test y comprueba cada fichero que digas haber escrito.

### Pregunta, no inventes
Si algo que necesitas no está en la especificación, en su registro de decisiones (`specs/<spec>/decisions.md`), en el código existente o en el prompt, **no lo supongas**. Responde con `status: needs_clarification` y la pregunta exacta. Una suposición errónea cuesta más que una pregunta.

### Los tests son el contrato
- Nombra cada test con el marcador de escenario que te da SpecForge (por ejemplo `SDD_0001_003`).
- En RED, el test debe compilar y fallar en una aserción. Añade solo los stubs que necesite para compilar.
- En GREEN y REFACTOR, nunca crees, edites, renombres ni borres un fichero de test.
- Nunca incluyas un fichero que no hayas escrito.

### Tu shell
- Una guardia lee cada comando que ejecutas, y los scripts, ficheros y scripts de paquete que este ejecuta. Escribe un script en un paso y ejecútalo en otro, para que pueda leerse; nunca ejecutes en el mismo comando un fichero que descargas, decodificas o copias.
- Nunca borres directorios de forma recursiva, descartes trabajo sin commit ni reescribas el historial de git. Si una tarea parece necesitarlo, pregunta.

### Oficio
- Escribe el código mínimo que hace pasar el test actual (YAGNI, KISS); refactoriza solo con los tests en verde.
- Mantén el dominio libre de frameworks y de E/S; depende de interfaces pequeñas definidas donde se usan.
- Una razón de cambio por tipo y por función; nada de objetos Dios.
- Gestiona cada error de forma explícita; nunca te tragues uno.
- Usa el lenguaje ubicuo de la especificación para los nombres; sin abreviaturas que nadie pidió.
- Sin duplicación, sin código muerto y sin secretos ni datos personales reales en código, tests o fixtures.
