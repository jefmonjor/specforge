# React & TypeScript Standard — SDD-Free

## 1. Arquitectura de Componentes
* React 18+ con TypeScript estricto (`noImplicitAny: true`).
* Arquitectura basada en características (`src/features/<feature>/`).
* Separación estricta entre componentes de presentación y hooks de lógica (`use<Feature>`).

## 2. Guardarraíles de Calidad
* **Linter:** ESLint con reglas TypeScript recomendadas (`npm run lint`).
* **Principio DRY:** `jscpd src --threshold 0` (cero duplicación tolerada).
* **Código Muerto:** `knip` (cero componentes huérfanos o dependencias sin usar).
* **Robustez de Tests:** `stryker run` (Mutation testing con umbral >= 80%).

## 3. Pruebas Unitarias
* Vitest + React Testing Library.
* Enfocarse en comportamiento y accesibilidad (`getByRole`, `getByText`), no en detalles de implementación interna.
