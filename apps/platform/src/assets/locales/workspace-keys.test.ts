import { describe, expect, it } from 'vitest';

const modules = import.meta.glob('./*/common.json', { eager: true }) as Record<
  string,
  { default: { workspace?: Record<string, Record<string, unknown>> } }
>;

const entries = Object.entries(modules);

const REQUIRED_SECTIONS = [
  'common',
  'nav',
  'list',
  'overview',
  'members',
  'roles',
  'permissions',
  'invites',
  'settings',
  'accept',
  'join',
  'switcher',
  'firstRun',
  'clientProfile',
] as const;

describe('workspace locale keys', () => {
  it('covers all 24 locales', () => {
    expect(entries).toHaveLength(24);
  });

  it.each(entries)('%s defines a complete workspace block', (_path, mod) => {
    const ws = mod.default.workspace;
    expect(ws, 'workspace block').toBeTruthy();
    for (const section of REQUIRED_SECTIONS) {
      expect(ws?.[section], section).toBeTruthy();
    }
    // Spot-check the permission labels used by the role editor are all present.
    expect(ws?.permissions?.administrator, 'permissions.administrator').toBeTruthy();
    expect(ws?.accept?.body, 'accept.body').toBeTruthy();
    expect(ws?.nav?.general, 'nav.general').toBeTruthy();
    expect(ws?.nav?.profile, 'nav.profile').toBeTruthy();
    expect(ws?.firstRun?.skip, 'firstRun.skip').toBeTruthy();
  });
});
