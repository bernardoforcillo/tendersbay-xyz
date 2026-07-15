import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { cn, navItemClass } from '@tendersbay/components/core';
import {
  ChevronsUpDown,
  Home,
  LayoutGrid,
  LogOut,
  Search,
  Settings,
  Sparkles,
  X,
} from 'lucide-react';
import type { ReactNode } from 'react';
import { Button, Dialog, DialogTrigger, Popover } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { CommandPalette } from '~/features/account/components/organisms/command-palette';
import { Logo } from '~/features/landing/components/atoms';
import { WorkspaceSwitcher } from '~/features/workspace/components/organisms/workspace-switcher';
import { detectLocale } from '~/i18n/detect-locale';
import { authClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';
import { useRecentWorkbenchesStore } from '~/store/recent-workbenches';
import { useSidebarStore } from '~/store/sidebar';
import { useWorkspaceStore } from '~/store/workspace';
import { sidebarNavKeys } from './sidebar-nav';

// ─── Types ────────────────────────────────────────────────────────────────────

type AccountLayoutProps = {
  children?: ReactNode;
};

type SidebarContentProps = {
  showClose: boolean;
  onClose: () => void;
};

// ─── Styles ───────────────────────────────────────────────────────────────────

const POPUP_LINK =
  'flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm no-underline ' +
  'text-ink-700 transition-colors hover:bg-cream-100 hover:text-ink-900';

const ICON_BTN =
  'rounded-md p-1.5 text-ink-400 outline-none transition ' +
  'data-[hovered]:bg-cream-200 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

// ─── SidebarContent ───────────────────────────────────────────────────────────

function SidebarContent({ showClose, onClose }: SidebarContentProps) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const clearAuth = useAuthStore((s) => s.clearAuth);
  const user = useAuthStore((s) => s.user);
  const setPaletteOpen = useSidebarStore((s) => s.setPaletteOpen);
  const recent = useRecentWorkbenchesStore((s) => s.items);

  const initial = (user?.displayName?.[0] ?? user?.email?.[0] ?? '?').toUpperCase();
  // Keep the workspace nav visible even on workspace-agnostic routes (e.g. Explore)
  // by falling back to the remembered active workspace — navigating there should
  // not feel like leaving the workspace.
  const { workspaceId: routeWorkspaceId } = useParams({ strict: false });
  const currentWorkspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const workspaceId = routeWorkspaceId ?? currentWorkspaceId ?? undefined;
  const keys = sidebarNavKeys(Boolean(workspaceId));

  async function handleLogout() {
    try {
      await authClient.logout({});
    } catch {
      /* best-effort */
    }
    clearAuth();
    await navigate({ to: '/$locale/auth/login', params: { locale: detectLocale() } });
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-16 shrink-0 items-center gap-2 px-5">
        <Link
          to="/$locale"
          params={{ locale: i18n.language }}
          aria-label="tendersbay home"
          className="no-underline outline-none"
        >
          <Logo />
        </Link>

        {showClose && (
          <Button
            onPress={onClose}
            aria-label="Close navigation"
            className={cn(ICON_BTN, 'ml-auto')}
          >
            <X size={16} aria-hidden="true" />
          </Button>
        )}
      </div>

      {/* Workspace switcher */}
      <div className="px-4 pb-1">
        <WorkspaceSwitcher />
      </div>

      {/* ⌘K trigger */}
      <div className="px-4 pb-2">
        <Button
          onPress={() => setPaletteOpen(true)}
          className="flex w-full items-center gap-2.5 rounded-xl border border-cream-300 bg-white px-3 py-2 text-left text-sm text-ink-400 outline-none transition-colors duration-150 data-[hovered]:border-cream-400 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600"
        >
          <Search size={14} aria-hidden="true" className="shrink-0" />
          <span className="min-w-0 flex-1 truncate">
            {t('shell.palette.trigger', 'Search or ask…')}
          </span>
          <kbd className="shrink-0 rounded border border-cream-300 bg-cream-100 px-1.5 py-0.5 font-mono text-[10px] text-ink-400">
            ⌘K
          </kbd>
        </Button>
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-4 py-2">
        <ul className="space-y-0.5">
          {keys.includes('today') && workspaceId && (
            <li>
              <Link
                to="/workspaces/$workspaceId"
                params={{ workspaceId }}
                activeOptions={{ exact: true }}
                className={navItemClass}
              >
                <Home size={16} aria-hidden="true" className="shrink-0" />
                {t('shell.nav.today', 'Today')}
              </Link>
            </li>
          )}
          {keys.includes('explore') && (
            <li>
              <Link to="/explore" className={navItemClass}>
                <Sparkles size={16} aria-hidden="true" className="shrink-0" />
                {t('shell.nav.explore', 'Explore')}
              </Link>
            </li>
          )}
          {keys.includes('workbenches') && workspaceId && (
            <li>
              <Link
                to="/workspaces/$workspaceId/workbenches"
                params={{ workspaceId }}
                className={navItemClass}
              >
                <LayoutGrid size={16} aria-hidden="true" className="shrink-0" />
                {t('shell.nav.workbenches', 'Workbenches')}
              </Link>
            </li>
          )}
        </ul>

        {/* Recent workbenches */}
        {recent.length > 0 && (
          <div className="mt-4 border-t border-cream-300 pt-3">
            <p className="px-3 pb-1 font-mono text-[10px] font-semibold uppercase tracking-wide text-ink-400">
              {t('shell.recent', 'Recent')}
            </p>
            <ul className="space-y-0.5">
              {recent.map((item) => (
                <li key={item.workbenchId}>
                  <Link
                    to="/workspaces/$workspaceId/workbench/$workbenchId"
                    params={{ workspaceId: item.workspaceId, workbenchId: item.workbenchId }}
                    className={cn(navItemClass, 'py-1.5 text-[13px]')}
                  >
                    <span className="truncate">{item.name}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        )}
      </nav>

      {/* Footer — user menu */}
      <div className="shrink-0 p-4">
        <DialogTrigger>
          <Button className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left outline-none transition data-[hovered]:bg-cream-200 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-brand-100 text-xs font-semibold text-brand-700">
              {initial}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium text-ink-900">
                {user?.displayName}
              </span>
              <span className="block truncate text-xs text-ink-500">{user?.email}</span>
            </span>
            <ChevronsUpDown size={14} className="shrink-0 text-ink-400" aria-hidden="true" />
          </Button>

          <Popover
            placement="top start"
            offset={6}
            className="z-50 w-64 rounded-xl border border-cream-200 bg-white shadow-soft outline-none"
          >
            <Dialog aria-label="Account menu" className="outline-none">
              <div className="flex items-center gap-3 border-b border-cream-200 px-4 py-3">
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-100 text-sm font-semibold text-brand-700">
                  {initial}
                </span>
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink-900">{user?.displayName}</p>
                  <p className="truncate text-xs text-ink-500">{user?.email}</p>
                </div>
              </div>

              <div className="p-1.5">
                <Link to="/settings" className={POPUP_LINK}>
                  <Settings size={14} aria-hidden="true" className="shrink-0 text-ink-400" />
                  {t('shell.user.settings', 'Settings')}
                </Link>
              </div>

              <div className="border-t border-cream-200 p-1.5">
                <Button
                  onPress={handleLogout}
                  className="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm text-red-600 outline-none transition data-[hovered]:bg-red-50 data-[hovered]:text-red-700 data-[focus-visible]:ring-2 data-[focus-visible]:ring-red-600"
                >
                  <LogOut size={14} aria-hidden="true" className="shrink-0" />
                  {t('shell.user.logout', 'Log out')}
                </Button>
              </div>
            </Dialog>
          </Popover>
        </DialogTrigger>
      </div>
    </div>
  );
}

// ─── AccountLayout ────────────────────────────────────────────────────────────

export function AccountLayout({ children }: AccountLayoutProps) {
  const collapsed = useSidebarStore((s) => s.collapsed);
  const drawerOpen = useSidebarStore((s) => s.drawerOpen);
  const closeDrawer = useSidebarStore((s) => s.closeDrawer);

  return (
    <div className="relative flex min-h-screen bg-cream-100 lg:h-screen lg:overflow-hidden">
      <CommandPalette />

      {/* ── Mobile overlay ── */}
      {drawerOpen && (
        <button
          type="button"
          className="fixed inset-0 z-40 cursor-default bg-black/20 lg:hidden"
          onClick={closeDrawer}
          aria-label="Close navigation"
        />
      )}

      {/* ── Mobile sidebar drawer ── */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 w-64 overflow-hidden bg-cream-100 shadow-soft transition-transform duration-200 ease-in-out lg:hidden',
          drawerOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <SidebarContent showClose={true} onClose={closeDrawer} />
      </aside>

      {/* ── Desktop sidebar — floating rounded card ── */}
      <aside
        className={cn(
          'fixed inset-y-3 left-3 hidden overflow-hidden rounded-3xl bg-cream-100 transition-[width] duration-200 ease-in-out lg:block',
          collapsed ? 'w-0' : 'w-64',
        )}
      >
        <div className="h-full w-64">
          <SidebarContent showClose={false} onClose={closeDrawer} />
        </div>
      </aside>

      {/* ── Main ── */}
      <main
        className={cn(
          'flex min-h-screen flex-1 flex-col transition-[padding-left] duration-200 ease-in-out lg:h-screen lg:min-h-0',
          collapsed ? 'lg:pl-0' : 'lg:pl-[calc(16rem+0.75rem)]',
        )}
      >
        <div className="relative flex flex-1 flex-col lg:m-3 lg:min-h-0 lg:overflow-hidden lg:rounded-xl lg:bg-white lg:shadow-sm">
          {/* Scroll region — content scrolls inside the fixed-height card */}
          <div className="flex flex-1 flex-col lg:min-h-0 lg:overflow-y-auto">{children}</div>
        </div>
      </main>
    </div>
  );
}
