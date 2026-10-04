### TypeScript / JavaScript (React)
- TypeScript en modo estricto; sin `any`, sin aserciones de tipo sin comprobar, sin `!` para callar al compilador.
- Carpetas por funcionalidad (`src/features/<funcionalidad>/`); los componentes pintan, los hooks y módulos planos tienen la lógica, para probar las reglas sin DOM.
- Componentes de función y hooks; el estado lo más cerca posible de donde se usa; los valores derivados se calculan, no se guardan.
- Vitest + Testing Library: prueba lo que el usuario ve y hace (`getByRole`, `getByLabelText`, `userEvent`), nunca detalles de implementación ni snapshots de marcado.
- Accesible por defecto: controles etiquetados, elementos semánticos, botones para acciones y enlaces para navegar.
- `npm run lint` limpio (ESLint + `tsc`); sin ficheros, exports ni dependencias sin usar (Knip); sin duplicación (jscpd); puntuación de mutación por encima del umbral (Stryker).
