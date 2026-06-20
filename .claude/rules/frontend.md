# Frontend app conventions

Applies to the React/Vite apps under `apps/` (e.g. `apps/platform`).

## `~` path alias → app `src/`

Import from an app's own `src/` with the `~` alias instead of long relative paths:

```ts
import { App } from '~/app';            // -> src/app
import { TenderCard } from '~/components/tender-card';
import '~/index.css';                   // bare `~` resolves to src itself
```

- `~/*` maps to `./src/*`; bare `~` maps to `./src`.
- The alias is **app-scoped**. Configure it per app in two places, kept in sync:
  - `tsconfig.json` → `compilerOptions.paths` (`"~": ["./src"]`, `"~/*": ["./src/*"]`).
  - `vite.config.ts` → `resolve.alias` (`'~': fileURLToPath(new URL('./src', import.meta.url))`).
- Cross-package imports still use the package name (`@tendersbay/components`), never `~`.

## `/<name>/index.ts(x)` module structure

Each module is a folder with an `index.ts`/`index.tsx` entry point, so the import is
the folder rather than a file:

```
src/
  app/
    index.tsx          // import { App } from '~/app'
  components/
    tender-card/
      index.tsx        // import { TenderCard } from '~/components/tender-card'
      use-card.ts      // helpers, styles, and tests co-located in the folder
```

Co-locate a module's helpers, hooks, styles, and tests inside its own folder.

## Lowercase kebab-case names

Name files and folders in lowercase kebab-case — `tender-card/`, `use-auth.ts`,
`index.tsx` — even when the exported symbol is PascalCase. For example
`components/tender-card/index.tsx` exports `TenderCard`.
