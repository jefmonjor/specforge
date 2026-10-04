# {{MODULE}}

{{DESCRIPTION}}. Esqueleto **React + Vite + TypeScript** del SDD SDDFramework.

- **Baseline SDD**: sdd-baseline-{{BASELINE}}
- **App**: `{{APP}}`

## Estructura
| Fichero | Para qué |
|---|---|
| `package.json` | scripts `dev`/`build`/`lint`/`test` (los usa el gate `sdd.ps1 build`) |
| `vite.config.ts` · `tsconfig.json` | Vite + TypeScript + Vitest (jsdom) |
| `eslint.config.js` | lint (endurecer según `standards/react.md`) |
| `src/` | `main.tsx`, `App.tsx`, `App.test.tsx` |

## Build y verificación (gate real)
```bash
npm ci && npm run lint && npm test && npm run build   # = sdd.ps1 build
```

## Flujo SDD (loop engineer)
```powershell
sdd.ps1 setup -Ai gemini -BaselineVersion {{BASELINE}}
sdd.ps1 loop -Intent "<qué construir>"      # motor OpenSpec (proyecto nuevo)
```
El loop especifica (`/opsx:propose`), implementa (`/opsx:apply`) con el build real + auto-reparación,
documenta y certifica; tú solo resuelves dudas y verificas en los GATES.
