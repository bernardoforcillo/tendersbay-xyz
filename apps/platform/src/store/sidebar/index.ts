import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

interface SidebarStore {
  /** Desktop rail collapsed — persisted across sessions. */
  collapsed: boolean;
  /** Mobile drawer open — ephemeral UI state, never persisted. */
  drawerOpen: boolean;
  /** Palette open state — ephemeral UI state, never persisted. */
  paletteOpen: boolean;
  toggleCollapsed: () => void;
  setCollapsed: (v: boolean) => void;
  openDrawer: () => void;
  closeDrawer: () => void;
  setPaletteOpen: (open: boolean) => void;
}

export const useSidebarStore = create<SidebarStore>()(
  persist(
    (set) => ({
      collapsed: false,
      drawerOpen: false,
      paletteOpen: false,
      toggleCollapsed: () => set((s) => ({ collapsed: !s.collapsed })),
      setCollapsed: (v) => set({ collapsed: v }),
      openDrawer: () => set({ drawerOpen: true }),
      closeDrawer: () => set({ drawerOpen: false }),
      setPaletteOpen: (open) => set({ paletteOpen: open }),
    }),
    {
      name: 'sidebar',
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ collapsed: s.collapsed }),
    },
  ),
);

export type { SidebarStore };
