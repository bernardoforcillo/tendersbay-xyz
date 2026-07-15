---
name: locale-namespace-insertion
description: "Adding an i18n namespace across the 24 locale files: anchor-splice script + biome format + <ns>-keys completeness test"
metadata:
  type: reference
  updated: 2026-07-13
  sources: [docs/superpowers/plans/2026-07-12-redesign-surfaces.md]
---

The proven end-to-end recipe for adding a whole i18n namespace to all 24
`apps/platform/src/assets/locales/<locale>/common.json` files. Precedents: the
`shell`, `today`, and `account` namespaces (the `account` one — 30 keys — landed in
redesign Phase 4; Account settings had rendered `defaultValue` English until then).
The base idea (identical file structure across locales + a completeness test) is in
`.claude/rules/frontend.md`; this page records the working recipe.

1. **Inventory the exact keys from code first** — grep `t('<ns>.` across the
   feature; the en-ie strings come from the code's `defaultValue`s.
2. **Splice with a scratchpad script**: insert the `"<ns>": { … },` block before an
   **anchor line that exists verbatim in all 24 files** (e.g. `  "landing": {`) —
   verify the anchor in every file before running; one translated payload per locale.
3. **Biome-format the touched files** (`pnpm exec biome check --write <paths>`, not
   repo-wide).
4. **Completeness test** `src/assets/locales/<ns>-keys.test.ts` (mirror
   `today-keys.test.ts`): `import.meta.glob` the locale files, assert 24 files and
   every required key present in each — a missed or malformed file fails loudly.

Translation register: it-it authored natively; the other 22 translated with the
register each file already uses (de-de formal "Sie"; nl-nl informal "je" per its
settings precedent); product terms stay as loanwords. Flag `ga-ie`/`mt-mt` for
native spot-check.

Used heavily by kit-adoption restyles — see [[core-component-kit]].
