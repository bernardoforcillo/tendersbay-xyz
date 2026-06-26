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

## Interaction & animation gotchas

Hard-won patterns for the `motion` + `react-aria-components` stack. Reach for these
before re-deriving them:

- **Instant hover cards from RAC `TooltipTrigger`.** It has a ~1.5s *global* hover
  warmup that `delay={0}` does **not** bypass, so the first hover looks dead. Make
  the tooltip **controlled** — `isOpen={hovered || focused}` driven by the trigger
  `Button`'s `onHoverChange` / `onFocusChange`. (Focus opens immediately even
  uncontrolled; only hover is delayed.)
- **Don't put `inert` on duplicated/decorative interactive content** (e.g. a
  marquee's second track). `inert` kills pointer events, so hover/click die on
  those copies. Keep them mouse-interactive but out of the a11y tree and tab order
  with `aria-hidden` on the container + `excludeFromTabOrder` on each RAC control.
- **Infinite marquees with motion**: drive a `useMotionValue` x with
  `useAnimationFrame`, wrapping by `(trackWidth + gap)` measured via
  `ResizeObserver`; pause on hover/focus, idle off-screen with `useInView`, and
  render a static fallback under `useReducedMotion`.

## Testing motion components (vitest + jsdom)

- jsdom lacks `ResizeObserver` and `IntersectionObserver` — both are stubbed in
  `apps/platform/vitest.setup.ts`. Add new globals there, not per-test.
- For components that branch on `useReducedMotion`, force the **static** variant in
  tests by overriding `window.matchMedia` in a `beforeEach` so
  `(prefers-reduced-motion: reduce)` matches. This gives deterministic markup
  (no animation-driven duplicates) and keeps heavy full-page renders inside the
  default vitest timeout.

## i18n, theming & "coming soon" states

- **Read array copy** with `t(key, { returnObjects: true }) as string[]` (precedent:
  `hero.trust`, `search.examples`). The bundled resources resolve synchronously, so the
  array is available on first render — no loading guard needed.
- **Batch edits across the 24 locales**: every
  `src/assets/locales/<locale>/common.json` shares an identical `"landing": {` opening, so
  the same structural insert/replace applies to all of them. Pair it with a
  `search-keys.test.ts`-style completeness test (`import.meta.glob` the locales, assert the
  count is 24 and each required key is present) so a missed or malformed file fails loudly.
- **There are no neutral-gray tokens** — the `ink` scale is green-tinted and `cream` is
  warm. For a disabled / "coming soon" / inactive look, apply the Tailwind `grayscale`
  filter utility (as `country-flag` and `search-dock` do) instead of reaching for a gray
  that isn't in the theme.
- **Disabled-but-focusable control** (e.g. a pre-launch teaser): use a RAC `Button` that is
  NOT `isDisabled` (that drops it from the tab order) but carries `aria-disabled="true"`, a
  no-op `onPress`, and `cursor-default`. It stays keyboard-focusable and is announced as
  unavailable. (If it also shows a hover hint, make the tooltip controlled per the
  `TooltipTrigger` note above, or the first hover lags ~1.5s.)
- **Iterate with a focused test**: `pnpm --filter platform exec vitest run <path>` instead
  of the whole suite.

## Vite config consuming a no-build TS workspace package

A private package that exports raw TS (`exports: { ".": "./src/index.ts" }`, no build step
— e.g. `@tendersbay/vite-plugin-seo`) imports fine from **app source** (Vite/esbuild bundles
it). But imported from **`vite.config.ts`** it breaks: Vite externalizes node_modules deps
while loading the config, then Node loads the raw `.ts` via native type-stripping, which
**cannot resolve extensionless relative imports** (fails with `Cannot find module '.../head'`).

Fix, in the package consumed at config-load time:

- give every internal relative import an explicit `.ts` extension —
  `import { headTags } from './head.ts'`, including `export type … from './options.ts'`;
- set `"allowImportingTsExtensions": true` in its `tsconfig.json` (valid because the shared
  base config sets `noEmit`).

Only the import closure reachable from the package entry at config-load needs this; packages
consumed solely by app source (like `@tendersbay/components`) do not — they are bundled, not
externalized.
