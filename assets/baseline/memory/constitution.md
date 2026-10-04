# Constitución de Desarrollo Asistido por IA — SDD-Free

Versión: 3.0.0 (Open Source Engine)

Este documento establece las reglas obligatorias e inmutables para el desarrollo de software asistido por IA gobernado bajo SDD-Free.

---

## 1. Principio de Verdad y Especificación (SDD First)
1.1. **Ninguna línea de código sin especificación previa:** Todo desarrollo debe originarse a partir de una especificación formal BDD / Gherkin (`specs/`).
1.2. **Inmutabilidad y Sellado Criptográfico:** Antes de iniciar la construcción, la especificación debe estar sellada mediante un hash SHA-256 (`<!-- seal: sha256:... -->`). Cualquier modificación manual posterior invalida el estado y exige re-aprobación explícita.
1.3. **Branch Protection:** Queda prohibido trabajar directamente sobre las ramas protegidas (`master` o `main`). Todo cambio se realiza en ramas de trabajo (`feature/...`) y se integra mediante Pull Request.

---

## 2. Confidencialidad y Datos
2.1. **Cero Datos Sensibles o PII:** Queda terminantemente prohibido incluir credenciales, tokens, secretos, nombres de clientes o volcados de datos reales en especificaciones, prompts o código de prueba. Se emplearán siempre fixtures y datos sintéticos.

---

## 3. Disciplina TDD Estricta (Red -> Green -> Refactor)
3.1. **Fase RED Obligatoria:** Toda funcionalidad comienza con una prueba unitaria o de integración que **falla de manera verificable**.
3.2. **Gate Anti-YAGNI:** Si un test pasa antes de escribir el código de implementación, la ejecución se aborta inmediatamente por violación YAGNI.
3.3. **Fase GREEN Mínima:** La IA debe generar estrictamente el código necesario para satisfacer la prueba, evitando sobre-ingeniería o abstracciones prematuras.

---

## 4. Arquitectura Limpia y Principios SOLID
4.1. **Inversión de Dependencias (DIP):** El dominio de negocio no depende de frameworks, bases de datos ni clientes externos; toda interacción con infraestructura se realiza a través de puertos e interfaces.
4.2. **Responsabilidad Única (SRP):** Cada clase, paquete o módulo debe responder a una única razón de cambio.

---

## 5. Guardarraíles de Calidad Automatizados (Fase REFACTOR)
5.1. **Principio DRY (Zero Duplication):** Tolerancia cero a bloques duplicados (verificado mediante `jscpd` con umbral 0%).
5.2. **Cero Código Muerto:** Detección y eliminación de exportaciones o dependencias no utilizadas (`knip`).
5.3. **Robustez de Pruebas (Mutation Testing):** Las pruebas unitarias deben demostrar resistencia activa ante mutantes (`Stryker` con Mutation Score >= 80%).
5.4. **Linters Estrictos:** El código debe superar las auditorías estáticas del lenguaje sin advertencias.

---

## 6. Gobernanza de la Inteligencia Artificial y Resiliencia
6.1. **Human-in-the-Loop:** La IA asiste y propone, pero la responsabilidad técnica y el veredicto final corresponden al desarrollador humano.
6.2. **Resiliencia y Tolerancia a Fallos (Cláusula 6.6):** Todo ciclo de desarrollo asistido por IA debe persistir deterministamente sus puntos de control en `.sdd-state.json`. Ningún fallo de red, cuota de API o error transitorio destruirá el progreso; la ejecución debe reanudarse (`--resume`) en el punto exacto de interrupción.
