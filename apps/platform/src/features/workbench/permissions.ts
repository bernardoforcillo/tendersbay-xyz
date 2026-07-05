// Client mirror of the backend workbench.Permission bitmask (bigint, since
// Connect-ES maps proto uint64 → bigint).
export const Permission = {
  ViewWorkbench: 1n << 0n,
  ManageWorkbench: 1n << 1n,
  ManageMembers: 1n << 2n,
  ManageRoles: 1n << 3n,
  Administrator: 1n << 6n,
} as const;

export const PERMISSION_KEYS = [
  'viewWorkbench',
  'manageWorkbench',
  'manageMembers',
  'manageRoles',
  'administrator',
] as const;

export type PermissionKey = (typeof PERMISSION_KEYS)[number];

export const PERMISSION_BITS: Record<PermissionKey, bigint> = {
  viewWorkbench: Permission.ViewWorkbench,
  manageWorkbench: Permission.ManageWorkbench,
  manageMembers: Permission.ManageMembers,
  manageRoles: Permission.ManageRoles,
  administrator: Permission.Administrator,
};

export function hasBit(perms: bigint, bit: bigint): boolean {
  return (perms & bit) === bit;
}

export function toggleBit(perms: bigint, bit: bigint, on: boolean): bigint {
  return on ? perms | bit : perms & ~bit;
}

/** UI gating — Administrator implies everything. Server still enforces. */
export function can(perms: bigint, bit: bigint): boolean {
  if ((perms & Permission.Administrator) === Permission.Administrator) return true;
  return (perms & bit) === bit;
}
