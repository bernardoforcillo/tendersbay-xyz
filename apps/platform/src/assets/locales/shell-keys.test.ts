import { describe, expect, it } from 'vitest';

type LocaleModule = { default: Record<string, unknown> };

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  LocaleModule
>;
const entries = Object.entries(modules);

function get(obj: unknown, path: string): unknown {
  return path
    .split('.')
    .reduce<unknown>((acc, key) => (acc as Record<string, unknown> | undefined)?.[key], obj);
}

const REQUIRED_KEYS = [
  'shell.nav.today',
  'shell.nav.explore',
  'shell.nav.workbenches',
  'shell.recent',
  'shell.palette.trigger',
  'shell.palette.label',
  'shell.palette.placeholder',
  'shell.palette.askAgent',
  'shell.palette.sections.navigate',
  'shell.palette.sections.recent',
  'shell.palette.sections.workspaces',
  'shell.palette.workspaceSettings',
  'shell.palette.accountSettings',
  'shell.palette.allWorkspaces',
  'shell.user.settings',
  'shell.user.logout',
  'workspace.switcher.settings',
] as const;

describe('shell locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines every shell key', (_path, mod) => {
    for (const key of REQUIRED_KEYS) {
      expect(get(mod.default, key), key).toBeTruthy();
    }
  });

  it.each(entries)('%s keeps the {{query}} placeholder in askAgent', (_path, mod) => {
    expect(get(mod.default, 'shell.palette.askAgent')).toContain('{{query}}');
  });
});
