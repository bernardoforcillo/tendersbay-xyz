import { PageHeader as KitPageHeader } from '@tendersbay/components/core';
import { Menu, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import type { ReactNode } from 'react';
import { Button } from 'react-aria-components';
import { useSidebarStore } from '~/store/sidebar';

type PageHeaderProps = {
  /** Element before the title (e.g. a monogram tile). */
  leading?: ReactNode;
  /** Main heading. Omit for a toggle-only bar. */
  title?: ReactNode;
  /** Secondary line under the title. */
  subtitle?: ReactNode;
  /** Right-aligned actions (buttons, gear link). */
  actions?: ReactNode;
  /** Content below the header row (e.g. a tab nav). */
  children?: ReactNode;
};

const TOGGLE_BTN =
  'flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-ink-400 outline-none transition ' +
  'data-[hovered]:bg-cream-200 data-[hovered]:text-ink-700 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600';

function SidebarToggle() {
  const collapsed = useSidebarStore((s) => s.collapsed);
  const toggleCollapsed = useSidebarStore((s) => s.toggleCollapsed);
  const openDrawer = useSidebarStore((s) => s.openDrawer);

  return (
    <>
      {/* Mobile: open the drawer */}
      <Button
        onPress={openDrawer}
        aria-label="Open navigation"
        className={`${TOGGLE_BTN} lg:hidden`}
      >
        <Menu size={18} aria-hidden="true" />
      </Button>
      {/* Desktop: collapse / expand the rail */}
      <Button
        onPress={toggleCollapsed}
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        className={`${TOGGLE_BTN} hidden lg:flex`}
      >
        {collapsed ? (
          <PanelLeftOpen size={18} aria-hidden="true" />
        ) : (
          <PanelLeftClose size={18} aria-hidden="true" />
        )}
      </Button>
    </>
  );
}

export function PageHeader({ leading, title, subtitle, actions, children }: PageHeaderProps) {
  return (
    <KitPageHeader
      leading={
        <>
          <SidebarToggle />
          {leading}
        </>
      }
      title={title}
      subtitle={subtitle}
      actions={actions}
      className="sticky top-0 z-10 bg-cream-100 lg:bg-white"
    >
      {children}
    </KitPageHeader>
  );
}

export type { PageHeaderProps };
