### TypeScript / JavaScript
- TypeScript en modo estricto; sin `any` ni aserciones de tipo sin comprobar.
- Carpetas por funcionalidad (`src/features/<funcionalidad>/`); componentes de UI separados de los hooks o servicios de lógica.
- Vitest o Jest; prueba comportamiento y accesibilidad (`getByRole`, `getByText`), no detalles de implementación.
- `npm run lint` limpio; sin exports ni dependencias sin uso (Knip).
