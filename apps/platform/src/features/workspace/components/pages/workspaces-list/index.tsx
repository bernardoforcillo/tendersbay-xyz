import { Link, useNavigate } from '@tanstack/react-router';
import { Banner, Button, Card, EmptyState, Field } from '@tendersbay/components/core';
import { Building2 } from 'lucide-react';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { useMyWorkspaces } from '~/features/workspace/hooks';
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
          <Button onPress={() => setShowForm((v) => !v)}>
            {t('workspace.list.new', 'New workspace')}
          </Button>
        }
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6 lg:p-8">
        {showForm && (
          <Card>
            <Form onSubmit={handleCreate} className="flex flex-col gap-4">
              <Field
                label={t('workspace.list.nameLabel', 'Workspace name')}
                placeholder={t('workspace.list.namePlaceholder', 'Acme Procurement')}
                value={name}
                onChange={setName}
                isRequired
              />
              {createError && <Banner tone="error">{createError}</Banner>}
              <Button type="submit" isDisabled={creating || name.trim() === ''}>
                {creating
                  ? t('workspace.common.creating', 'Creating…')
                  : t('workspace.list.create', 'Create workspace')}
              </Button>
            </Form>
          </Card>
        )}

        {loading && (
          <p className="text-sm text-ink-500">{t('workspace.common.loading', 'Loading…')}</p>
        )}
        {error && <Banner tone="error">{error}</Banner>}

        {workspaces && workspaces.length === 0 && !showForm && (
          <EmptyState
            icon={<Building2 size={28} />}
            title={t('workspace.list.empty', 'You are not part of any workspace yet.')}
            action={
              <Button onPress={() => setShowForm(true)}>
                {t('workspace.list.createFirst', 'Create your first workspace')}
              </Button>
            }
          />
        )}

        {workspaces && workspaces.length > 0 && (
          <ul className="flex flex-col gap-2">
            {workspaces.map((w) => (
              <li key={w.id}>
                <Card padding="none">
                  <Link
                    to="/workspaces/$workspaceId"
                    params={{ workspaceId: w.id }}
                    className="flex items-center justify-between gap-3 rounded-2xl p-5 no-underline outline-none transition-colors duration-150 hover:bg-cream-50 focus-visible:ring-2 focus-visible:ring-brand-600"
                  >
                    <span className="font-medium text-ink-900">{w.name}</span>
                    <span className="text-xs text-ink-400">@{w.slug}</span>
                  </Link>
                </Card>
              </li>
            ))}
          </ul>
        )}
      </div>
    </AccountLayout>
  );
}
