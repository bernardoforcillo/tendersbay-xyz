import { useState } from 'react';
import { Button, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkspaceContext } from '~/features/workspace/context';
import { useEmailInvites, useInviteLinks, useRoles } from '~/features/workspace/hooks';
import { can, Permission } from '~/features/workspace/permissions';
import {
  BTN_DANGER,
  BTN_PRIMARY,
  BTN_SECONDARY,
  CARD,
  ERROR_BOX,
  INPUT,
  LABEL,
} from '~/features/workspace/ui';
import { workspaceClient } from '~/lib/api/client';

const SELECT =
  'rounded-lg border border-cream-300 bg-cream-50 px-2.5 py-2 text-sm text-ink-800 outline-none transition focus:border-brand-400 focus:ring-2 focus:ring-brand-100';

export function WorkspaceInvitesPage() {
  const { t, i18n } = useTranslation();
  const { workspaceId, myPermissions } = useWorkspaceContext();
  const { data: roles } = useRoles(workspaceId);
  const { data: invites, refetch: refetchInvites } = useEmailInvites(workspaceId);
  const { data: links, refetch: refetchLinks } = useInviteLinks(workspaceId);
  const canCreate = can(myPermissions, Permission.CreateInvite);

  const [email, setEmail] = useState('');
  const [emailRole, setEmailRole] = useState('');
  const [linkRole, setLinkRole] = useState('');
  const [maxUses, setMaxUses] = useState('0');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const roleOptions = roles ?? [];
  const defaultRoleId = roleOptions.find((r) => r.isDefault)?.id ?? roleOptions[0]?.id ?? '';

  async function sendEmailInvite(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await workspaceClient.inviteByEmail({
        workspaceId,
        email: email.trim(),
        roleId: emailRole || defaultRoleId,
        locale: i18n.language,
      });
      setEmail('');
      refetchInvites();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to send invite');
    } finally {
      setBusy(false);
    }
  }

  async function createLink(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await workspaceClient.createInviteLink({
        workspaceId,
        roleId: linkRole || defaultRoleId,
        maxUses: Number.parseInt(maxUses, 10) || 0,
        expiresAt: '',
      });
      refetchLinks();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create link');
    } finally {
      setBusy(false);
    }
  }

  async function revokeInvite(id: string) {
    await workspaceClient.revokeEmailInvitation({ workspaceId, invitationId: id }).catch(() => {});
    refetchInvites();
  }

  async function revokeLink(id: string) {
    await workspaceClient.revokeInviteLink({ workspaceId, linkId: id }).catch(() => {});
    refetchLinks();
  }

  function linkUrl(code: string) {
    return `${window.location.origin}/join/${code}`;
  }

  async function copy(code: string) {
    try {
      await navigator.clipboard.writeText(linkUrl(code));
      setCopied(code);
      window.setTimeout(() => setCopied(null), 1500);
    } catch {
      /* clipboard unavailable */
    }
  }

  if (!canCreate) {
    return (
      <p className="text-sm text-ink-600">
        {t('workspace.invites.noPermission', 'You do not have permission to manage invitations.')}
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}

      {/* Email invitations */}
      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-ink-900">
          {t('workspace.invites.emailTitle', 'Invite by email')}
        </h2>
        <div className={CARD}>
          <Form onSubmit={sendEmailInvite} className="flex flex-col gap-4 sm:flex-row sm:items-end">
            <TextField
              value={email}
              onChange={setEmail}
              type="email"
              isRequired
              className="flex flex-1 flex-col gap-1.5"
            >
              <Label className={LABEL}>{t('workspace.invites.email', 'Email address')}</Label>
              <Input className={INPUT} placeholder="teammate@company.eu" />
            </TextField>
            <label className="flex flex-col gap-1.5">
              <span className={LABEL}>{t('workspace.invites.role', 'Role')}</span>
              <select
                className={SELECT}
                value={emailRole || defaultRoleId}
                onChange={(e) => setEmailRole(e.target.value)}
              >
                {roleOptions.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            </label>
            <Button type="submit" isDisabled={busy} className={BTN_PRIMARY}>
              {t('workspace.invites.send', 'Send invite')}
            </Button>
          </Form>
        </div>
        <ul className="flex flex-col gap-2">
          {invites?.map((inv) => (
            <li key={inv.id} className={`${CARD} flex items-center justify-between gap-3 py-3`}>
              <span className="truncate text-sm text-ink-800">{inv.email}</span>
              <Button className={BTN_DANGER} onPress={() => revokeInvite(inv.id)}>
                {t('workspace.invites.revoke', 'Revoke')}
              </Button>
            </li>
          ))}
        </ul>
      </section>

      {/* Shareable invite links */}
      <section className="flex flex-col gap-4">
        <h2 className="font-display text-lg text-ink-900">
          {t('workspace.invites.linkTitle', 'Shareable links')}
        </h2>
        <div className={CARD}>
          <Form onSubmit={createLink} className="flex flex-col gap-4 sm:flex-row sm:items-end">
            <label className="flex flex-col gap-1.5">
              <span className={LABEL}>{t('workspace.invites.role', 'Role')}</span>
              <select
                className={SELECT}
                value={linkRole || defaultRoleId}
                onChange={(e) => setLinkRole(e.target.value)}
              >
                {roleOptions.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.name}
                  </option>
                ))}
              </select>
            </label>
            <TextField
              value={maxUses}
              onChange={setMaxUses}
              type="number"
              className="flex flex-col gap-1.5"
            >
              <Label className={LABEL}>
                {t('workspace.invites.maxUses', 'Max uses (0 = unlimited)')}
              </Label>
              <Input className={INPUT} inputMode="numeric" min={0} />
            </TextField>
            <Button type="submit" isDisabled={busy} className={BTN_PRIMARY}>
              {t('workspace.invites.createLink', 'Create link')}
            </Button>
          </Form>
        </div>
        <ul className="flex flex-col gap-2">
          {links
            ?.filter((l) => !l.revoked)
            .map((l) => (
              <li
                key={l.id}
                className={`${CARD} flex flex-wrap items-center justify-between gap-3 py-3`}
              >
                <div className="min-w-0">
                  <p className="truncate font-mono text-xs text-ink-700">{linkUrl(l.code)}</p>
                  <p className="text-xs text-ink-500">
                    {l.maxUses > 0
                      ? t('workspace.invites.usesLimited', '{{used}} / {{max}} uses', {
                          used: l.useCount,
                          max: l.maxUses,
                        })
                      : t('workspace.invites.usesUnlimited', '{{used}} uses', { used: l.useCount })}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button className={BTN_SECONDARY} onPress={() => copy(l.code)}>
                    {copied === l.code
                      ? t('workspace.invites.copied', 'Copied')
                      : t('workspace.invites.copy', 'Copy')}
                  </Button>
                  <Button className={BTN_DANGER} onPress={() => revokeLink(l.id)}>
                    {t('workspace.invites.revoke', 'Revoke')}
                  </Button>
                </div>
              </li>
            ))}
        </ul>
      </section>
    </div>
  );
}
