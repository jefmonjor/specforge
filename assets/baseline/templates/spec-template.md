# Especificación NNNN — <Título conciso de la funcionalidad>

> **Craftsmanship Note**: Esta especificación es la única Fuente de Verdad. Describe el **QUÉ** y el **POR QUÉ** utilizando el *Lenguaje Ubicuo* del dominio de negocio. Cualquier decisión tecnológica (el *CÓMO*) está estrictamente prohibida aquí y debe delegarse al Plan Técnico.

- **Estado:** `[ DRAFT | REVIEW | APPROVED ]`
- **Autor:** <Nombre / Rol>
- **Fecha:** <YYYY-MM-DD>
- **Referencia Jira:** <TICKET-XXXX>

---

## 1. Intención de Negocio (El Por Qué)
*El problema que estamos resolviendo o el valor exacto que aportamos al usuario. Máximo 3 líneas. Sé directo.*
> **Ejemplo:** "Necesitamos reducir la tasa de abandono en el onboarding permitiendo la validación asíncrona del KYC, evitando que el usuario espere en la pantalla de carga."

## 2. Lenguaje Ubicuo (Glosario del Dominio)
*Define los términos de negocio que se usarán en el código (Domain-Driven Design).*
- **`Término 1`**: Definición estricta para que Negocio y Desarrollo hablen el mismo idioma.
- **`Término 2`**: ...

## 3. Invariantes del Dominio (Reglas Inquebrantables)
*Leyes fundamentales del negocio que el sistema NUNCA puede permitir violar, bajo ningún estado.*
- **INV-01**: <Ejemplo: El saldo de una cuenta corriente nunca puede ser negativo si no dispone de línea de crédito aprobada.>
- **INV-02**: <Ejemplo: Ninguna transferencia puede ejecutarse si el estado de la cuenta origen es BLOQUEADA.>

## 4. Historias de Usuario (User Stories)
*Formato: Como [Rol], quiero [Acción] para [Beneficio].*
- **US1:** Como <rol>, quiero <acción> para <beneficio>.

## 5. Criterios de Aceptación (Comportamiento BDD)
*Las reglas ejecutables. Cada escenario servirá de base directa para el Test-Driven Development (Red-Green-Refactor).*

```gherkin
Feature: <Nombre de la US>

  Scenario: <Happy Path - El camino feliz>
    Given <estado inicial válido>
    And <precondición extra>
    When <acción del usuario o evento del sistema>
    Then <resultado observable>
    And <efecto secundario esperado>

  Scenario: <Sad Path - Caso de fallo manejado>
    Given <estado inicial problemático>
    When <acción o evento>
    Then <mensaje de error o comportamiento degradado elegante>
```

## 6. Fronteras y Fuera de Alcance (KISS / YAGNI)
*Especifica qué cosas **NO** vamos a construir ahora para evitar la sobre-ingeniería (YAGNI).*
- [ ] NO se contempla...
- [ ] Queda fuera de esta spec...

## 7. Cuestiones Abiertas
- [NEEDS CLARIFICATION]: <Anota aquí cualquier ambigüedad antes de aprobar. La IA no avanzará hasta que esto esté vacío.>

---
**🛡️ GATE 1 (Aprobación de Negocio):** El Agente de IA se detendrá aquí. Requiere validación explícita del Product Owner o responsable funcional antes de iniciar el diseño arquitectónico. No usar *fire-and-forget*.
