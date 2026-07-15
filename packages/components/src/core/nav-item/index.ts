/**
 * className for sidebar/nav links. Current-page styling is driven by the
 * `aria-current="page"` attribute the router sets on the active link, so this
 * stays a static string and the kit stays router-agnostic.
 */
export const navItemClass =
  'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium no-underline ' +
  'text-ink-500 transition-colors duration-150 hover:bg-cream-200 hover:text-ink-900 ' +
  'outline-none focus-visible:ring-2 focus-visible:ring-brand-600 ' +
  '[&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

/**
 * className for tab links (e.g. a settings/gear sub-nav). Same current-page
 * pattern as `navItemClass`, sized for a horizontal tab strip rather than a
 * sidebar list.
 */
export const tabClass =
  'flex items-center gap-2 min-h-10 rounded-lg px-3 py-2 text-sm font-medium no-underline ' +
  'text-ink-500 transition-colors duration-150 hover:bg-cream-200 hover:text-ink-900 ' +
  'outline-none focus-visible:ring-2 focus-visible:ring-brand-600 ' +
  '[&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';
