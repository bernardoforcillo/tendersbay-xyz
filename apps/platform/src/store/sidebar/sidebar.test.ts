import { beforeEach, describe, expect, it } from 'vitest';
import { useSidebarStore } from './index';

describe('useSidebarStore', () => {
  beforeEach(() => {
    useSidebarStore.setState({ collapsed: false, drawerOpen: false });
    localStorage.clear();
  });

  it('toggles the desktop collapsed flag', () => {
    expect(useSidebarStore.getState().collapsed).toBe(false);
    useSidebarStore.getState().toggleCollapsed();
    expect(useSidebarStore.getState().collapsed).toBe(true);
    useSidebarStore.getState().toggleCollapsed();
    expect(useSidebarStore.getState().collapsed).toBe(false);
  });

  it('sets the collapsed flag explicitly', () => {
    useSidebarStore.getState().setCollapsed(true);
    expect(useSidebarStore.getState().collapsed).toBe(true);
  });

  it('opens and closes the mobile drawer', () => {
    useSidebarStore.getState().openDrawer();
    expect(useSidebarStore.getState().drawerOpen).toBe(true);
    useSidebarStore.getState().closeDrawer();
    expect(useSidebarStore.getState().drawerOpen).toBe(false);
  });

  it('persists only collapsed to localStorage', () => {
    useSidebarStore.getState().setCollapsed(true);
    useSidebarStore.getState().openDrawer();
    const stored = JSON.parse(localStorage.getItem('sidebar') ?? '{}');
    expect(stored.state.collapsed).toBe(true);
    expect(stored.state.drawerOpen).toBeUndefined();
  });
});
