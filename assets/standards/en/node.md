### TypeScript / JavaScript (React)
- TypeScript in strict mode; no `any`, no unchecked type assertions, no non-null `!` to silence the compiler.
- Feature folders (`src/features/<feature>/`); components render, hooks and plain modules hold the logic, so rules are tested without a DOM.
- Function components and hooks; state as close to where it is used as possible; derived values computed, not stored.
- Vitest + Testing Library: test what the user sees and does (`getByRole`, `getByLabelText`, `userEvent`), never implementation details or snapshots of markup.
- Accessible by default: labelled controls, semantic elements, buttons for actions and links for navigation.
- `npm run lint` clean (ESLint + `tsc`); no unused files, exports or dependencies (Knip); no duplication (jscpd); mutation score above the threshold (Stryker).
