import { describe, expect, it } from 'vitest';
import { can, hasBit, Permission, toggleBit } from './permissions';

describe('workspace permission bits', () => {
  it('sets and clears bits', () => {
    let p = 0n;
    p = toggleBit(p, Permission.ManageMembers, true);
    expect(hasBit(p, Permission.ManageMembers)).toBe(true);
    expect(hasBit(p, Permission.ManageRoles)).toBe(false);
    p = toggleBit(p, Permission.ManageMembers, false);
    expect(hasBit(p, Permission.ManageMembers)).toBe(false);
  });

  it('administrator implies every capability via can()', () => {
    const admin = Permission.Administrator;
    expect(can(admin, Permission.ManageWorkspace)).toBe(true);
    expect(can(admin, Permission.CreateInvite)).toBe(true);
    // hasBit is literal — the administrator bit alone does not set other bits.
    expect(hasBit(admin, Permission.ManageWorkspace)).toBe(false);
  });

  it('can() requires the exact bit without administrator', () => {
    const viewer = Permission.ViewWorkspace;
    expect(can(viewer, Permission.ViewWorkspace)).toBe(true);
    expect(can(viewer, Permission.ManageMembers)).toBe(false);
  });
});
