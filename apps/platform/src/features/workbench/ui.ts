// Shared Tailwind class strings for the workbench feature, matching the warm
// brand/ink/cream design system used across the app.

export const CARD = 'rounded-xl border border-cream-200 bg-white p-6 shadow-sm';

export const BTN_PRIMARY =
  'inline-flex items-center justify-center gap-2 rounded-xl bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition data-[hovered]:bg-brand-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

export const BTN_SECONDARY =
  'inline-flex items-center justify-center gap-2 rounded-xl border border-cream-300 bg-cream-50 px-4 py-2.5 text-sm font-medium text-ink-700 transition data-[hovered]:bg-cream-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

export const BTN_DANGER =
  'inline-flex items-center justify-center gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-2.5 text-sm font-medium text-red-700 transition data-[hovered]:bg-red-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-600 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60';

export const INPUT =
  'w-full rounded-xl border border-cream-300 bg-cream-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition placeholder:text-ink-300 focus:border-brand-400 focus:ring-2 focus:ring-brand-100';

export const LABEL = 'text-sm font-medium text-ink-700';

export const ERROR_BOX =
  'rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700';
