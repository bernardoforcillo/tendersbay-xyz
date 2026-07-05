import { describe, expect, it } from 'vitest';

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { workbench?: Record<string, Record<string, unknown>> } }
>;

const entries = Object.entries(modules);

const REQUIRED_SECTIONS = [
  'common',
  'nav',
  'errors',
  'breadcrumb',
  'visibility',
  'list',
  'members',
  'roles',
  'permissions',
  'permissionsHint',
  'settings',
] as const;

describe('workbench locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines a complete workbench block', (_path, mod) => {
    const wb = mod.default.workbench;
    expect(wb, 'workbench block').toBeTruthy();
    for (const section of REQUIRED_SECTIONS) {
      expect(wb?.[section], section).toBeTruthy();
    }
    expect(wb?.permissions?.administrator, 'permissions.administrator').toBeTruthy();
    expect(wb?.visibility?.private, 'visibility.private').toBeTruthy();
    expect(wb?.nav?.general, 'nav.general').toBeTruthy();
  });
});
