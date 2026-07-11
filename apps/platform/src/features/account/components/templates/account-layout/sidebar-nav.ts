export type SidebarNavKey = 'today' | 'explore' | 'workbenches';

/**
 * Primary nav for the app shell. Three destinations max (working-memory rule);
 * workspace settings live in the switcher popover, account settings in the
 * user footer. Without an active workspace only Explore is reachable.
 */
export function sidebarNavKeys(hasWorkspace: boolean): SidebarNavKey[] {
  return hasWorkspace ? ['today', 'explore', 'workbenches'] : ['explore'];
}
