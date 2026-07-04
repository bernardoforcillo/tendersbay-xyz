// Client mirror of the backend workspace.Permission bitmask. Connect-ES maps a
// proto `uint64` to a JavaScript `bigint`, so every permission value here is a
// bigint and all masking uses BigInt operators.

export const Permission = {
  ViewWorkspace: 1n << 0n,
  ManageWorkspace: 1n << 1n,
  ManageMembers: 1n << 2n,
  ManageRoles: 1n << 3n,
  CreateInvite: 1n << 4n,
  ManageInvites: 1n << 5n,
  Administrator: 1n << 6n,
} as const;

/** The permission bits shown as toggles in the role editor, in display order. */
export const PERMISSION_KEYS = [
  'viewWorkspace',
  'manageWorkspace',
  'manageMembers',
  'manageRoles',
  'createInvite',
  'manageInvites',
  'administrator',
] as const;

export type PermissionKey = (typeof PERMISSION_KEYS)[number];

export const PERMISSION_BITS: Record<PermissionKey, bigint> = {
  viewWorkspace: Permission.ViewWorkspace,
  manageWorkspace: Permission.ManageWorkspace,
  manageMembers: Permission.ManageMembers,
  manageRoles: Permission.ManageRoles,
  createInvite: Permission.CreateInvite,
  manageInvites: Permission.ManageInvites,
  administrator: Permission.Administrator,
};

/** Whether a role literally has a bit set (used by the role editor). */
export function hasBit(perms: bigint, bit: bigint): boolean {
  return (perms & bit) === bit;
}

/** Set or clear a bit, returning the new mask (used by the role editor). */
export function toggleBit(perms: bigint, bit: bigint, on: boolean): bigint {
  return on ? perms | bit : perms & ~bit;
}

/**
 * Whether the effective permissions grant a capability — Administrator implies
 * everything. Use this for UI gating (showing/hiding actions); the server still
 * enforces the real check.
 */
export function can(perms: bigint, bit: bigint): boolean {
  if ((perms & Permission.Administrator) === Permission.Administrator) return true;
  return (perms & bit) === bit;
}
