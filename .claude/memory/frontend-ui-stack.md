---
name: frontend-ui-stack
description: "Standard UI stack for tendersbay frontend apps — motion + react-aria-components, no emoji"
metadata:
  type: reference
  updated: 2026-07-11
  sources: []
---

For tendersbay React/Vite apps (e.g. `apps/platform`), use this standard UI stack:

- **`motion`** (the package, successor to Framer Motion) for all animation — import from
  `motion/react`. Use `whileInView` for scroll reveals, `AnimatePresence` for
  enter/exit, and `useReducedMotion()` to honor `prefers-reduced-motion`.
- **`react-aria-components`** for accessible interactive primitives (Select, Link, Button,
  ListBox, etc.) — style via Tailwind on render states (`data-[hovered]`,
  `data-[focus-visible]`, `data-[pressed]`).
- **`lucide-react`** for icons — wrap it in a small `Icon` atom that maps semantic names
  to lucide components (keeps consumers decoupled, centralizes sizing/stroke).
- **No emoji** in UI — use lucide-react icons (`currentColor`) only.

**Why:** the user explicitly chose these as the project's standard ("add this choice to
Claude toolkits") while building the landing page. They want accessibility-first
primitives + real animation, with a clean icon set (lucide) instead of emoji.

**How to apply:** default to these for any new UI work in tendersbay apps. Add deps with
`pnpm add <pkg> --filter <workspace>`. Pair with the design language in
[[landing-page-design]]. Since the redesign's Phase 0, reusable primitives built on this
stack (Button, Pill, Card, Field, EmptyState, PageHeader, `cn`) live in the shared kit —
see [[core-component-kit]] before hand-rolling one in an app.
