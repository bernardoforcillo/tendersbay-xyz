import { describe, expect, it } from 'vitest';
import { can, hasBit, Permission, toggleBit } from './permissions';

describe('workbench permissions', () => {
  it('administrator can do anything via can()', () => {
    expect(can(Permission.Administrator, Permission.ManageRoles)).toBe(true);
  });
  it('hasBit is literal (no admin implication)', () => {
    expect(hasBit(Permission.Administrator, Permission.ManageRoles)).toBe(false);
  });
  it('toggleBit sets and clears', () => {
    const p = toggleBit(0n, Permission.ManageMembers, true);
    expect(hasBit(p, Permission.ManageMembers)).toBe(true);
    expect(hasBit(toggleBit(p, Permission.ManageMembers, false), Permission.ManageMembers)).toBe(
      false,
    );
  });
});
