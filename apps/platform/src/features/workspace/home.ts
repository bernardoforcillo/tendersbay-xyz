/**
 * Where an authenticated user lands from `/`: their active workspace if one is
 * remembered (persisted in the workspace store), otherwise the workspace list
 * to pick or create one.
 */
export type HomeTarget = { kind: 'workspace'; workspaceId: string } | { kind: 'list' };

export function homeTarget(currentWorkspaceId: string | null): HomeTarget {
  return currentWorkspaceId
    ? { kind: 'workspace', workspaceId: currentWorkspaceId }
    : { kind: 'list' };
}
