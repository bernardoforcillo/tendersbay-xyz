import { Banner, Button, Field, Switch } from '@tendersbay/components/core';
import { useState } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import {
  hasBit,
  PERMISSION_BITS,
  PERMISSION_KEYS,
  toggleBit,
} from '~/features/workbench/permissions';

type RoleEditorProps = {
  initialName?: string;
  initialPermissions?: bigint;
  submitting?: boolean;
  error?: string | null;
  /** Whether the current user may grant a given permission bit (server enforces too). */
  canGrant: (bit: bigint) => boolean;
  onSubmit: (name: string, permissions: bigint) => void;
  onCancel: () => void;
};

export function RoleEditor({
  initialName = '',
  initialPermissions = 0n,
  submitting,
  error,
  canGrant,
  onSubmit,
  onCancel,
}: RoleEditorProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialName);
  const [perms, setPerms] = useState(initialPermissions);

  return (
    <Form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(name.trim(), perms);
      }}
      className="flex flex-col gap-4"
    >
      <Field
        label={t('workbench.roles.nameLabel', 'Role name')}
        placeholder={t('workbench.roles.namePlaceholder', 'Editor')}
        value={name}
        onChange={setName}
        isRequired
      />

      <div className="flex flex-col gap-1">
        <span className="text-sm font-medium text-ink-700">
          {t('workbench.roles.permissionsLabel', 'Permissions')}
        </span>
        <div className="mt-1 flex flex-col divide-y divide-cream-200 rounded-xl border border-cream-200">
          {PERMISSION_KEYS.map((key) => {
            const bit = PERMISSION_BITS[key];
            const grantable = canGrant(bit);
            return (
              <Switch
                key={key}
                isSelected={hasBit(perms, bit)}
                isDisabled={!grantable}
                onChange={(v) => setPerms(toggleBit(perms, bit, v))}
                className="w-full justify-between px-4 py-3 data-[disabled]:opacity-50"
              >
                <span className="flex flex-col">
                  <span className="text-sm font-medium text-ink-800">
                    {t(`workbench.permissions.${key}`, key)}
                  </span>
                  <span className="text-xs text-ink-500">
                    {t(`workbench.permissionsHint.${key}`, '')}
                  </span>
                </span>
              </Switch>
            );
          })}
        </div>
      </div>

      {error && <Banner tone="error">{error}</Banner>}

      <div className="flex gap-3">
        <Button type="submit" isDisabled={submitting || name.trim() === ''}>
          {submitting
            ? t('workbench.common.saving', 'Saving…')
            : t('workbench.common.save', 'Save')}
        </Button>
        <Button type="button" variant="ghost" onPress={onCancel}>
          {t('workbench.common.cancel', 'Cancel')}
        </Button>
      </div>
    </Form>
  );
}
