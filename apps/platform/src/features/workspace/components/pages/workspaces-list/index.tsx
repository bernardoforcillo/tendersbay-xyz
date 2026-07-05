import { Link, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { Button, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { useMyWorkspaces } from '~/features/workspace/hooks';
import { BTN_PRIMARY, CARD, ERROR_BOX, INPUT, LABEL } from '~/features/workspace/ui';
import { workspaceClient } from '~/lib/api/client';

export function WorkspacesListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: workspaces, loading, error } = useMyWorkspaces();
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  async function handleCreate(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setCreating(true);
    setCreateError(null);
    try {
      const res = await workspaceClient.createWorkspace({ name: name.trim(), slug: '' });
      if (res.workspace) {
        await navigate({
          to: '/workspaces/$workspaceId',
          params: { workspaceId: res.workspace.id },
        });
      }
    } catch (err: unknown) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create workspace');
    } finally {
      setCreating(false);
    }
  }

  return (
    <AccountLayout>
      <PageHeader
        title={t('workspace.list.title', 'Workspaces')}
        actions={
          <Button className={BTN_PRIMARY} onPress={() => setShowForm((v) => !v)}>
            {t('workspace.list.new', 'New workspace')}
          </Button>
        }
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 lg:p-8">
        {showForm && (
          <div className={CARD}>
            <Form onSubmit={handleCreate} className="flex flex-col gap-4">
              <TextField
                value={name}
                onChange={setName}
                isRequired
                className="flex flex-col gap-1.5"
              >
                <Label className={LABEL}>{t('workspace.list.nameLabel', 'Workspace name')}</Label>
                <Input
                  className={INPUT}
                  placeholder={t('workspace.list.namePlaceholder', 'Acme Procurement')}
                />
              </TextField>
              {createError && (
                <p role="alert" className={ERROR_BOX}>
                  {createError}
                </p>
              )}
              <Button
                type="submit"
                isDisabled={creating || name.trim() === ''}
                className={BTN_PRIMARY}
              >
                {creating
                  ? t('workspace.common.creating', 'Creating…')
                  : t('workspace.list.create', 'Create workspace')}
              </Button>
            </Form>
          </div>
        )}

        {loading && (
          <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
        )}
        {error && (
          <p role="alert" className={ERROR_BOX}>
            {error}
          </p>
        )}

        {workspaces && workspaces.length === 0 && !showForm && (
          <div className={`${CARD} text-center`}>
            <p className="text-sm text-ink-600">
              {t('workspace.list.empty', 'You are not part of any workspace yet.')}
            </p>
            <Button className={`${BTN_PRIMARY} mt-4`} onPress={() => setShowForm(true)}>
              {t('workspace.list.createFirst', 'Create your first workspace')}
            </Button>
          </div>
        )}

        {workspaces && workspaces.length > 0 && (
          <ul className="flex flex-col gap-2">
            {workspaces.map((w) => (
              <li key={w.id}>
                <Link
                  to="/workspaces/$workspaceId"
                  params={{ workspaceId: w.id }}
                  className={`${CARD} flex items-center justify-between no-underline transition hover:border-brand-300`}
                >
                  <span className="font-medium text-ink-900">{w.name}</span>
                  <span className="text-xs text-ink-400">@{w.slug}</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </AccountLayout>
  );
}
