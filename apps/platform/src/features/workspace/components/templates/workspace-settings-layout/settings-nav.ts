/**
 * Settings hub sub-nav. Invites is permission-gated (create or manage invites);
 * the rest are shown to any workspace member.
 */
export type SettingsNavKey = 'general' | 'profile' | 'members' | 'roles' | 'invites';

export function settingsNavKeys(canInvite: boolean): SettingsNavKey[] {
  return canInvite
    ? ['general', 'profile', 'members', 'roles', 'invites']
    : ['general', 'profile', 'members', 'roles'];
}
