import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

export interface RecentWorkbench {
  workbenchId: string;
  workspaceId: string;
  name: string;
  visitedAt: number;
}

const MAX_RECENT = 5;

interface RecentWorkbenchesStore {
  items: RecentWorkbench[];
  record: (item: Omit<RecentWorkbench, 'visitedAt'>) => void;
}

/**
 * Client-side MRU of visited workbenches — there is no cross-workspace
 * "recent" RPC, so the sidebar's Recenti section is fed locally.
 */
export const useRecentWorkbenchesStore = create<RecentWorkbenchesStore>()(
  persist(
    (set) => ({
      items: [],
      record: (item) =>
        set((s) => ({
          items: [
            { ...item, visitedAt: Date.now() },
            ...s.items.filter((i) => i.workbenchId !== item.workbenchId),
          ].slice(0, MAX_RECENT),
        })),
    }),
    { name: 'recent-workbenches', storage: createJSONStorage(() => localStorage) },
  ),
);
