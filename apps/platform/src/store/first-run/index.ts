import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

interface FirstRunStore {
  /** Workspace ids the advisor has explicitly dismissed first-run bid-profile capture for. */
  skippedWorkspaceIds: string[];
  skipWorkspace: (id: string) => void;
  hasSkipped: (id: string) => boolean;
}

/**
 * Tracks which workspaces the advisor skipped the first-run client-profile
 * capture for, so it doesn't reappear every visit within the same browser
 * session. Per-session like `useWorkspaceStore` (sessionStorage), not
 * permanent — a fresh browser session gets another chance to prompt.
 */
export const useFirstRunStore = create<FirstRunStore>()(
  persist(
    (set, get) => ({
      skippedWorkspaceIds: [],
      skipWorkspace: (id) =>
        set((s) =>
          s.skippedWorkspaceIds.includes(id)
            ? s
            : { skippedWorkspaceIds: [...s.skippedWorkspaceIds, id] },
        ),
      hasSkipped: (id) => get().skippedWorkspaceIds.includes(id),
    }),
    {
      name: 'first-run',
      storage: createJSONStorage(() => sessionStorage),
    },
  ),
);

export type { FirstRunStore };
