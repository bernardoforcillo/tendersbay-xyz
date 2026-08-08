import { create } from 'zustand';

/**
 * Cross-component UI state for the scheda gara.
 *
 * `repairOpen` is promoted out of a component only because its *trigger* lives
 * outside the component that renders the result — the promotion rule in
 * @.claude/rules/frontend.md, not "it might be needed elsewhere": a gap row's
 * "why?" button sits outside the collapsed chat panel it opens.
 *
 * WHY there is nothing here for the open citation, which an earlier shape of
 * this slice carried: a quote's expansion has exactly one reader — the
 * disclosure that owns it — so it stays `useState` inside `EvidenceQuote`. A
 * store field nobody reads is not "ready for a future reader", it is state whose
 * correctness nothing checks; the same YAGNI that applies to code applies to
 * state. Promote it the day a second component actually needs it.
 *
 * Rejected alternative: `nuqs` URL state, which would make an expanded quote
 * deep-linkable. It would also push a history entry per expansion and turn the
 * back button into "undo a read" rather than "go back" — the same reason
 * `store/sidebar` keeps `drawerOpen` out of the URL.
 *
 * There is deliberately NO `persist` middleware: every field here is transient,
 * the class `drawerOpen`/`paletteOpen` occupy in `store/sidebar`, and wrapping a
 * store in `persist` with an empty `partialize` is ceremony around a no-op. If a
 * genuinely persisted field is ever added, add `persist` + `partialize` listing
 * only that field, exactly as `store/sidebar` does.
 */
interface SchedaGaraStore {
  /** The repair chat panel, collapsed by default — chat is demoted on this surface. */
  repairOpen: boolean;
  setRepairOpen: (open: boolean) => void;
  reset: () => void;
}

export const useSchedaGaraStore = create<SchedaGaraStore>()((set) => ({
  repairOpen: false,
  setRepairOpen: (repairOpen) => set({ repairOpen }),
  reset: () => set({ repairOpen: false }),
}));

export type { SchedaGaraStore };
