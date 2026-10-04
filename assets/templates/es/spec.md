---
id: ""
title: ""
status: draft
created: "{{.Date}}"
approved_by: ""
approved_at: ""
---

# {{.ID}} · {{.Title}}

<!--
Esta especificación dice QUÉ debe hacer el software y POR QUÉ, con las palabras
del negocio. El CÓMO va en el plan. Sustituye cada TODO: `specforge spec approve`
se niega mientras quede uno, y también con cualquier cuestión abierta en la sección 12.
-->

## 1. Intención

TODO: el problema que resuelve y el valor que aporta, en dos o tres frases.

## 2. Actores

- **TODO rol**: qué puede y qué no puede hacer.

## 3. Lenguaje ubicuo

- **TODO término**: su significado exacto. Todo término usado en los escenarios se define aquí.

## 4. Invariantes

- **INV-01**: TODO una regla que el sistema no puede romper nunca, en ningún estado.

## 5. Historias de usuario

- **US-1**: Como TODO, quiero TODO para TODO.

## 6. Escenarios

<!-- Un comportamiento por escenario: un Cuando, al menos un Entonces, título
     único, sin palabras de interfaz ni de tecnología. Cada invariante tiene un
     escenario de fallo. -->

```gherkin
# language: es
Característica: {{.Title}}

  Escenario: TODO el camino principal
    Dado TODO
    Cuando TODO
    Entonces TODO

  Escenario: TODO un fallo controlado
    Dado TODO
    Cuando TODO
    Entonces TODO
```

## 7. Contratos de datos

| Campo | Tipo | Obligatorio | Regla |
| :--- | :--- | :---: | :--- |
| TODO | | | |

## 8. Errores

| Código | Cuándo | Mensaje |
| :--- | :--- | :--- |
| TODO | | |

## 9. Requisitos no funcionales

- TODO: un número (latencia, volumen, retención…) o "ninguno".

## 10. Fuera de alcance

- TODO: lo que esta especificación deliberadamente no cubre.

## 11. Supuestos

- TODO: lo que se da por hecho y quién lo confirmó.

## 12. Cuestiones abiertas

<!-- Una por línea, como "- [NEEDS CLARIFICATION]: la pregunta". Debe quedar
     vacía antes de aprobar: el agente nunca construye sobre una suposición. -->
