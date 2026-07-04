import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useState } from 'react';
import { Button, Form, Input, Label, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { useWorkbenches } from '~/features/workbench/hooks';
import { BTN_PRIMARY, CARD, ERROR_BOX, INPUT, LABEL } from '~/features/workbench/ui';
import { workbenchClient } from '~/lib/api/client';

const SELECT =
  'w-full rounded-xl border border-cream-300 bg-cream-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition focus:border-brand-400 focus:ring-2 focus:ring-brand-100';

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
    <div className="flex flex-col gap-6 p-6 lg:p-8">
      <h1 className="font-display text-2xl text-ink-900">
        {t('workbench.list.title', 'Workbenches')}
      </h1>

      <div className={CARD}>
        <h2 className="mb-3 font-display text-lg text-ink-900">
          {t('workbench.list.new', 'New workbench')}
        </h2>
        <Form
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) create();
          }}
          className="flex flex-col gap-3"
        >
          <TextField value={name} onChange={setName} isRequired className="flex flex-col gap-1.5">
            <Label className={LABEL}>{t('workbench.list.nameLabel', 'Workbench name')}</Label>
            <Input className={INPUT} placeholder={t('workbench.list.namePlaceholder', 'Q3 Bids')} />
          </TextField>
          <TextField
            value={description}
            onChange={setDescription}
            className="flex flex-col gap-1.5"
          >
            <Label className={LABEL}>{t('workbench.list.descriptionLabel', 'Description')}</Label>
            <Input className={INPUT} />
          </TextField>
          <div className="flex flex-col gap-1.5">
            <Label className={LABEL} htmlFor="wb-visibility">
              {t('workbench.visibility.label', 'Visibility')}
            </Label>
            <select
              id="wb-visibility"
              className={SELECT}
              value={visibility}
              onChange={(e) => setVisibility(e.target.value)}
            >
              <option value="private">{t('workbench.visibility.private', 'Private')}</option>
              <option value="shared">{t('workbench.visibility.shared', 'Shared')}</option>
            </select>
          </div>
          {formError && (
            <p role="alert" className={ERROR_BOX}>
              {formError}
            </p>
          )}
          <Button type="submit" isDisabled={busy || name.trim() === ''} className={BTN_PRIMARY}>
            {busy
              ? t('workbench.common.creating', 'Creating…')
              : t('workbench.list.create', 'Create workbench')}
          </Button>
        </Form>
      </div>

      {loading && (
        <p className="text-sm text-ink-500">{t('workbench.common.loading', 'Loading…')}</p>
      )}
      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}
      {workbenches && workbenches.length === 0 && !loading && (
        <p className="text-sm text-ink-500">{t('workbench.list.empty', 'No workbenches yet.')}</p>
      )}
      <ul className="flex flex-col gap-2">
        {workbenches?.map((wb) => (
          <li key={wb.id}>
            <Link
              to="/workspaces/$workspaceId/workbench/$workbenchId"
              params={{ workspaceId, workbenchId: wb.id }}
              className={`${CARD} flex items-center justify-between gap-3 no-underline outline-none transition hover:border-brand-300 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2`}
            >
              <span className="min-w-0">
                <span className="block truncate font-medium text-ink-900">{wb.name}</span>
                {wb.description && (
                  <span className="block truncate text-xs text-ink-500">{wb.description}</span>
                )}
              </span>
              <span className="shrink-0 rounded-full bg-cream-200 px-2 py-0.5 text-xs text-ink-600">
                {t(`workbench.visibility.${wb.visibility}`, wb.visibility)}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
