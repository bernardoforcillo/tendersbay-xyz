import { describe, expect, it } from 'vitest';
import { settingsNavKeys } from './settings-nav';

describe('settingsNavKeys', () => {
  it('hides Invites without invite permission', () => {
    expect(settingsNavKeys(false)).toEqual(['general', 'members', 'roles']);
  });
  it('shows Invites with invite permission', () => {
    expect(settingsNavKeys(true)).toEqual(['general', 'members', 'roles', 'invites']);
  });
});
