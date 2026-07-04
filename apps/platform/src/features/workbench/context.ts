import type { Workbench } from '@tendersbay/proto/workbench/v1/workbench_pb';
import { createContext, useContext } from 'react';

export interface WorkbenchCtx {
  workbenchId: string;
  workbench: Workbench;
  myPermissions: bigint;
  workspaceName: string;
  refetch: () => void;
}

export const WorkbenchContext = createContext<WorkbenchCtx | null>(null);

export function useWorkbenchContext(): WorkbenchCtx {
  const ctx = useContext(WorkbenchContext);
  if (!ctx) throw new Error('useWorkbenchContext must be used within a WorkbenchLayout');
  return ctx;
}
