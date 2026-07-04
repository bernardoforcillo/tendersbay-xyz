import { Link, useNavigate, useParams } from '@tanstack/react-router';
import {
  ChevronsUpDown,
  Home,
  LayoutGrid,
  LogOut,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  Sparkles,
  X,
} from 'lucide-react';
import { type ReactNode, useState } from 'react';
import { Button, Dialog, DialogTrigger, Popover } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Logo } from '~/features/landing/components/atoms';
import { WorkspaceSwitcher } from '~/features/workspace/components/organisms/workspace-switcher';
import { detectLocale } from '~/i18n/detect-locale';
import { authClient } from '~/lib/api/client';
import { useAuthStore } from '~/store/auth';
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

const NAV_ITEM =
  'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium no-underline ' +
  'text-ink-500 transition-colors hover:bg-cream-200 hover:text-ink-900 ' +
  '[&[aria-current=page]]:bg-cream-200 [&[aria-current=page]]:text-ink-900';

const POPUP_LINK =
  'flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm no-underline ' +
  'text-ink-700 transition-colors hover:bg-cream-100 hover:text-ink-900';

const ICON_BTN =
  'rounded-md p-1.5 text-ink-400 outline-none transition ' +
  'data-[hovered]:bg-cream-200 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

// ─── SidebarContent ───────────────────────────────────────────────────────────

function SidebarContent({ showClose, onClose }: SidebarContentProps) {
  const { i18n } = useTranslation();
  const navigate = useNavigate();
  const clearAuth = useAuthStore((s) => s.clearAuth);
  const user = useAuthStore((s) => s.user);

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
          <Button onPress={onClose} aria-label="Close navigation" className={`${ICON_BTN} ml-auto`}>
            <X size={16} aria-hidden="true" />
          </Button>
        )}
      </div>

      {/* Workspace switcher */}
      <div className="px-4 pb-1">
        <WorkspaceSwitcher />
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-4 py-3">
        <ul className="space-y-0.5">
          {keys.includes('overview') && workspaceId && (
            <li>
              <Link
                to="/workspaces/$workspaceId"
                params={{ workspaceId }}
                activeOptions={{ exact: true }}
                className={NAV_ITEM}
              >
                <Home size={16} aria-hidden="true" className="shrink-0" />
                Overview
              </Link>
            </li>
          )}
          {keys.includes('workbenches') && workspaceId && (
            <li>
              <Link
                to="/workspaces/$workspaceId/workbenches"
                params={{ workspaceId }}
                className={NAV_ITEM}
              >
                <LayoutGrid size={16} aria-hidden="true" className="shrink-0" />
                Workbenches
              </Link>
            </li>
          )}
          {keys.includes('explore') && (
            <li>
              <Link to="/explore" className={NAV_ITEM}>
                <Sparkles size={16} aria-hidden="true" className="shrink-0" />
                Explore
              </Link>
            </li>
          )}
          {keys.includes('settings') && workspaceId && (
            <li>
              <Link
                to="/workspaces/$workspaceId/settings"
                params={{ workspaceId }}
                className={NAV_ITEM}
              >
                <Settings size={16} aria-hidden="true" className="shrink-0" />
                Settings
              </Link>
            </li>
          )}
        </ul>
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
                  Settings
                </Link>
              </div>

              <div className="border-t border-cream-200 p-1.5">
                <Button
                  onPress={handleLogout}
                  className="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm text-red-600 outline-none transition data-[hovered]:bg-red-50 data-[hovered]:text-red-700 data-[focus-visible]:ring-2 data-[focus-visible]:ring-red-600"
                >
                  <LogOut size={14} aria-hidden="true" className="shrink-0" />
                  Log out
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
  const { i18n } = useTranslation();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem('sidebar-collapsed') === 'true';
    } catch {
      return false;
    }
  });

  function toggleCollapsed() {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem('sidebar-collapsed', String(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  }

  return (
    <div className="relative flex min-h-screen bg-cream-100 lg:h-screen lg:overflow-hidden">
      {/* ── Mobile header ── */}
      <header className="sticky top-0 z-40 flex h-14 items-center gap-3 border-b border-cream-200 bg-cream-100 px-4 lg:hidden">
        <Button
          onPress={() => setSidebarOpen(true)}
          aria-label="Open navigation"
          className={ICON_BTN}
        >
          <Menu size={18} aria-hidden="true" />
        </Button>
        <Link
          to="/$locale"
          params={{ locale: i18n.language }}
          aria-label="tendersbay home"
          className="no-underline outline-none"
        >
          <Logo />
        </Link>
      </header>

      {/* ── Mobile overlay ── */}
      {sidebarOpen && (
        <button
          type="button"
          className="fixed inset-0 z-40 cursor-default bg-black/20 lg:hidden"
          onClick={() => setSidebarOpen(false)}
          aria-label="Close navigation"
        />
      )}

      {/* ── Mobile sidebar drawer ── */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 overflow-hidden bg-cream-100 shadow-soft transition-transform duration-200 ease-in-out lg:hidden ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}`}
      >
        <SidebarContent showClose={true} onClose={() => setSidebarOpen(false)} />
      </aside>

      {/* ── Desktop sidebar — floating rounded card ── */}
      <aside
        className={`fixed inset-y-3 left-3 hidden overflow-hidden rounded-3xl bg-cream-100 transition-[width] duration-200 ease-in-out lg:block ${collapsed ? 'w-0' : 'w-64'}`}
      >
        <div className="h-full w-64">
          <SidebarContent showClose={false} onClose={() => setSidebarOpen(false)} />
        </div>
      </aside>

      {/* ── Main ── */}
      <main
        className={`flex min-h-screen flex-1 flex-col transition-[padding-left] duration-200 ease-in-out lg:h-screen lg:min-h-0 ${collapsed ? 'lg:pl-0' : 'lg:pl-[calc(16rem+0.75rem)]'}`}
      >
        <div className="relative flex flex-1 flex-col lg:m-3 lg:min-h-0 lg:overflow-hidden lg:rounded-xl lg:bg-white lg:shadow-sm">
          {/* Toggle — no background, sits at the top-left corner of the card */}
          <Button
            onPress={toggleCollapsed}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            className="absolute left-0 top-0 z-10 hidden h-11 w-11 items-center justify-center text-ink-400 outline-none data-[hovered]:text-ink-700 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 lg:flex"
          >
            {collapsed ? (
              <PanelLeftOpen size={16} aria-hidden="true" />
            ) : (
              <PanelLeftClose size={16} aria-hidden="true" />
            )}
          </Button>

          {/* Scroll region — content scrolls inside the fixed-height card */}
          <div className="flex flex-1 flex-col lg:min-h-0 lg:overflow-y-auto">{children}</div>
        </div>
      </main>
    </div>
  );
}
