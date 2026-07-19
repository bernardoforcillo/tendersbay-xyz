---
name: spacing-and-visual-rhythm
description: How to decide spacing/proximity between elements inside a component (relationship strength -> proximity, margin lives on the larger element, never eyeball) and the optical-correction gotcha for padding around text
metadata:
  type: reference
  updated: 2026-07-18
---

Applies when hand-tuning spacing inside a kit component (`Card`, `PageHeader`, any
label/value pairing) rather than reaching for a default. Pairs with
[[core-component-kit]].

- **Every pair of elements has a relationship strength, and proximity should reflect
  it.** A heading and its direct subheading are strongly related — small gap. A
  heading and an unrelated sidebar item are weakly related — larger gap, or no
  implied relationship at all. When three text elements are visually stacked
  (title/subtitle/paragraph), decide which two are more tightly coupled first — the
  weaker pairing always gets the bigger gap, never a value picked by eye.
- **The margin between two text elements is a property of the larger one.** When
  spacing a heading against the paragraph beneath it, the space belongs to the
  heading (the bigger, heavier element), not the paragraph — this is what keeps the
  same visual rhythm consistent as text sizes change around it.
- **Never eyeball a spacing value — pick from a fixed scale.** Tailwind's spacing
  scale (`p-4`, `gap-2`, …) already is that fixed scale here; the discipline is
  picking *which* step the relationship strength calls for, not inventing an
  arbitrary pixel value. This repo's existing shadow scale
  (`--shadow-soft`/`-md`/`-lg` in `packages/tailwind/theme.css`) is the same idea
  applied to elevation instead of spacing.
- **Optical correction: mathematically symmetric padding around text does not look
  symmetric.** A font's bounding-box height and the actual rendered pixel height of
  its glyphs differ, so equal top/left padding around a text block reads as too much
  space on top — the text doesn't look like it's "hugging" the corner the way equal
  numbers suggest it should. Fixing this means offsetting the top (or bottom) inset
  by roughly the difference between the font's line-box height and its visible glyph
  height, not just using the same padding value on every side. Check this by eye
  against a corner-anchored text block, not by trusting equal padding numbers alone.
