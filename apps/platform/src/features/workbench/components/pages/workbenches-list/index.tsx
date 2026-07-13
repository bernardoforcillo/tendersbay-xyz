import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { Banner, Button, Card, EmptyState, Field, Pill, Select } from '@tendersbay/components/core';
import { LayoutGrid, Lock, Users } from 'lucide-react';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { PageHeader } from '~/features/account/components/organisms';
import { useWorkbenches } from '~/features/workbench/hooks';
import { workbenchClient } from '~/lib/api/client';

export function WorkbenchesListPage() {
  const { t } = useTranslation();
  const { workspaceId } = useParams({
    from: '/_authenticated/workspaces/$workspaceId/workbenches',
  });
  const navigate = useNavigate();
  const { data: workbenches, loading, error, refetch } = useWorkbenches(workspaceId);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [visibility, setVisibility] = useState('private');
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  async function create() {
    setBusy(true);
    setFormError(null);
    try {
      const res = await workbenchClient.createWorkbench({
        workspaceId,
        name: name.trim(),
        description: description.trim(),
        visibility,
      });
      setName('');
      setDescription('');
      setShowForm(false);
      refetch();
      if (res.workbench) {
        await navigate({
          to: '/workspaces/$workspaceId/workbench/$workbenchId',
          params: { workspaceId, workbenchId: res.workbench.id },
        });
      }
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'Failed to create workbench');
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <PageHeader
        title={t('workbench.list.title', 'Workbenches')}
        actions={
          <Button onPress={() => setShowForm((v) => !v)}>
            {t('workbench.list.new', 'New workbench')}
          </Button>
        }
      />
      <div className="flex flex-col gap-6 p-6 lg:p-8">
        {showForm && (
          <Card>
            <Form
              onSubmit={(e) => {
                e.preventDefault();
                if (name.trim()) create();
              }}
              className="flex flex-col gap-3"
            >
              <Field
                label={t('workbench.list.nameLabel', 'Workbench name')}
                placeholder={t('workbench.list.namePlaceholder', 'Q3 Bids')}
                value={name}
                onChange={setName}
                isRequired
              />
              <Field
                label={t('workbench.list.descriptionLabel', 'Description')}
                value={description}
                onChange={setDescription}
              />
              <Select
                label={t('workbench.visibility.label', 'Visibility')}
                className="w-full"
                value={visibility}
                onChange={(e) => setVisibility(e.target.value)}
              >
                <option value="private">{t('workbench.visibility.private', 'Private')}</option>
                <option value="shared">{t('workbench.visibility.shared', 'Shared')}</option>
              </Select>
              {formError && <Banner tone="error">{formError}</Banner>}
              <Button type="submit" isDisabled={busy || name.trim() === ''}>
                {busy
                  ? t('workbench.common.creating', 'Creating…')
                  : t('workbench.list.create', 'Create workbench')}
              </Button>
            </Form>
          </Card>
        )}

        {loading && (
          <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>
        )}
        {error && <Banner tone="error">{error}</Banner>}
        {workbenches && workbenches.length === 0 && !loading && (
          <EmptyState
            icon={<LayoutGrid size={28} />}
            title={t('workbench.list.empty', 'No workbenches yet.')}
            action={
              <Button onPress={() => setShowForm(true)}>
                {t('workbench.list.new', 'New workbench')}
              </Button>
            }
          />
        )}
        <ul className="flex flex-col gap-2">
          {workbenches?.map((wb) => (
            <li key={wb.id}>
              <Card padding="none">
                <Link
                  to="/workspaces/$workspaceId/workbench/$workbenchId"
                  params={{ workspaceId, workbenchId: wb.id }}
                  className="flex items-center gap-4 rounded-2xl p-5 no-underline outline-none transition-colors duration-150 hover:bg-cream-50 focus-visible:ring-2 focus-visible:ring-brand-600"
                >
                  <span
                    aria-hidden="true"
                    className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-100 font-display text-base font-semibold text-brand-700"
                  >
                    {wb.name.charAt(0).toUpperCase()}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium text-ink-900">{wb.name}</span>
                    {wb.description && (
                      <span className="block truncate text-xs text-ink-500">{wb.description}</span>
                    )}
                  </span>
                  <Pill tone="neutral" className="shrink-0 gap-1.5">
                    {wb.visibility === 'shared' ? (
                      <Users size={14} aria-hidden="true" />
                    ) : (
                      <Lock size={14} aria-hidden="true" />
                    )}
                    {t(`workbench.visibility.${wb.visibility}`, wb.visibility)}
                  </Pill>
                </Link>
              </Card>
            </li>
          ))}
        </ul>
      </div>
    </>
  );
}
