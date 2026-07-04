/**
 * Which primary sidebar items are visible. Overview and Settings are
 * workspace-scoped, so they show whenever a workspace is active — either the
 * current route's workspace or the remembered active one — so workspace-agnostic
 * routes like Explore keep them. Explore is always available.
 */
export type SidebarNavKey = 'overview' | 'explore' | 'settings';

export function sidebarNavKeys(hasWorkspace: boolean): SidebarNavKey[] {
  return hasWorkspace ? ['overview', 'explore', 'settings'] : ['explore'];
}
