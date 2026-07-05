import type { Workspace } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { createContext, useContext } from 'react';

export interface WorkspaceCtx {
  workspaceId: string;
  workspace: Workspace;
  /** The caller's effective permission bitmask in this workspace. */
  myPermissions: bigint;
  /** Re-fetch the workspace + permissions (e.g. after a settings change). */
  refetch: () => void;
}

export const WorkspaceContext = createContext<WorkspaceCtx | null>(null);

export function useWorkspaceContext(): WorkspaceCtx {
  const ctx = useContext(WorkspaceContext);
  if (!ctx) {
    throw new Error('useWorkspaceContext must be used within a WorkspaceLayout');
  }
  return ctx;
}
