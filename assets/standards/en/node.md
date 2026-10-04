### TypeScript / JavaScript
- TypeScript in strict mode; no `any`, no unchecked type assertions.
- Feature folders (`src/features/<feature>/`); UI components separate from logic hooks or services.
- Vitest or Jest; test behaviour and accessibility (`getByRole`, `getByText`), not implementation details.
- `npm run lint` clean; no unused exports or dependencies (Knip).
