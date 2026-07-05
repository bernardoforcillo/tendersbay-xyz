import { describe, expect, it } from 'vitest';
import { homeTarget } from './home';

describe('homeTarget', () => {
  it('enters the remembered workspace', () => {
    expect(homeTarget('ws_123')).toEqual({ kind: 'workspace', workspaceId: 'ws_123' });
  });
  it('falls back to the workspace list when none is remembered', () => {
    expect(homeTarget(null)).toEqual({ kind: 'list' });
  });
});
