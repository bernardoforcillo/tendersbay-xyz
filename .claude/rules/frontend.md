# Frontend app conventions

Applies to the React/Vite apps under `apps/` (e.g. `apps/platform`).

## `~` path alias → app `src/`

Import from an app's own `src/` with the `~` alias instead of long relative paths:

```ts
import { App } from '~/app';            // -> src/app
import { TenderCard } from '~/features/tenders';  // feature barrel -> src/features/tenders
import '~/index.css';                   // bare `~` resolves to src itself
```

- `~/*` maps to `./src/*`; bare `~` maps to `./src`.
- The alias is **app-scoped**. Configure it per app in two places, kept in sync:
  - `tsconfig.json` → `compilerOptions.paths` (`"~": ["./src"]`, `"~/*": ["./src/*"]`).
  - `vite.config.ts` → `resolve.alias` (`'~': fileURLToPath(new URL('./src', import.meta.url))`).
- Cross-package imports still use the package name (`@tendersbay/components`), never `~`.

## `/<name>/index.ts(x)` module structure

Every module — a screen, a feature, a component, a group of hooks — is a **folder** with
an `index.ts`/`index.tsx` entry point, so the import targets the folder, not a file:

```
src/
  app/
    index.tsx          // import { App } from '~/app'
  tenders/
    index.ts           // import { TenderCard } from '~/tenders'
    use-tenders.ts     // helpers, hooks, styles, and tests co-located in the folder
```

Co-locate a module's helpers, hooks, styles, and tests inside its own folder. Components
additionally follow the atomic-design layout below.

## Feature-based + atomic-design components

Components are organized **by feature first, then by atomic-design tier**. Each module
is a folder with an `index.ts(x)` entry point, so the import is the folder, not a file.
In **apps** the feature root is `src/features/<feature>/`; in the **shared library** it is
`src/<feature>/` (no `features/` wrapper):

```
src/
  features/                        // apps only; the shared library omits this level
    <feature>/
      index.ts                     // feature barrel: re-exports each tier
      components/
        atoms/
          <name>/index.tsx           // a component (folder = module)
          index.ts                   // tier barrel: re-exports its components
        molecules/
        organisms/
        templates/
        pages/                       // apps only — see below
```

- **Tiers:** `atoms`, `molecules`, `organisms`, `templates`. `pages` is **app-only**
  (pages are routes, not reusable components) and is **never** added to the shared
  `@tendersbay/components` library.
- **Barrels** are maintained automatically by the generator: a tier barrel re-exports
  its components; a feature barrel re-exports its tiers (once each).
- Co-locate a module's helpers, hooks, styles, and tests inside its own folder.

## Shared library vs app components

- **`@tendersbay/components`** holds cross-app components under `src/<feature>/…`. Import
  them per-feature: `import { TenderCard } from '@tendersbay/components/tenders'`.
- **An app** holds its own components under `src/features/<feature>/…`, imported via the
  `~` alias (`import { TenderCard } from '~/features/tenders'`). Infra that is not a
  feature (TanStack routes in `src/routes/`, i18n in `src/i18n/`, translation files in
  `src/assets/locales/<locale>/common.json`) stays outside `features/`. Cross-package
  imports always use the package name, never `~`.

## Generating components — `pnpm gen`

Scaffold a component with the Turborepo generator instead of hand-creating folders:

```sh
pnpm gen   # prompts: target (shared lib | app) → feature → tier → name
```

It creates `<base>/features/<feature>/components/<tier>/<name>/index.tsx` for apps (or
`<base>/<feature>/components/<tier>/<name>/index.tsx` for the shared library) — an
`FC<Props>` stub — and updates the tier and feature barrels. It refuses `pages` for the
shared library.

## Lowercase kebab-case names

Name files and folders in lowercase kebab-case — `tender-card/`, `use-auth.ts`,
`index.tsx` — even when the exported symbol is PascalCase. For example
`tenders/components/molecules/tender-card/index.tsx` exports `TenderCard`.
