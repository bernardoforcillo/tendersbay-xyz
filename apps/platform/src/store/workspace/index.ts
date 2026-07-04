import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

interface WorkspaceStore {
  /** The last workspace the user was active in — drives the switcher default. */
  currentWorkspaceId: string | null;
  setCurrentWorkspace: (id: string | null) => void;
}

export const useWorkspaceStore = create<WorkspaceStore>()(
  persist(
    (set) => ({
      currentWorkspaceId: null,
      setCurrentWorkspace: (id) => set({ currentWorkspaceId: id }),
    }),
    {
      name: 'workspace',
      storage: createJSONStorage(() => sessionStorage),
    },
  ),
);

export type { WorkspaceStore };
