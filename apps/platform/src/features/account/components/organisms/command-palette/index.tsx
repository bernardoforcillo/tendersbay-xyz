import { useNavigate, useParams } from '@tanstack/react-router';
import { Search, Sparkles } from 'lucide-react';
import { useEffect, useState } from 'react';
import {
  Autocomplete,
  Dialog,
  Header,
  Input,
  Menu,
  MenuItem,
  MenuSection,
  Modal,
  ModalOverlay,
  SearchField,
  useFilter,
} from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useMyWorkspaces } from '~/features/workspace/hooks';
import { useChatStore } from '~/store/chat';
import { useRecentWorkbenchesStore } from '~/store/recent-workbenches';
import { useSidebarStore } from '~/store/sidebar';
import { useWorkspaceStore } from '~/store/workspace';
import { buildCommands, type PaletteCommand } from './command-list';

const SECTION_HEADER =
  'px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wide text-ink-400';
const ITEM =
  'flex cursor-default items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-ink-700 outline-none ' +
  'transition-colors duration-150 data-[focused]:bg-cream-200 data-[focused]:text-ink-900';

export function CommandPalette() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const open = useSidebarStore((s) => s.paletteOpen);
  const setOpen = useSidebarStore((s) => s.setPaletteOpen);
  const { workspaceId: routeWorkspaceId } = useParams({ strict: false });
  const currentWorkspaceId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const workspaceId = routeWorkspaceId ?? currentWorkspaceId ?? null;
  const recent = useRecentWorkbenchesStore((s) => s.items);
  const { data: workspaces } = useMyWorkspaces();
  const setDraft = useChatStore((s) => s.setDraft);
  const [query, setQuery] = useState('');
  const { contains } = useFilter({ sensitivity: 'base' });

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen(!useSidebarStore.getState().paletteOpen);
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [setOpen]);

  const commands = buildCommands({
    workspaceId,
    recent,
    workspaces: workspaces ?? [],
    labels: {
      today: t('shell.nav.today', 'Today'),
      explore: t('shell.nav.explore', 'Explore'),
      workbenches: t('shell.nav.workbenches', 'Workbenches'),
      workspaceSettings: t('shell.palette.workspaceSettings', 'Workspace settings'),
      accountSettings: t('shell.palette.accountSettings', 'Account settings'),
      allWorkspaces: t('shell.palette.allWorkspaces', 'All workspaces'),
    },
  });
  const sections: { key: PaletteCommand['section']; title: string }[] = [
    { key: 'navigate', title: t('shell.palette.sections.navigate', 'Go to') },
    { key: 'recent', title: t('shell.palette.sections.recent', 'Recent workbenches') },
    { key: 'workspaces', title: t('shell.palette.sections.workspaces', 'Workspaces') },
  ];

  function close() {
    setOpen(false);
    setQuery('');
  }

  function run(command: PaletteCommand) {
    close();
    void navigate({ to: command.to, params: command.params });
  }

  function ask() {
    const q = query.trim();
    close();
    if (q) setDraft(q);
    void navigate({ to: '/explore', params: undefined });
  }

  return (
    <ModalOverlay
      isOpen={open}
      onOpenChange={(next) => (next ? setOpen(true) : close())}
      isDismissable
      className="fixed inset-0 z-50 flex items-start justify-center bg-ink-950/25 p-4 pt-[15vh]"
    >
      <Modal className="w-full max-w-lg">
        <Dialog
          aria-label={t('shell.palette.label', 'Command palette')}
          className="overflow-hidden rounded-2xl border border-cream-200 bg-white shadow-soft-lg outline-none"
        >
          <Autocomplete filter={contains}>
            <SearchField
              aria-label={t('shell.palette.label', 'Command palette')}
              value={query}
              onChange={setQuery}
              className="flex items-center gap-2.5 border-b border-cream-200 px-4"
            >
              <Search size={16} aria-hidden="true" className="shrink-0 text-ink-400" />
              <Input
                autoFocus
                placeholder={t('shell.palette.placeholder', 'Search or ask anything…')}
                className="h-12 min-w-0 flex-1 bg-transparent text-sm text-ink-900 outline-none placeholder:text-ink-300 [&::-webkit-search-cancel-button]:hidden"
              />
            </SearchField>
            <Menu className="max-h-80 overflow-y-auto p-1.5 outline-none">
              {sections.map(({ key, title }) => {
                const items = commands.filter((c) => c.section === key);
                if (items.length === 0) return null;
                return (
                  <MenuSection key={key}>
                    <Header className={SECTION_HEADER}>{title}</Header>
                    {items.map((command) => (
                      <MenuItem
                        key={command.id}
                        id={command.id}
                        textValue={command.label}
                        onAction={() => run(command)}
                        className={ITEM}
                      >
                        {command.label}
                      </MenuItem>
                    ))}
                  </MenuSection>
                );
              })}
              <MenuSection>
                <MenuItem
                  id="ask-agent"
                  textValue={query || t('shell.palette.trigger', 'Search or ask…')}
                  onAction={ask}
                  className={ITEM}
                >
                  <Sparkles size={14} aria-hidden="true" className="shrink-0 text-brand-600" />
                  <span className="truncate">
                    {query.trim()
                      ? t('shell.palette.askAgent', {
                          defaultValue: 'Ask the agent: “{{query}}”',
                          query: query.trim(),
                        })
                      : t('shell.palette.trigger', 'Search or ask…')}
                  </span>
                </MenuItem>
              </MenuSection>
            </Menu>
          </Autocomplete>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
