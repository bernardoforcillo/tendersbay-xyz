/**
 * Settings hub sub-nav. Invites is permission-gated (create or manage invites);
 * the rest are shown to any workspace member.
 */
export type SettingsNavKey = 'general' | 'profile' | 'company' | 'members' | 'roles' | 'invites';

/**
 * `company` sits next to `profile` because they are the two halves of one
 * question and are constantly mistaken for each other: the client profile says
 * what this operator WANTS to bid on, the company dossier says what it may
 * lawfully be admitted to. Separating them across the nav would invite entering
 * capability data into the preferences form.
 */
export function settingsNavKeys(canInvite: boolean): SettingsNavKey[] {
  return canInvite
    ? ['general', 'profile', 'company', 'members', 'roles', 'invites']
    : ['general', 'profile', 'company', 'members', 'roles'];
}
