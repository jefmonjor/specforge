# Especificación 0001 — Restablecer contraseña

- **Estado:** `APPROVED`

## 1. Intención de negocio
Reducir las llamadas a soporte por contraseñas olvidadas.

## 5. Criterios de aceptación

```gherkin
# language: es
Característica: Restablecer contraseña

  Antecedentes:
    Dado un usuario registrado con email "ana@example.com"

  Escenario: Solicitud de enlace
    Cuando solicita restablecer la contraseña
    Entonces recibe un enlace de un solo uso
    Y el enlace caduca en 30 minutos

  Escenario: Enlace caducado
    Dado un enlace emitido hace 31 minutos
    Cuando lo abre
    Entonces ve el error "enlace caducado"

  Esquema del escenario: Validez según canal
    Cuando solicita el enlace por <canal>
    Entonces el enlace caduca en <minutos> minutos

    Ejemplos:
      | canal | minutos |
      | email | 30      |
      | sms   | 10      |
```

## 7. Cuestiones abiertas
- [NEEDS CLARIFICATION]: <Anota aquí cualquier ambigüedad antes de aprobar.>
