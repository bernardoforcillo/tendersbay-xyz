export type SidebarNavKey = 'today' | 'tenders' | 'explore' | 'workbenches';

/**
 * Primary nav for the app shell. Workspace settings live in the switcher
 * popover, account settings in the user footer. Explore (chat) is only
 * available inside a workspace context; Tenders (search) is always reachable.
 */
export function sidebarNavKeys(hasWorkspace: boolean): SidebarNavKey[] {
  return hasWorkspace ? ['today', 'tenders', 'explore', 'workbenches'] : ['tenders'];
}
