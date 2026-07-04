/**
 * Which primary sidebar items are visible. Overview and Settings are
 * workspace-scoped, so they show only while a workspace route is active;
 * Explore is always available.
 */
export type SidebarNavKey = 'overview' | 'explore' | 'settings';

export function sidebarNavKeys(hasWorkspace: boolean): SidebarNavKey[] {
  return hasWorkspace ? ['overview', 'explore', 'settings'] : ['explore'];
}
